package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"BUPT_EC/service/model"
)

const (
	ServerConfigURL = "https://jwglweixin.bupt.edu.cn/sjd/serverconfig.json"
	DefaultAPIURL   = "https://jwglweixin.bupt.edu.cn/bjyddx/"

	// ColdWaitTimeout bounds how long a request without usable cache waits for
	// the refresh loop. The refresh itself keeps running.
	ColdWaitTimeout = 5 * time.Second

	jwRequestTimeout  = 12 * time.Second
	refreshTimeout    = 30 * time.Second
	freshTTL          = 5 * time.Minute
	partialRetryDelay = 30 * time.Second
)

var (
	ErrNoTodayCache = errors.New("no today classroom cache")
	// ErrRefreshWaitTimeout means a cold request waited ColdWaitTimeout and the
	// refresh had not finished yet.
	ErrRefreshWaitTimeout = errors.New("refresh still running after cold wait timeout")
)

const partialCampusErrorMessage = "部分校区数据刷新失败，已展示可用数据"

type campus struct {
	ID   string
	Name string
}

var defaultCampuses = []campus{
	{ID: "01", Name: "西土城"},
	{ID: "04", Name: "沙河"},
}

// businessLocation is the calendar for "today" and day-boundary expiry.
// FixedZone covers hosts without tzdata.
var businessLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}()

// ClassroomService serves today's classroom snapshot from memory. A single
// refresh loop (Run) owns all JW traffic and token state; request handlers
// only read the published snapshot and may nudge the loop.
type ClassroomService struct {
	jw       JWClient
	campuses []campus
	clock    func() time.Time
	coldWait time.Duration

	// cache holds the latest published snapshot. Published values are never
	// mutated; responses that need different Stale/Error fields copy first.
	cache atomic.Pointer[model.TodayClassrooms]
	kick  chan struct{}

	mu          sync.Mutex
	lastErr     error         // last total refresh failure; nil after any usable result
	nextAllowed time.Time     // request nudges before this instant are ignored
	refreshed   chan struct{} // closed (and replaced) after each refresh attempt

	// Owned by the Run goroutine.
	apiURL string
	token  string
}

func NewClassroomService(jw JWClient) *ClassroomService {
	return &ClassroomService{
		jw:        jw,
		campuses:  defaultCampuses,
		clock:     time.Now,
		coldWait:  ColdWaitTimeout,
		kick:      make(chan struct{}, 1),
		refreshed: make(chan struct{}),
	}
}

func (s *ClassroomService) now() time.Time {
	return s.clock().In(businessLocation)
}

// GetTodayClassrooms returns usable same-day data, nudging the refresh loop
// when it is stale or partial. Without usable data it waits up to
// ColdWaitTimeout for the next refresh attempt.
func (s *ClassroomService) GetTodayClassrooms(ctx context.Context) (*model.TodayClassrooms, error) {
	now := s.now()
	if cached := s.todayCache(now); cached != nil {
		if now.Before(cached.ExpiresAt) {
			if cached.Error != nil {
				s.requestRefresh()
			}
			return cached, nil
		}
		s.requestRefresh()
		stale := *cached
		stale.Stale = true
		if err := s.lastError(); err != nil {
			stale.Error = &model.APIError{
				Type:    classifyError(err),
				Message: "教务系统暂时不可用，当前展示的是今天最后一次成功刷新数据",
			}
		}
		return &stale, nil
	}

	s.mu.Lock()
	refreshed, lastErr, backingOff := s.refreshed, s.lastErr, now.Before(s.nextAllowed)
	s.mu.Unlock()
	if backingOff && lastErr != nil {
		return nil, lastErr
	}
	s.requestRefresh()

	timer := time.NewTimer(s.coldWait)
	defer timer.Stop()
	select {
	case <-refreshed:
		if cached := s.todayCache(s.now()); cached != nil {
			return cached, nil
		}
		if err := s.lastError(); err != nil {
			return nil, err
		}
		return nil, ErrNoTodayCache
	case <-timer.C:
		return nil, ErrRefreshWaitTimeout
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Ready reports whether usable same-day data is available.
func (s *ClassroomService) Ready() bool {
	return s.todayCache(s.now()) != nil
}

// todayCache returns the published snapshot if it belongs to now's business day.
func (s *ClassroomService) todayCache(now time.Time) *model.TodayClassrooms {
	cached := s.cache.Load()
	if cached == nil || cached.Date != now.Format(time.DateOnly) {
		return nil
	}
	return cached
}

func (s *ClassroomService) lastError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastErr
}

func (s *ClassroomService) requestRefresh() {
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

// Run refreshes immediately, then keeps today's snapshot current until ctx is
// canceled: at midnight, after failures (30s/1m/2m/5m), every 5 minutes while
// partial, and whenever a request nudges it outside the backoff window.
func (s *ClassroomService) Run(ctx context.Context) {
	failures := 0
	for {
		err := s.refresh(ctx)
		if ctx.Err() != nil {
			return
		}
		now := s.now()
		var wait, block time.Duration
		switch {
		case err != nil:
			failures++
			wait = failureBackoff(failures)
			block = wait
		case s.cache.Load().Error != nil:
			failures = 0
			wait, block = freshTTL, partialRetryDelay
		default:
			failures = 0
			wait, block = endOfDay(now).Sub(now), freshTTL
		}
		wait = min(wait, endOfDay(now).Sub(now))

		s.mu.Lock()
		s.lastErr = err
		s.nextAllowed = now.Add(block)
		close(s.refreshed)
		s.refreshed = make(chan struct{})
		s.mu.Unlock()

		if !s.sleep(ctx, wait) {
			return
		}
	}
}

// sleep waits for the scheduled delay or an admissible request nudge.
func (s *ClassroomService) sleep(ctx context.Context, wait time.Duration) bool {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-timer.C:
			return true
		case <-s.kick:
			s.mu.Lock()
			allowed := !s.now().Before(s.nextAllowed)
			s.mu.Unlock()
			if allowed {
				return true
			}
		}
	}
}

func failureBackoff(failures int) time.Duration {
	switch failures {
	case 1:
		return 30 * time.Second
	case 2:
		return time.Minute
	case 3:
		return 2 * time.Minute
	default:
		return 5 * time.Minute
	}
}

// endOfDay returns the next Asia/Shanghai midnight after t.
func endOfDay(t time.Time) time.Time {
	t = t.In(businessLocation)
	year, month, day := t.Date()
	return time.Date(year, month, day+1, 0, 0, 0, 0, businessLocation)
}

// refresh queries every campus and publishes a snapshot when at least one
// campus succeeded. Failed campuses keep their previous same-day data. The
// returned error is non-nil only when every campus failed.
func (s *ClassroomService) refresh(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, refreshTimeout)
	defer cancel()
	started := time.Now()

	results := s.queryCampuses(ctx)

	var failures []error
	var partial []string
	for i, result := range results {
		if result.err != nil {
			failures = append(failures, fmt.Errorf("campus %s: %w", s.campuses[i].ID, result.err))
			partial = append(partial, s.campuses[i].ID)
		}
	}
	elapsed := time.Since(started)
	if len(partial) == len(s.campuses) {
		err := errors.Join(failures...)
		slog.WarnContext(ctx, "classroom refresh failed", "elapsed", elapsed, "err", err)
		return err
	}

	// Stamp at completion so a refresh straddling midnight belongs to the new day.
	now := s.now()
	previous := map[string]model.CampusInfo{}
	if cached := s.todayCache(now); cached != nil {
		for _, info := range cached.Campuses {
			previous[info.ID] = info
		}
	}
	campuses := make([]model.CampusInfo, len(s.campuses))
	for i, c := range s.campuses {
		switch prev, ok := previous[c.ID]; {
		case results[i].err == nil:
			campuses[i] = buildCampusInfo(c, results[i].rows)
		case ok:
			campuses[i] = prev
		default:
			campuses[i] = buildCampusInfo(c, nil)
		}
	}

	today := &model.TodayClassrooms{
		Date:            now.Format(time.DateOnly),
		UpdatedAt:       now,
		ExpiresAt:       now.Add(freshTTL),
		StaleUntil:      endOfDay(now),
		Campuses:        campuses,
		PartialCampuses: partial,
	}
	if len(partial) > 0 {
		today.Error = &model.APIError{Type: string(jwErrorQuery), Message: partialCampusErrorMessage}
		slog.WarnContext(ctx, "classroom refresh partially succeeded",
			"elapsed", elapsed, "failed_campuses", partial, "errors", errors.Join(failures...))
	} else {
		slog.InfoContext(ctx, "classroom refresh succeeded", "elapsed", elapsed)
	}
	s.cache.Store(today)
	return nil
}

type campusResult struct {
	rows []model.JWClassInfo
	err  error
}

// queryCampuses queries all campuses in parallel with the cached token. Campuses
// rejected for authentication are retried once after a fresh login.
func (s *ClassroomService) queryCampuses(ctx context.Context) []campusResult {
	results := make([]campusResult, len(s.campuses))
	fail := func(indexes []int, err error) {
		for _, i := range indexes {
			results[i].err = errors.Join(results[i].err, err)
		}
	}
	all := make([]int, len(s.campuses))
	for i := range all {
		all[i] = i
	}

	if s.apiURL == "" {
		apiURL, err := s.jw.FetchAPIURL(ctx)
		if err != nil {
			fail(all, err)
			return results
		}
		s.apiURL = apiURL
	}
	if s.token == "" {
		if err := s.login(ctx); err != nil {
			fail(all, err)
			return results
		}
	}

	s.queryIndexes(ctx, all, results)
	var rejected []int
	for i, result := range results {
		if isJWErrorKind(result.err, jwErrorAuth) {
			rejected = append(rejected, i)
		}
	}
	if len(rejected) == 0 {
		return results
	}
	if err := s.login(ctx); err != nil {
		fail(rejected, err)
		return results
	}
	firstErrs := make([]error, len(results))
	for _, i := range rejected {
		firstErrs[i], results[i] = results[i].err, campusResult{}
	}
	s.queryIndexes(ctx, rejected, results)
	for _, i := range rejected {
		if results[i].err != nil {
			results[i].err = errors.Join(firstErrs[i], results[i].err)
		}
	}
	return results
}

func (s *ClassroomService) queryIndexes(ctx context.Context, indexes []int, results []campusResult) {
	var wg sync.WaitGroup
	for _, i := range indexes {
		wg.Go(func() {
			rows, err := s.jw.QueryCampus(ctx, s.apiURL, s.campuses[i].ID, s.token)
			results[i] = campusResult{rows: rows, err: err}
		})
	}
	wg.Wait()
}

func (s *ClassroomService) login(ctx context.Context) error {
	s.token = ""
	started := time.Now()
	token, err := s.jw.Login(ctx, s.apiURL)
	if err != nil {
		slog.WarnContext(ctx, "jw login failed", "elapsed", time.Since(started), "err", err)
		return err
	}
	slog.InfoContext(ctx, "jw login succeeded", "elapsed", time.Since(started))
	s.token = token
	return nil
}
