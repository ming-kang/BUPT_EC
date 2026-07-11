package service

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestPrometheusMetricsIsolatedRegistryLoginSeries(t *testing.T) {
	metrics, err := NewPrometheusMetrics()
	if err != nil {
		t.Fatalf("NewPrometheusMetrics() error = %v", err)
	}

	// Baseline: collectors are registered but no login samples yet.
	if got := counterValue(t, metrics.registry, "bupt_ec_login_total", map[string]string{
		"outcome": "success",
		"source":  "login",
	}); got != 0 {
		t.Fatalf("baseline login success counter = %v, want 0", got)
	}

	metrics.ObserveLogin("success", "login", 150*time.Millisecond)
	metrics.ObserveLogin("failed", "override", 250*time.Millisecond)
	metrics.ObserveLogin("success", "override", 50*time.Millisecond)
	metrics.ObserveLogin("failed", "login", 0)

	if got := counterValue(t, metrics.registry, "bupt_ec_login_total", map[string]string{
		"outcome": "success",
		"source":  "login",
	}); got != 1 {
		t.Fatalf("login success/login counter = %v, want 1", got)
	}
	if got := counterValue(t, metrics.registry, "bupt_ec_login_total", map[string]string{
		"outcome": "failed",
		"source":  "override",
	}); got != 1 {
		t.Fatalf("login failed/override counter = %v, want 1", got)
	}
	if got := counterValue(t, metrics.registry, "bupt_ec_login_total", map[string]string{
		"outcome": "success",
		"source":  "override",
	}); got != 1 {
		t.Fatalf("login success/override counter = %v, want 1", got)
	}
	if got := counterValue(t, metrics.registry, "bupt_ec_login_total", map[string]string{
		"outcome": "failed",
		"source":  "login",
	}); got != 1 {
		t.Fatalf("login failed/login counter = %v, want 1", got)
	}

	count, sum := histogramSample(t, metrics.registry, "bupt_ec_login_duration_seconds", map[string]string{
		"outcome": "success",
	})
	if count != 2 {
		t.Fatalf("success duration sample count = %d, want 2", count)
	}
	if sum < 0 {
		t.Fatalf("success duration sum = %v, want non-negative", sum)
	}

	// Existing families remain registered and writable.
	metrics.ObserveRefresh("full", time.Second)
	metrics.ObserveCacheServe("fresh")
	metrics.ObserveCampusFailure("01", string(jwErrorAuth))
	metrics.ObserveRefreshSuppressed()
	metrics.SetRefreshInFlight(true)

	families := gatherFamilies(t, metrics.registry)
	for _, name := range []string{
		"bupt_ec_refresh_total",
		"bupt_ec_refresh_duration_seconds",
		"bupt_ec_refresh_in_flight",
		"bupt_ec_refresh_suppressed_total",
		"bupt_ec_cache_serves_total",
		"bupt_ec_login_total",
		"bupt_ec_login_duration_seconds",
		"bupt_ec_campus_query_failures_total",
	} {
		if _, ok := families[name]; !ok {
			t.Fatalf("missing metric family %q", name)
		}
	}

	// Labels must stay low-cardinality enums; never attach secrets or free text.
	loginFamily := families["bupt_ec_login_total"]
	for _, metric := range loginFamily.GetMetric() {
		for _, label := range metric.GetLabel() {
			name := label.GetName()
			value := label.GetValue()
			if name != "outcome" && name != "source" {
				t.Fatalf("unexpected login label %q=%q", name, value)
			}
			if strings.Contains(strings.ToLower(value), "token") ||
				strings.Contains(strings.ToLower(value), "password") ||
				strings.Contains(value, "http") {
				t.Fatalf("login label leaked sensitive value %q=%q", name, value)
			}
		}
	}
}

func TestPrometheusMetricsNormalizeLoginLabels(t *testing.T) {
	metrics, err := NewPrometheusMetrics()
	if err != nil {
		t.Fatalf("NewPrometheusMetrics() error = %v", err)
	}
	// Unknown outcome/source collapse to safe enums, never free text.
	metrics.ObserveLogin("boom\nsecret-token", "http://evil.example/token?u=admin", time.Second)

	if got := counterValue(t, metrics.registry, "bupt_ec_login_total", map[string]string{
		"outcome": "failed",
		"source":  "login",
	}); got != 1 {
		t.Fatalf("normalized login counter = %v, want 1", got)
	}
}

func gatherFamilies(t *testing.T, registry *prometheus.Registry) map[string]*dto.MetricFamily {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	out := make(map[string]*dto.MetricFamily, len(families))
	for _, family := range families {
		out[family.GetName()] = family
	}
	return out
}

func counterValue(t *testing.T, registry *prometheus.Registry, name string, labels map[string]string) float64 {
	t.Helper()
	family, ok := gatherFamilies(t, registry)[name]
	if !ok {
		return 0
	}
	for _, metric := range family.GetMetric() {
		if labelsMatch(metric.GetLabel(), labels) {
			if metric.Counter == nil {
				t.Fatalf("metric %q is not a counter", name)
			}
			return metric.Counter.GetValue()
		}
	}
	return 0
}

func histogramSample(t *testing.T, registry *prometheus.Registry, name string, labels map[string]string) (uint64, float64) {
	t.Helper()
	family, ok := gatherFamilies(t, registry)[name]
	if !ok {
		t.Fatalf("missing histogram family %q", name)
	}
	for _, metric := range family.GetMetric() {
		if labelsMatch(metric.GetLabel(), labels) {
			if metric.Histogram == nil {
				t.Fatalf("metric %q is not a histogram", name)
			}
			return metric.Histogram.GetSampleCount(), metric.Histogram.GetSampleSum()
		}
	}
	t.Fatalf("histogram %q with labels %v not found", name, labels)
	return 0, 0
}

func labelsMatch(got []*dto.LabelPair, want map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for _, pair := range got {
		value, ok := want[pair.GetName()]
		if !ok || value != pair.GetValue() {
			return false
		}
	}
	return true
}
