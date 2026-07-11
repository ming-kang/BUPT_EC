package service

import (
	"context"
	"errors"
	"sync"
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

// TodayClassroomCache is the typed process-local cache for same-day classroom data.
// Implementations must not require callers to know storage keys or interface{} casts.
type TodayClassroomCache interface {
	Load() (*model.TodayClassrooms, bool)
	Store(value *model.TodayClassrooms, expiration time.Duration)
}

// ClassroomService owns all runtime state for classroom queries:
// token/API URL caching, refresh coordination and runtime status.
type ClassroomService struct {
	tokenManager *TokenManager
	cache        TodayClassroomCache
	campuses     []config.CampusConfig
	jwClient     JWClient
	clock        Clock
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
	// Metrics is optional; nil disables runtime metric emission.
	Metrics RuntimeMetrics
	// BackoffRandom is optional; nil uses a concurrent-safe production source.
	// Only unit samples are accepted — the jitter policy clamps and bounds them.
	BackoffRandom RandomSample
}

func NewClassroomService(options ClassroomServiceOptions, store TodayClassroomCache, client JWClient) (*ClassroomService, error) {
	if isNilDependency(store) {
		return nil, errors.New("classroom cache store is required")
	}
	if isNilDependency(client) {
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
	s := &ClassroomService{
		cache:         store,
		campuses:      append([]config.CampusConfig(nil), options.Campuses...),
		jwClient:      client,
		clock:         clock,
		backoffRandom: backoffRandom,
		warmupJitter:  randomWarmupJitter,
		metrics:       options.Metrics,
	}
	s.tokenManager = &TokenManager{
		jwClient:       client,
		overrideToken:  options.TokenOverride,
		clock:          clock,
		metrics:        options.Metrics,
		onLoginSuccess: s.recordLoginSuccess,
		onLoginFailure: s.recordLoginFailure,
	}
	return s, nil
}

// now returns the business-location instant from the injected Clock.
// Production and tests share this single time seam (same instance as TokenManager).
func (s *ClassroomService) now() time.Time {
	if s == nil || s.clock == nil {
		return time.Now().In(businessLocation)
	}
	return s.clock.Now().In(businessLocation)
}
