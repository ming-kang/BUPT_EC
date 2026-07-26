package service

import (
	"context"
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
	campuses   []config.CampusConfig
	jwClient   JWClient
	clock      Clock
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

	backgroundMu       sync.Mutex
	backgroundStopping bool
	warmupStarted      bool
	warmupDone         chan struct{}
	warmupCancel       context.CancelFunc
	warmupJitter       func() time.Duration

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
}

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
	metrics := options.Metrics
	if metrics == nil {
		metrics = NoopMetrics{}
	}
	s := &ClassroomService{
		campuses:      append([]config.CampusConfig(nil), options.Campuses...),
		jwClient:      client,
		clock:         clock,
		backoffRandom: backoffRandom,
		warmupJitter:  warmupJitter,
		metrics:       metrics,
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
