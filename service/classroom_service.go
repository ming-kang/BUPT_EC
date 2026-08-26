package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"BUPT_EC/config"
	"BUPT_EC/service/model"
)

// businessLocation is the calendar used for "today" and day-boundary cache expiry.
// Asia/Shanghai matches BUPT academic operations; FixedZone covers hosts without tzdata.
var businessLocation = loadBusinessLocation()

func loadBusinessLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}

// Clock provides the wall clock for cache policy, refresh backoff, login, and
// runtime status timestamps. Implementations must be safe for concurrent use.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// ClassroomService owns all runtime state for classroom queries:
// token/API URL caching, refresh coordination and runtime status.
type ClassroomService struct {
	tokenManager *TokenManager
	// todayCache holds the single same-day classroom payload. Stored values are
	// treated as immutable: responses copy before mutating (classroomResponse),
	// and cross-day rejection happens on read via the Date guard in
	// getCachedTodayClassroomsAt, so no TTL/janitor is needed.
	todayCache atomic.Pointer[model.TodayClassrooms]
	// cachedDataJSON holds the pre-serialized JSON bytes of the fresh
	// TodayClassrooms value (Stale=false, Error=nil). Updated atomically
	// alongside todayCache on every successful refresh. The handler uses
	// these bytes on the hot path to skip per-request json.Marshal entirely.
	cachedDataJSON atomic.Pointer[[]byte]
	campuses       []config.CampusConfig
	jwClient       JWClient
	clock          Clock
	// backoffRandom returns one sample in [0,1] for total-failure jitter.
	// Always non-nil after construction (production or injected).
	backoffRandom RandomSample

	refreshMu                sync.Mutex
	refreshInFlight          bool
	refreshAttempt           *classroomRefreshAttempt
	nextRefreshAllowed       time.Time
	lastRefreshErr           error
	consecutiveTotalFailures int
	refreshWorkers           sync.WaitGroup
	metrics                  RuntimeMetrics

	backgroundMu    sync.Mutex
	lifecycleCtx    context.Context    // nil until Run starts the scheduler
	lifecycleCancel context.CancelFunc // set by Run; canceled by Run/Shutdown under backgroundMu
	schedulerDone   chan struct{}      // closed when the Run scheduler goroutine exits
	warmupJitter    func() time.Duration
	// coldWaitTimeout bounds the cache-miss wait in GetTodayClassrooms.
	// Always positive after construction (default or injected).
	coldWaitTimeout time.Duration

	statusMu sync.RWMutex
	status   RuntimeStatus
}

// RandomSample returns a unit-interval sample used by total-failure jitter.
// Implementations should return a value intended for [0,1]; the policy clamps
// invalid samples and never trusts callers to supply an arbitrary duration.
type RandomSample func() float64

type ClassroomServiceOptions struct {
	Campuses      []config.CampusConfig
	TokenOverride string
	// Clock is optional; nil uses the real wall clock. Instants are converted to
	// Asia/Shanghai for business-day logic by ClassroomService.now.
	Clock Clock
	// Metrics is optional; nil uses NoopMetrics (no runtime metric emission).
	Metrics RuntimeMetrics
	// BackoffRandom is optional; nil uses a concurrent-safe production source.
	// Only unit samples are accepted — the jitter policy clamps and bounds them.
	BackoffRandom RandomSample
	// WarmupJitter is optional; nil uses the randomized production jitter added
	// to the warmup midnight rollover wait (randomWarmupJitter).
	WarmupJitter func() time.Duration
	// ColdWaitTimeout bounds how long a cache-miss request waits for the
	// in-flight cold-start refresh before returning ErrRefreshWaitTimeout.
	// Zero or negative values resolve to defaultColdWaitTimeout. The refresh
	// itself keeps running; only the wait is bounded.
	ColdWaitTimeout time.Duration
}

// defaultColdWaitTimeout covers a typical login + two-campus query (~1-3s)
// with headroom, while failing fast enough that cold-start users see a clear
// error instead of a 30s spinner.
// DefaultColdWaitTimeout is the production cold-miss wait bound used when
// ClassroomServiceOptions.ColdWaitTimeout is zero or negative.
const DefaultColdWaitTimeout = 5 * time.Second

const defaultColdWaitTimeout = DefaultColdWaitTimeout

func NewClassroomService(options ClassroomServiceOptions, client JWClient) (*ClassroomService, error) {
	if client == nil {
		return nil, errors.New("JW client is required")
	}
	if len(options.Campuses) == 0 {
		return nil, errors.New("at least one campus is required")
	}

	clock := options.Clock
	if clock == nil {
		clock = systemClock{}
	}
	backoffRandom := options.BackoffRandom
	if backoffRandom == nil {
		backoffRandom = productionBackoffRandom
	}
	warmupJitter := options.WarmupJitter
	if warmupJitter == nil {
		warmupJitter = randomWarmupJitter
	}
	coldWaitTimeout := options.ColdWaitTimeout
	if coldWaitTimeout <= 0 {
		coldWaitTimeout = defaultColdWaitTimeout
	}
	metrics := options.Metrics
	if metrics == nil {
		metrics = NoopMetrics{}
	}
	s := &ClassroomService{
		campuses:        append([]config.CampusConfig(nil), options.Campuses...),
		jwClient:        client,
		clock:           clock,
		backoffRandom:   backoffRandom,
		warmupJitter:    warmupJitter,
		metrics:         metrics,
		coldWaitTimeout: coldWaitTimeout,
	}
	s.tokenManager = &TokenManager{
		jwClient:       client,
		overrideToken:  options.TokenOverride,
		clock:          clock,
		metrics:        metrics,
		onLoginSuccess: s.recordLoginSuccess,
		onLoginFailure: s.recordLoginFailure,
	}
	return s, nil
}

// now returns the business-location instant from the injected Clock.
// Production and tests share this single time seam (same instance as
// TokenManager); the constructor guarantees a non-nil clock.
func (s *ClassroomService) now() time.Time {
	return s.clock.Now().In(businessLocation)
}

// updateCachedDataJSON pre-serializes the fresh TodayClassrooms value so the
// handler can write it directly without per-request json.Marshal. Called once
// per successful refresh, immediately after todayCache.Store. A marshal failure
// (unreachable for the fixed model types) clears the cached bytes so the
// handler falls back to runtime encoding gracefully.
func (s *ClassroomService) updateCachedDataJSON(today *model.TodayClassrooms) {
	// The handler's fast path serves the fresh variant (Stale=false, Error=nil).
	// Build that exact shape so the JSON output is byte-identical to what
	// writeJSON would produce for classroomResponse(cached, false, nil).
	fresh := *today
	fresh.Stale = false
	fresh.Error = nil
	data, err := json.Marshal(&fresh)
	if err != nil {
		// Unreachable: model types are fixed and always marshal cleanly.
		s.cachedDataJSON.Store(nil)
		return
	}
	s.cachedDataJSON.Store(&data)
}

// GetCachedDataJSON returns the pre-serialized fresh TodayClassrooms JSON and
// true if a usable cached value exists for the current business day. Returns
// nil, false when no cache is available, the cache is cross-day, or the
// pre-serialized bytes were never computed.
func (s *ClassroomService) GetCachedDataJSON() ([]byte, bool) {
	cached, ok := s.getCachedTodayClassrooms()
	if !ok {
		return nil, false
	}
	// Only serve the pre-serialized fast path for the fully fresh, error-free
	// case. Stale and partial responses have different Stale/Error fields and
	// must fall back to runtime serialization.
	now := s.now()
	if cached.ExpiresAt.Before(now) || cached.Error != nil {
		return nil, false
	}
	ptr := s.cachedDataJSON.Load()
	if ptr == nil {
		return nil, false
	}
	return *ptr, true
}
