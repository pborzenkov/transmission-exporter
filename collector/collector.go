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

	turtleModeDesc *prometheus.Desc

	activeTorrentsDesc *prometheus.Desc
	pausedTorrentsDesc *prometheus.Desc

	downloadedBytesTotalDesc *prometheus.Desc
	uploadedBytesTotalDesc   *prometheus.Desc
}

func NewTransmissionCollector(client *transmission.Client, logger log.Logger) (*TransmissionCollector, error) {
	return &TransmissionCollector{
		client: client,
		logger: logger,

		turtleModeDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "is_turtle_mode_active"),
			"Indicates whether or not turtle mode is active.",
			nil, nil,
		),

		activeTorrentsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "active_torrents"),
			"Number of active torrents.",
			nil, nil,
		),
		pausedTorrentsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "paused_torrents"),
			"Number of paused torrents.",
			nil, nil,
		),

		downloadedBytesTotalDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "downloaded_bytes_total"),
			"Total amount of downloaded data.",
			nil, nil,
		),
		uploadedBytesTotalDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "uploaded_bytes_total"),
			"Total amount of uploaded data.",
			nil, nil,
		),
	}, nil
}

// Describe implements the prometheus.Collector interface
func (t *TransmissionCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- t.turtleModeDesc
	ch <- t.activeTorrentsDesc
	ch <- t.pausedTorrentsDesc

	ch <- t.downloadedBytesTotalDesc
	ch <- t.uploadedBytesTotalDesc
}

// Collect implements the prometheus.Collector interface.
func (t *TransmissionCollector) Collect(ch chan<- prometheus.Metric) {
	fns := []func(chan<- prometheus.Metric){
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

	ch <- prometheus.MustNewConstMetric(t.activeTorrentsDesc, prometheus.GaugeValue, float64(stats.ActiveTorrents))
	ch <- prometheus.MustNewConstMetric(t.pausedTorrentsDesc, prometheus.GaugeValue, float64(stats.PausedTorrents))
	ch <- prometheus.MustNewConstMetric(t.downloadedBytesTotalDesc, prometheus.GaugeValue, float64(stats.AllSessions.Downloaded))
	ch <- prometheus.MustNewConstMetric(t.uploadedBytesTotalDesc, prometheus.GaugeValue, float64(stats.AllSessions.Uploaded))
}
