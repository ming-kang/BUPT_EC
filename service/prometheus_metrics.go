package service

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusMetrics records ClassroomService events on an isolated registry.
type PrometheusMetrics struct {
	registry *prometheus.Registry

	refreshTotal      *prometheus.CounterVec
	refreshDuration   *prometheus.HistogramVec
	refreshInFlight   prometheus.Gauge
	refreshSuppressed prometheus.Counter
	cacheServes       *prometheus.CounterVec
	loginTotal        *prometheus.CounterVec
	loginDuration     *prometheus.HistogramVec
	campusFailures    *prometheus.CounterVec
}

// NewPrometheusMetrics registers collectors on a private registry.
func NewPrometheusMetrics() (*PrometheusMetrics, error) {
	registry := prometheus.NewRegistry()
	m := &PrometheusMetrics{
		registry: registry,
		refreshTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bupt_ec_refresh_total",
			Help: "Classroom refresh attempts by outcome.",
		}, []string{"outcome"}),
		refreshDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "bupt_ec_refresh_duration_seconds",
			Help:    "Classroom refresh duration by outcome.",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 20, 30},
		}, []string{"outcome"}),
		refreshInFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "bupt_ec_refresh_in_flight",
			Help: "Whether a classroom refresh worker is currently running.",
		}),
		refreshSuppressed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "bupt_ec_refresh_suppressed_total",
			Help: "Refresh starts suppressed by adaptive backoff.",
		}),
		cacheServes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bupt_ec_cache_serves_total",
			Help: "Cache serve outcomes for GetTodayClassrooms.",
		}, []string{"state"}),
		loginTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bupt_ec_login_total",
			Help: "JW login outcomes by source.",
		}, []string{"outcome", "source"}),
		loginDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "bupt_ec_login_duration_seconds",
			Help:    "JW login duration by outcome.",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 15},
		}, []string{"outcome"}),
		campusFailures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bupt_ec_campus_query_failures_total",
			Help: "Per-campus JW query failures by error kind.",
		}, []string{"campus", "kind"}),
	}

	collectors := []prometheus.Collector{
		m.refreshTotal,
		m.refreshDuration,
		m.refreshInFlight,
		m.refreshSuppressed,
		m.cacheServes,
		m.loginTotal,
		m.loginDuration,
		m.campusFailures,
	}
	for _, collector := range collectors {
		if err := registry.Register(collector); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// Registry exposes the private Prometheus registry for /metrics.
func (m *PrometheusMetrics) Registry() *prometheus.Registry {
	if m == nil {
		return nil
	}
	return m.registry
}

func (m *PrometheusMetrics) ObserveRefresh(outcome string, duration time.Duration) {
	if m == nil {
		return
	}
	outcome = normalizeOutcome(outcome)
	m.refreshTotal.WithLabelValues(outcome).Inc()
	m.refreshDuration.WithLabelValues(outcome).Observe(duration.Seconds())
}

func (m *PrometheusMetrics) ObserveRefreshSuppressed() {
	if m == nil {
		return
	}
	m.refreshSuppressed.Inc()
}

func (m *PrometheusMetrics) SetRefreshInFlight(inFlight bool) {
	if m == nil {
		return
	}
	if inFlight {
		m.refreshInFlight.Set(1)
		return
	}
	m.refreshInFlight.Set(0)
}

func (m *PrometheusMetrics) ObserveCacheServe(state string) {
	if m == nil {
		return
	}
	m.cacheServes.WithLabelValues(normalizeCacheState(state)).Inc()
}

func (m *PrometheusMetrics) ObserveLogin(outcome, source string, duration time.Duration) {
	if m == nil {
		return
	}
	m.loginTotal.WithLabelValues(normalizeOutcome(outcome), normalizeLoginSource(source)).Inc()
	m.loginDuration.WithLabelValues(normalizeOutcome(outcome)).Observe(duration.Seconds())
}

func (m *PrometheusMetrics) ObserveCampusFailure(campusID, kind string) {
	if m == nil {
		return
	}
	m.campusFailures.WithLabelValues(normalizeCampusID(campusID), normalizeErrorKind(kind)).Inc()
}

func normalizeOutcome(outcome string) string {
	switch outcome {
	case "full", "partial", "failed", "success":
		return outcome
	default:
		return "failed"
	}
}

func normalizeCacheState(state string) string {
	switch state {
	case "fresh", "stale", "partial", "miss":
		return state
	default:
		return "miss"
	}
}

func normalizeLoginSource(source string) string {
	switch source {
	case "override", "login":
		return source
	default:
		return "login"
	}
}

func normalizeCampusID(campusID string) string {
	switch campusID {
	case "01", "04":
		return campusID
	default:
		return "unknown"
	}
}

func normalizeErrorKind(kind string) string {
	switch kind {
	case string(jwErrorAuth), string(jwErrorConfig), string(jwErrorLogin),
		string(jwErrorQuery), string(jwErrorParse), string(jwErrorUpstream):
		return kind
	default:
		return string(jwErrorUpstream)
	}
}
