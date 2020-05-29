package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	exporterName = "tc4400_exporter"
	namespace    = "tc4400"
)

func main() {
	var (
		listenAddress   = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9623").OverrideDefaultFromEnvar("TC4400_EXPORTER_PORT").String()
		metricsPath     = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		clientScrapeURI = kingpin.Flag("client.scrape-uri", "Base URI on which to scrape TC4400.").Default("http://admin:bEn2o%23US9s@192.168.100.1/").OverrideDefaultFromEnvar("TC4400_EXPORTER_SCRAPEURI").String()
		clientTimeout   = kingpin.Flag("client.timeout", "Timeout for HTTP requests to TC440.").Default("50s").OverrideDefaultFromEnvar("TC4400_EXPORTER_CLIENTTIMEOUT").Duration()
	)

	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print(exporterName))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting", exporterName, version.Info())
	log.Infoln("Build context", version.BuildContext())

	exporter, err := NewExporter(*clientScrapeURI, *clientTimeout)
	if err != nil {
		log.Fatal(err)
	}
	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector(exporterName))

	log.Infoln("Listening on", *listenAddress)
	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>TC4400 Exporter</title></head>
             <body>
             <h1>TC4400 Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
