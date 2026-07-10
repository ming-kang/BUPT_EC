package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"BUPT_EC/config"
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

func businessNow() time.Time {
	return time.Now().In(businessLocation)
}

// CacheStore is the cache abstraction used by ClassroomService.
// *gocache.Cache satisfies it directly.
type CacheStore interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, expiration time.Duration)
	Delete(key string)
}

// ClassroomService owns all runtime state for classroom queries:
// token/API URL caching, refresh coordination and runtime status.
type ClassroomService struct {
	tokenManager *TokenManager
	cache        CacheStore
	campuses     []config.CampusConfig
	jwClient     JWClient
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
}

func NewClassroomService(options ClassroomServiceOptions, store CacheStore, client JWClient) (*ClassroomService, error) {
	if isNilDependency(store) {
		return nil, errors.New("classroom cache store is required")
	}
	if isNilDependency(client) {
		return nil, errors.New("JW client is required")
	}
	if len(options.Campuses) == 0 {
		return nil, errors.New("at least one campus is required")
	}

	s := &ClassroomService{
		cache:        store,
		campuses:     append([]config.CampusConfig(nil), options.Campuses...),
		jwClient:     client,
		now:          businessNow,
		warmupJitter: randomWarmupJitter,
	}
	s.tokenManager = &TokenManager{
		jwClient:       client,
		overrideToken:  options.TokenOverride,
		onLoginSuccess: s.recordLoginSuccess,
		onLoginFailure: s.recordLoginFailure,
	}
	return s, nil
}
