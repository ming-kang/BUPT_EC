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
	now          func() time.Time

	refreshMu          sync.Mutex
	refreshInFlight    bool
	refreshAttempt     *classroomRefreshAttempt
	nextRefreshAllowed time.Time
	lastRefreshErr     error
	refreshWorkers     sync.WaitGroup

	backgroundMu       sync.Mutex
	backgroundStopping bool
	warmupStarted      bool
	warmupDone         chan struct{}
	warmupCancel       context.CancelFunc
	warmupJitter       func() time.Duration

	statusMu sync.RWMutex
	status   RuntimeStatus
}

type ClassroomServiceOptions struct {
	Campuses      []config.CampusConfig
	TokenOverride string
	// Clock is optional; nil uses the real wall clock. Instants are converted to
	// Asia/Shanghai for business-day logic by ClassroomService.now.
	Clock Clock
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
	s := &ClassroomService{
		cache:        store,
		campuses:     append([]config.CampusConfig(nil), options.Campuses...),
		jwClient:     client,
		clock:        clock,
		now:          func() time.Time { return clock.Now().In(businessLocation) },
		warmupJitter: randomWarmupJitter,
	}
	s.tokenManager = &TokenManager{
		jwClient:       client,
		overrideToken:  options.TokenOverride,
		clock:          clock,
		onLoginSuccess: s.recordLoginSuccess,
		onLoginFailure: s.recordLoginFailure,
	}
	return s, nil
}
