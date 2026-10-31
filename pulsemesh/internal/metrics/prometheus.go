package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

// PeerMetrics is a snapshot of computed metrics for a single peer.
type PeerMetrics struct {
	P50Ms    float64 `json:"p50_ms"`
	P95Ms    float64 `json:"p95_ms"`
	P99Ms    float64 `json:"p99_ms"`
	JitterMs float64 `json:"jitter_ms"`
	LossPct  float64 `json:"loss_pct"`
}

// MetricsProvider supplies snapshot metrics for all peers.
type MetricsProvider interface {
	GetAllMetrics() map[string]PeerMetrics
}

// PrometheusCollector implements prometheus.Collector, exposing PulseMesh
// probe metrics as Prometheus gauges.
type PrometheusCollector struct {
	provider MetricsProvider
	nodeName string

	rttDesc  *prometheus.Desc
	lossDesc *prometheus.Desc
	jitDesc  *prometheus.Desc
}

// RegisterPrometheusMetrics creates and registers a PrometheusCollector with the
// default Prometheus registry.
func RegisterPrometheusMetrics(provider MetricsProvider, nodeName string) *PrometheusCollector {
	c := &PrometheusCollector{
		provider: provider,
		nodeName: nodeName,
		rttDesc: prometheus.NewDesc(
			"pulsemesh_rtt_milliseconds",
			"Round-trip time in milliseconds by quantile.",
			[]string{"src", "dst", "quantile"}, nil,
		),
		lossDesc: prometheus.NewDesc(
			"pulsemesh_packet_loss_ratio",
			"Packet loss ratio (0.0–1.0).",
			[]string{"src", "dst"}, nil,
		),
		jitDesc: prometheus.NewDesc(
			"pulsemesh_jitter_milliseconds",
			"Interarrival jitter in milliseconds (RFC 3550).",
			[]string{"src", "dst"}, nil,
		),
	}

	prometheus.MustRegister(c)
	return c
}

// Describe implements prometheus.Collector.
func (c *PrometheusCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.rttDesc
	ch <- c.lossDesc
	ch <- c.jitDesc
}

// Collect implements prometheus.Collector.
func (c *PrometheusCollector) Collect(ch chan<- prometheus.Metric) {
	allMetrics := c.provider.GetAllMetrics()

	for peer, pm := range allMetrics {
		src := c.nodeName
		dst := peer

		// RTT quantiles.
		for _, q := range []struct {
			label string
			value float64
		}{
			{"0.5", pm.P50Ms},
			{"0.95", pm.P95Ms},
			{"0.99", pm.P99Ms},
		} {
			ch <- prometheus.MustNewConstMetric(
				c.rttDesc,
				prometheus.GaugeValue,
				q.value,
				src, dst, q.label,
			)
		}

		// Loss ratio (convert from percent to 0–1).
		ch <- prometheus.MustNewConstMetric(
			c.lossDesc,
			prometheus.GaugeValue,
			pm.LossPct/100.0,
			src, dst,
		)

		// Jitter.
		ch <- prometheus.MustNewConstMetric(
			c.jitDesc,
			prometheus.GaugeValue,
			pm.JitterMs,
			src, dst,
		)
	}
}

// Ensure PrometheusCollector satisfies the interface at compile time.
var _ prometheus.Collector = (*PrometheusCollector)(nil)

// String returns a description for debugging.
func (c *PrometheusCollector) String() string {
	return fmt.Sprintf("PrometheusCollector(node=%s)", c.nodeName)
}

