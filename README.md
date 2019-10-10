# TC4400 Exporter

This is a Prometheus exporter for the Technicolor TC4400 DOCSIS 3.1 cable modem.
It gathers metrics by scraping the modem's web interface.

This was developed using a TC4400 running firmware SR70.12.33-180327, untested on other releases.

A scrape takes about 20 seconds so the scrape interval and timeout have to be configured accordingly:

```yaml
  - job_name: 'tc4400'
    scrape_interval: '1m'
    scrape_timeout: '55s'
    static_configs:
      - targets:
        - 'localhost:9623'
```

Known issues:

* The values of tc4400_network_receive_bytes_total and tc4400_network_transmit_bytes_total don't chanage.
