package collector

import (
	"context"
	"sync"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pborzenkov/go-transmission/transmission"
	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "transmission"

// TransmissionCollector implements the prometheus.Collector interface.
type TransmissionCollector struct {
	client *transmission.Client
	logger log.Logger

	portOpenDesc   *prometheus.Desc
	turtleModeDesc *prometheus.Desc

	activeTorrents *prometheus.Desc
	pausedTorrents *prometheus.Desc

	downloadedBytesTotal *prometheus.Desc
	uploadedBytesTotal   *prometheus.Desc
}

func NewTransmissionCollector(client *transmission.Client, logger log.Logger) (*TransmissionCollector, error) {
	return &TransmissionCollector{
		client: client,
		logger: logger,

		portOpenDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "is_port_open"),
			"Indicates whether or not the peer port is accessible from the Internet.",
			nil, nil,
		),
		turtleModeDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "is_turtle_mode_active"),
			"Indicates whether or not turtle mode is active.",
			nil, nil,
		),

		activeTorrents: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "active_torrents"),
			"Number of active torrents.",
			nil, nil,
		),
		pausedTorrents: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "paused_torrents"),
			"Number of paused torrents.",
			nil, nil,
		),

		downloadedBytesTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "downloaded_bytes_total"),
			"Total amount of downloaded data.",
			nil, nil,
		),
		uploadedBytesTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "uploaded_bytes_total"),
			"Total amount of uploaded data.",
			nil, nil,
		),
	}, nil
}

// Describe implements the prometheus.Collector interface
func (t *TransmissionCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- t.portOpenDesc
	ch <- t.turtleModeDesc
}

// Collect implements the prometheus.Collector interface.
func (t *TransmissionCollector) Collect(ch chan<- prometheus.Metric) {
	fns := []func(chan<- prometheus.Metric){
		t.collectPortOpen,
		t.collectTurtleMode,
		t.collectSessionStats,
	}

	var wg sync.WaitGroup

	wg.Add(len(fns))
	for _, fn := range fns {
		fn := fn
		go func() {
			fn(ch)
			wg.Done()
		}()
	}

	wg.Wait()
}

func (t *TransmissionCollector) collectPortOpen(ch chan<- prometheus.Metric) {
	open, err := t.client.IsPortOpen(context.Background())
	if err != nil {
		level.Error(t.logger).Log("msg", "failed to get peer port state", "err", err)
		return
	}

	val := 0.
	if open {
		val = 1.
	}
	ch <- prometheus.MustNewConstMetric(t.portOpenDesc, prometheus.GaugeValue, val)
}

func (t *TransmissionCollector) collectTurtleMode(ch chan<- prometheus.Metric) {
	sess, err := t.client.GetSession(context.Background(), transmission.SessionFieldTurtleEnabled)
	if err != nil {
		level.Error(t.logger).Log("msg", "failed to query session info", "err", err)
		return
	}

	val := 0.
	if sess.TurtleEnabled {
		val = 1.
	}
	ch <- prometheus.MustNewConstMetric(t.turtleModeDesc, prometheus.GaugeValue, val)
}

func (t *TransmissionCollector) collectSessionStats(ch chan<- prometheus.Metric) {
	stats, err := t.client.GetSessionStats(context.Background())
	if err != nil {
		level.Error(t.logger).Log("msg", "failed to query session statistics", "err", err)
		return
	}

	ch <- prometheus.MustNewConstMetric(t.activeTorrents, prometheus.GaugeValue, float64(stats.ActiveTorrents))
	ch <- prometheus.MustNewConstMetric(t.pausedTorrents, prometheus.GaugeValue, float64(stats.PausedTorrents))
	ch <- prometheus.MustNewConstMetric(t.downloadedBytesTotal, prometheus.GaugeValue, float64(stats.AllSessions.Downloaded))
	ch <- prometheus.MustNewConstMetric(t.uploadedBytesTotal, prometheus.GaugeValue, float64(stats.AllSessions.Uploaded))
}
