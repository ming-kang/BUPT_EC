package service

import "time"

// RuntimeMetrics is the low-cardinality observability seam for ClassroomService.
// Implementations must never accept secrets, raw upstream text, log IDs, or room names.
type RuntimeMetrics interface {
	ObserveRefresh(outcome string, duration time.Duration)
	ObserveRefreshSuppressed()
	SetRefreshInFlight(inFlight bool)
	ObserveCacheServe(state string)
	ObserveLogin(outcome, source string, duration time.Duration)
	ObserveCampusFailure(campusID, kind string)
}

// NoopMetrics discards all observations.
type NoopMetrics struct{}

func (NoopMetrics) ObserveRefresh(string, time.Duration)       {}
func (NoopMetrics) ObserveRefreshSuppressed()                  {}
func (NoopMetrics) SetRefreshInFlight(bool)                    {}
func (NoopMetrics) ObserveCacheServe(string)                   {}
func (NoopMetrics) ObserveLogin(string, string, time.Duration) {}
func (NoopMetrics) ObserveCampusFailure(string, string)        {}
