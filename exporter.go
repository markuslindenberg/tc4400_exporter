package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
)

var (
	channelLabelNames   = []string{"channel"}
	interfaceLabelNames = []string{"interface"}
)

func newChannelMetric(subsystemName, metricName, docString string, extraLabels ...string) *prometheus.Desc {
	return prometheus.NewDesc(prometheus.BuildFQName(namespace, subsystemName, metricName), docString, append(channelLabelNames, extraLabels...), nil)
}

type metrics map[int]*prometheus.Desc

var (
	targetUpMetric = prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "up"), "Was the last scrape of TC4400 succesful.", nil, nil)

	networkMetrics = metrics{
		1: prometheus.NewDesc(prometheus.BuildFQName(namespace, "network", "receive_bytes_total"), "", []string{"interface"}, nil),
		2: prometheus.NewDesc(prometheus.BuildFQName(namespace, "network", "receive_packets_total"), "", []string{"interface"}, nil),
		3: prometheus.NewDesc(prometheus.BuildFQName(namespace, "network", "receive_errs_total"), "", []string{"interface"}, nil),
		4: prometheus.NewDesc(prometheus.BuildFQName(namespace, "network", "receive_drop_total"), "", []string{"interface"}, nil),
		5: prometheus.NewDesc(prometheus.BuildFQName(namespace, "network", "transmit_bytes_total"), "", []string{"interface"}, nil),
		6: prometheus.NewDesc(prometheus.BuildFQName(namespace, "network", "transmit_packets_total"), "", []string{"interface"}, nil),
		7: prometheus.NewDesc(prometheus.BuildFQName(namespace, "network", "transmit_errs_total"), "", []string{"interface"}, nil),
		8: prometheus.NewDesc(prometheus.BuildFQName(namespace, "network", "transmit_drop_total"), "", []string{"interface"}, nil),
	}

	downstreamChannelMetrics = metrics{
		2:  newChannelMetric("downstream", "locked", "Downstream Lock Status"),
		3:  newChannelMetric("downstream", "channel_type", "Downstream Channel Type", "type"),
		4:  newChannelMetric("downstream", "bonded", "Downstream Bonding Status"),
		5:  newChannelMetric("downstream", "center_frequency_hz", "Downstream Center Frequency"),
		6:  newChannelMetric("downstream", "width_hz", "Downstream Width"),
		7:  newChannelMetric("downstream", "snr_threshold_db", "Downstream SNR/MER Threshold Value"),
		8:  newChannelMetric("downstream", "receive_level_dbmv", "Downstream Receive Level"),
		9:  newChannelMetric("downstream", "modulation", "Downstream Modulation/Profile ID", "modulation"),
		10: newChannelMetric("downstream", "codewords_unerrored_total", "Downstream Unerrored Codewords"),
		11: newChannelMetric("downstream", "codewords_corrected_total", "Downstream Corrected Codewords"),
		12: newChannelMetric("downstream", "codewords_uncorrectable_total", "Downstream Uncorrectable Codewords"),
	}

	upstreamChannelMetrics = metrics{
		2: newChannelMetric("upstream", "locked", "Upstream Lock Status"),
		3: newChannelMetric("upstream", "channel_type", "Downstream Channel Type", "type"),
		4: newChannelMetric("upstream", "bonded", "Upstream Bonding Status"),
		5: newChannelMetric("upstream", "center_frequency_hz", "Upstream Center Frequency"),
		6: newChannelMetric("upstream", "width_hz", "Upstream Width"),
		7: newChannelMetric("upstream", "transmit_level_dbmv", "Upstream Transmit Level"),
		8: newChannelMetric("upstream", "modulation", "Upstream Modulation/Profile ID", "modulation"),
	}
)

type Exporter struct {
	baseURL string
	client  *http.Client
	mutex   sync.RWMutex

	totalScrapes          prometheus.Counter
	parseFailures         *prometheus.CounterVec
	clientRequestCount    *prometheus.CounterVec
	clientRequestDuration *prometheus.HistogramVec
}

func NewExporter(uri string, timeout time.Duration) (*Exporter, error) {
	client := &http.Client{}
	client.Timeout = timeout

	clientRequestCount := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "exporter_client_requests_total",
		Help:      "HTTP requests to TC4400",
	}, []string{"code", "method"})

	clientRequestDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "exporter_client_request_duration_seconds",
		Help:      "Histogram of TC4400 HTTP request latencies.",
	}, []string{"code", "method"})

	client.Transport = promhttp.InstrumentRoundTripperCounter(clientRequestCount,
		promhttp.InstrumentRoundTripperDuration(clientRequestDuration, http.DefaultTransport))

	return &Exporter{
		baseURL: uri,
		client:  client,
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrapes_total",
			Help:      "Current total TC4400 scrapes.",
		}),
		parseFailures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_parse_errors_total",
			Help:      "Number of errors while parsing HTML tables.",
		}, []string{"file"}),
		clientRequestCount:    clientRequestCount,
		clientRequestDuration: clientRequestDuration,
	}, nil
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range networkMetrics {
		ch <- m
	}
	for _, m := range downstreamChannelMetrics {
		ch <- m
	}
	for _, m := range upstreamChannelMetrics {
		ch <- m
	}

	ch <- targetUpMetric
	ch <- e.totalScrapes.Desc()
	e.parseFailures.Describe(ch)
	e.clientRequestCount.Describe(ch)
	e.clientRequestDuration.Describe(ch)
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	up := e.scrape(ch)
	ch <- prometheus.MustNewConstMetric(targetUpMetric, prometheus.GaugeValue, up)

	ch <- e.totalScrapes
	e.parseFailures.Collect(ch)
	e.clientRequestCount.Collect(ch)
	e.clientRequestDuration.Collect(ch)
}

func (e *Exporter) fetch(filename string) (io.ReadCloser, error) {
	u, err := url.Parse(e.baseURL)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, filename)

	resp, err := e.client.Get(u.String())
	if err != nil {
		return nil, err
	}
	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		resp.Body.Close()
		return nil, fmt.Errorf("Scraping %s failed: HTTP status %d", u.String(), resp.StatusCode)
	}
	return resp.Body, nil
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) (up float64) {
	e.totalScrapes.Inc()

	// networkMetrics - statsifc.html

	body, err := e.fetch("statsifc.html")
	if err == nil {
		tables, err := parseTables(body)
		body.Close()
		if err != nil {
			log.Errorln(err)
			e.parseFailures.WithLabelValues("statsifc.html").Inc()
		} else {
			if len(tables) < 1 || len(tables[0]) < 2 {
				log.Errorln("No table found in statsifc.html")
				e.parseFailures.WithLabelValues("statsifc.html").Inc()
			} else {
				for _, row := range tables[0][2:] {
					if len(row) != 9 {
						continue
					}

					for i, metric := range networkMetrics {
						valueInt, err := strconv.ParseInt(row[i], 10, 64)
						value := float64(valueInt)
						if err != nil {
							log.Errorln(err)
							e.parseFailures.WithLabelValues("statsifc.html").Inc()
							continue
						}
						ch <- prometheus.MustNewConstMetric(metric, prometheus.CounterValue, value, row[0])
					}
				}
			}
		}
	}

	// upstreamChannelMetrics, downstreamChannelMetrics - cmconnectionstatus.html

	body, err = e.fetch("cmconnectionstatus.html")
	if err == nil {
		tables, err := parseTables(body)
		body.Close()
		if err != nil {
			log.Errorln(err)
			e.parseFailures.WithLabelValues("cmconnectionstatus.html").Inc()
		} else {
			if len(tables) < 3 || len(tables[1]) < 2 || len(tables[2]) < 2 {
				log.Errorln("Tables not found in cmconnectionstatus.html")
				e.parseFailures.WithLabelValues("cmconnectionstatus.html").Inc()
			} else {

				// downstreamChannelMetrics
				for _, row := range tables[1][2:] {
					if len(row) != 13 {
						continue
					}

					channel, err := strconv.ParseInt(row[1], 10, 64)
					if err != nil {
						log.Errorln(err)
						e.parseFailures.WithLabelValues("cmconnectionstatus.html").Inc()
						continue
					}
					channelLabel := fmt.Sprintf("%02d", channel)

					for i, metric := range downstreamChannelMetrics {
						var err error = nil
						var value float64
						var valueInt int64
						var labelValues = []string{channelLabel}
						switch i {
						case 10, 11, 12:
							valueInt, err = strconv.ParseInt(row[i], 10, 64)
							value = float64(valueInt)
						case 2:
							if row[i] == "Locked" {
								value = 1
							} else {
								value = 0
							}
						case 3, 9:
							labelValues = append(labelValues, row[i])
							value = 1
						case 4:
							if row[i] == "Bonded" {
								value = 1
							} else {
								value = 0
							}
						case 5, 6:
							parts := strings.Split(row[i], " ")
							if len(parts) != 2 {
								continue
							}
							valueInt, err = strconv.ParseInt(parts[0], 10, 64)
							switch parts[1] {
							case "Hz":
							case "kHz":
								valueInt = valueInt * 1000
							default:
								continue
							}
							value = float64(valueInt)
						case 7:
							parts := strings.Split(row[i], " ")
							if len(parts) != 2 || parts[1] != "dB" {
								continue
							}
							value, err = strconv.ParseFloat(parts[0], 64)
						case 8:
							parts := strings.Split(row[i], " ")
							if len(parts) != 2 || parts[1] != "dBmV" {
								continue
							}
							value, err = strconv.ParseFloat(parts[0], 64)
						default:
							continue
						}

						if err != nil {
							log.Errorln(err)
							e.parseFailures.WithLabelValues("cmconnectionstatus.html").Inc()
							continue
						}
						ch <- prometheus.MustNewConstMetric(metric, prometheus.CounterValue, value, labelValues...)
					}
				}

				// upstreamChannelMetrics
				for _, row := range tables[2][2:] {
					if len(row) != 9 {
						continue
					}

					channel, err := strconv.ParseInt(row[1], 10, 64)
					if err != nil {
						log.Errorln(err)
						e.parseFailures.WithLabelValues("cmconnectionstatus.html").Inc()
						continue
					}
					channelLabel := fmt.Sprintf("%02d", channel)

					for i, metric := range upstreamChannelMetrics {
						var err error = nil
						var value float64
						var valueInt int64
						var labelValues = []string{channelLabel}
						switch i {
						case 2:
							if row[i] == "Locked" {
								value = 1
							} else {
								value = 0
							}
						case 3, 8:
							labelValues = append(labelValues, row[i])
							value = 1
						case 4:
							if row[i] == "Bonded" {
								value = 1
							} else {
								value = 0
							}
						case 5, 6:
							parts := strings.Split(row[i], " ")
							if len(parts) != 2 {
								continue
							}
							valueInt, err = strconv.ParseInt(parts[0], 10, 64)
							switch parts[1] {
							case "Hz":
							case "kHz":
								valueInt = valueInt * 1000
							default:
								continue
							}
							value = float64(valueInt)
						case 7:
							parts := strings.Split(row[i], " ")
							if len(parts) != 2 || parts[1] != "dBmV" {
								continue
							}
							value, err = strconv.ParseFloat(parts[0], 64)
						default:
							continue
						}

						if err != nil {
							log.Errorln(err)
							e.parseFailures.WithLabelValues("cmconnectionstatus.html").Inc()
							continue
						}
						ch <- prometheus.MustNewConstMetric(metric, prometheus.CounterValue, value, labelValues...)
					}
				}

			}
		}
	}

	return 1
}
