package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pborzenkov/go-transmission/transmission"
	"github.com/pborzenkov/transmission-exporter/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

func newHandler(turl string, logger log.Logger) (http.Handler, error) {
	trans, err := transmission.New(turl)
	if err != nil {
		return nil, fmt.Errorf("couldn't create transmission client: %s", err)
	}

	tc, err := collector.NewTransmissionCollector(trans, logger)
	if err != nil {
		return nil, fmt.Errorf("couldn't create collector: %s", err)
	}

	r := prometheus.NewRegistry()
	if err := r.Register(tc); err != nil {
		return nil, fmt.Errorf("couldn't register node collector: %s", err)
	}

	handler := promhttp.HandlerFor(
		prometheus.Gatherers{r},
		promhttp.HandlerOpts{
			ErrorHandling: promhttp.HTTPErrorOnError,
		},
	)

	return handler, nil
}

func must(h http.Handler, err error) http.Handler {
	if err != nil {
		panic(err)
	}

	return h
}

func main() {
	listenAddress := kingpin.Flag(
		"web.listen-address",
		"Address on which to expose metrics and web interface.",
	).Default(":29100").String()
	metricsPath := kingpin.Flag(
		"web.telemetry-path",
		"Path under which to expose metrics.",
	).Default("/metrics").String()
	transmissionURL := kingpin.Flag(
		"transmission.url",
		"Transmission RPC server URL",
	).Default("http://127.0.0.1:9091").String()

	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(version.Print("transmission-exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promlog.New(promlogConfig)

	level.Info(logger).Log("msg", "Starting transmission-exporter", "version", version.Info())

	http.Handle(*metricsPath, must(newHandler(*transmissionURL, logger)))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Transmission Exporter</title></head>
			<body>
			<h1>Node Exporter</h1>
			<p><a href="` + *metricsPath + `">Metrics</a></p>
			</body>
			</html>`))
	})

	level.Info(logger).Log("msg", "Listening on", "address", *listenAddress)
	server := &http.Server{Addr: *listenAddress}
	if err := web.ListenAndServe(server, "", logger); err != nil {
		level.Error(logger).Log("err", err)
		os.Exit(1)
	}
}
