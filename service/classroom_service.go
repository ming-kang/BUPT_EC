package service

import (
	"BUPT_EC/config"
	"sync"
	"time"
)

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

	statusMu sync.RWMutex
	status   RuntimeStatus
}

// NewClassroomService creates a ClassroomService backed by the real JW system client.
func NewClassroomService(cfg config.Config, store CacheStore) *ClassroomService {
	return newClassroomService(cfg, store, &defaultJWClient{})
}

func newClassroomService(cfg config.Config, store CacheStore, client JWClient) *ClassroomService {
	s := &ClassroomService{
		cache:    store,
		campuses: cfg.Campuses,
		jwClient: client,
		now:      time.Now,
	}
	s.tokenManager = &TokenManager{
		jwClient:       client,
		onLoginSuccess: s.recordLoginSuccess,
		onLoginFailure: s.recordLoginFailure,
	}
	return s
}
