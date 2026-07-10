package service

import (
	"BUPT_EC/config"
	"BUPT_EC/service/model"
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

const (
	ServerConfigURL = "https://jwglweixin.bupt.edu.cn/sjd/serverconfig.json"
	DefaultAPIURL   = "https://jwglweixin.bupt.edu.cn/bjyddx/"

	jwRequestTimeout  = 12 * time.Second
	classroomFreshTTL = 5 * time.Minute
	// ClassroomRefreshLimit is the max duration for a shared JW classroom refresh.
	// HTTP WriteTimeout in main must exceed this so a cold-path handler that
	// blocks until refresh finishes can still write the JSON response.
	ClassroomRefreshLimit = 30 * time.Second
	staleRefreshWait      = 300 * time.Millisecond
	staleRefreshBackoff   = 30 * time.Second
)

var ErrNoTodayCache = errors.New("no today classroom cache")

const partialCampusErrorMessage = "部分校区数据刷新失败，已展示可用数据"

func (s *ClassroomService) Login(ctx context.Context) error {
	_, err := s.tokenManager.EnsureToken(ctx, true)
	return err
}

func (s *ClassroomService) QueryOne(ctx context.Context, id string) ([]model.JWClassInfo, error) {
	return s.queryCampus(ctx, id)
}

func (s *ClassroomService) QueryAll(ctx context.Context) (*model.TodayClassrooms, error) {
	return classroomResponseFromRefresh(s.refreshTodayClassrooms(ctx))
}

func (s *ClassroomService) GetTodayClassrooms(ctx context.Context) (*model.TodayClassrooms, error) {
	now := s.now()
	if cached, ok := s.getCachedTodayClassrooms(); ok {
		fresh := !cached.ExpiresAt.Before(now)
		// Fully fresh success: serve without touching JW.
		if fresh && cached.Error == nil {
			s.observeCacheServe("fresh")
			return classroomResponse(cached, false, nil), nil
		}
		// Soft-stale: past fresh TTL, or partial-campus error still inside the
		// fresh window. Always return usable data; kick/share a refresh so a
		// failed campus is retried without waiting the full 5m TTL. Single-flight
		// and failure/partial backoff prevent JW thrashing.
		if now.Before(cached.StaleUntil) {
			if fresh {
				s.startClassroomRefresh(ctx, now)
				if cached.Error != nil || len(cached.PartialCampuses) > 0 {
					s.observeCacheServe("partial")
				} else {
					s.observeCacheServe("fresh")
				}
				return classroomResponse(cached, false, cached.Error), nil
			}
			s.observeCacheServe("stale")
			return s.getStaleTodayClassrooms(ctx, cached, now), nil
		}
	}

	attempt, started := s.startClassroomRefresh(ctx, now)
	if !started {
		s.observeCacheServe("miss")
		if err := s.getLastRefreshError(); err != nil {
			return nil, err
		}
		return nil, ErrNoTodayCache
	}
	select {
	case <-attempt.done:
		if attempt.result.kind == refreshPartial {
			s.observeCacheServe("partial")
		} else if attempt.result.err == nil {
			s.observeCacheServe("fresh")
		} else {
			s.observeCacheServe("miss")
		}
		return classroomResponseFromRefresh(attempt.result)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *ClassroomService) observeCacheServe(state string) {
	if s.metrics != nil {
		s.metrics.ObserveCacheServe(state)
	}
}

func (s *ClassroomService) queryCampus(ctx context.Context, campusID string) ([]model.JWClassInfo, error) {
	token, err := s.tokenManager.EnsureToken(ctx, false)
	if err != nil {
		return nil, err
	}

	rows, err := s.queryCampusWithToken(ctx, campusID, token)
	if err == nil {
		return rows, nil
	}
	if !isJWErrorKind(err, jwErrorAuth) {
		return nil, err
	}

	token, refreshErr := s.tokenManager.RefreshAfterAuthFailure(ctx, token)
	if refreshErr != nil {
		return nil, errors.Join(err, refreshErr)
	}
	rows, retryErr := s.queryCampusWithToken(ctx, campusID, token)
	if retryErr != nil {
		return nil, errors.Join(err, retryErr)
	}
	return rows, nil
}

func (s *ClassroomService) queryCampusWithToken(ctx context.Context, campusID string, token string) ([]model.JWClassInfo, error) {
	apiURL, err := s.tokenManager.APIURL(ctx)
	if err != nil {
		return nil, err
	}
	return s.jwClient.QueryCampus(ctx, apiURL, campusID, token)
}

func (s *ClassroomService) refreshTodayClassrooms(ctx context.Context) classroomRefreshResult {
	startedAt := s.now()
	result := s.doRefreshTodayClassrooms(ctx)
	completedAt := s.now()
	elapsed := completedAt.Sub(startedAt)
	switch result.kind {
	case refreshFailed:
		s.recordRefreshFailure(result.err)
		s.observeRefresh("failed", elapsed)
		for _, failure := range result.failures {
			s.observeCampusFailure(failure.CampusID, classifyError(failure.Err))
		}
		slog.WarnContext(ctx, "classroom refresh failed", "elapsed", elapsed, "err", result.err)
	case refreshPartial:
		s.recordRefreshPartial(completedAt)
		s.observeRefresh("partial", elapsed)
		for _, failure := range result.failures {
			s.observeCampusFailure(failure.CampusID, classifyError(failure.Err))
		}
		slog.WarnContext(ctx, "classroom refresh partially succeeded",
			"elapsed", elapsed,
			"failed_campuses", failedCampusIDs(result.failures),
			"errors", joinCampusRefreshFailures(result.failures))
	default:
		s.recordRefreshSuccess(completedAt)
		s.observeRefresh("full", elapsed)
		slog.InfoContext(ctx, "classroom refresh succeeded", "elapsed", elapsed)
	}
	return result
}

func (s *ClassroomService) observeRefresh(outcome string, duration time.Duration) {
	if s.metrics != nil {
		s.metrics.ObserveRefresh(outcome, duration)
	}
}

func (s *ClassroomService) observeCampusFailure(campusID, kind string) {
	if s.metrics != nil {
		s.metrics.ObserveCampusFailure(campusID, kind)
	}
}

type campusQueryResult struct {
	info model.CampusInfo
	err  error
	ok   bool
}

func (s *ClassroomService) doRefreshTodayClassrooms(ctx context.Context) classroomRefreshResult {
	results := make([]campusQueryResult, len(s.campuses))

	var group errgroupNoCancel
	for i, campusConfig := range s.campuses {
		i, campusConfig := i, campusConfig
		group.Go(func() {
			jwRows, err := s.queryCampus(ctx, campusConfig.ID)
			if err != nil {
				results[i] = campusQueryResult{err: err}
				return
			}
			results[i] = campusQueryResult{
				info: buildCampusInfo(campusConfig, jwRows),
				ok:   true,
			}
		})
	}
	group.Wait()

	successCount := 0
	failures := make([]campusRefreshFailure, 0, len(results))
	for i, result := range results {
		if result.ok {
			successCount++
			continue
		}
		if result.err != nil {
			failures = append(failures, campusRefreshFailure{
				CampusID: s.campuses[i].ID,
				Err:      result.err,
			})
		}
	}
	if successCount == 0 {
		return classroomRefreshResult{
			kind:     refreshFailed,
			failures: failures,
			err:      joinCampusRefreshFailures(failures),
		}
	}

	// Stamp metadata at refresh completion so a JW round-trip that straddles
	// Asia/Shanghai midnight is labeled with the completion business day (not start).
	now := s.now()

	previousByID := map[string]model.CampusInfo{}
	if prev, ok := s.getCachedTodayClassrooms(); ok {
		for _, campus := range prev.Campuses {
			previousByID[campus.ID] = campus
		}
	}

	campuses := make([]model.CampusInfo, len(s.campuses))
	for i, campusConfig := range s.campuses {
		if results[i].ok {
			campuses[i] = results[i].info
			continue
		}
		if prev, ok := previousByID[campusConfig.ID]; ok {
			campuses[i] = prev
			continue
		}
		campuses[i] = emptyCampusInfo(campusConfig)
	}

	var apiErr *model.APIError
	var kind refreshKind
	partialCampuses := failedCampusIDs(failures)
	if successCount < len(s.campuses) {
		kind = refreshPartial
		apiErr = &model.APIError{
			Type:    string(jwErrorQuery),
			Message: partialCampusErrorMessage,
		}
	} else {
		kind = refreshFull
	}

	today := &model.TodayClassrooms{
		Date:            now.Format("2006-01-02"),
		UpdatedAt:       now,
		ExpiresAt:       now.Add(classroomFreshTTL),
		StaleUntil:      endOfDay(now),
		Stale:           false,
		Campuses:        campuses,
		PartialCampuses: partialCampuses,
		Error:           apiErr,
	}

	s.cache.Store(today, cacheExpiration(now, today.StaleUntil))
	return classroomRefreshResult{
		value:    today,
		kind:     kind,
		failures: failures,
	}
}

// cacheExpiration returns the go-cache TTL for a today-classrooms entry.
// go-cache treats non-positive durations as never-expire; clamp to a short
// positive TTL if StaleUntil is not after now (defensive; completion-time
// stamping normally keeps StaleUntil in the future).
func cacheExpiration(now, staleUntil time.Time) time.Duration {
	d := staleUntil.Sub(now)
	if d < time.Second {
		return time.Second
	}
	return d
}

func emptyCampusInfo(campusConfig config.CampusConfig) model.CampusInfo {
	return model.CampusInfo{
		ID:        campusConfig.ID,
		Name:      campusConfig.Name,
		Buildings: []model.BuildingInfo{},
		Nodes:     []model.NodeInfo{},
	}
}

// errgroupNoCancel runs goroutines without canceling siblings on the first error.
type errgroupNoCancel struct {
	wg sync.WaitGroup
}

func (g *errgroupNoCancel) Go(fn func()) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		fn()
	}()
}

func (g *errgroupNoCancel) Wait() {
	g.wg.Wait()
}

func (s *ClassroomService) getCachedTodayClassrooms() (*model.TodayClassrooms, bool) {
	return s.getCachedTodayClassroomsAt(s.now())
}

func (s *ClassroomService) getCachedTodayClassroomsAt(now time.Time) (*model.TodayClassrooms, bool) {
	cached, ok := s.cache.Load()
	if !ok || cached == nil {
		return nil, false
	}
	if cached.Date != now.In(businessLocation).Format("2006-01-02") {
		return nil, false
	}
	return cached, true
}

func classroomResponseFromRefresh(result classroomRefreshResult) (*model.TodayClassrooms, error) {
	if result.err != nil {
		return nil, result.err
	}
	if result.value == nil {
		return nil, newJWError(jwErrorParse, "classroom refresh", nil, "unexpected refresh result")
	}
	return classroomResponse(result.value, false, result.value.Error), nil
}

func classroomResponse(in *model.TodayClassrooms, stale bool, apiErr *model.APIError) *model.TodayClassrooms {
	if in == nil {
		return nil
	}
	out := *in
	out.Stale = stale
	if apiErr != nil {
		errCopy := *apiErr
		out.Error = &errCopy
	} else {
		out.Error = nil
	}
	return &out
}

func staleAPIError(err error) *model.APIError {
	if err == nil {
		return nil
	}
	return &model.APIError{
		Type:    classifyError(err),
		Message: "教务系统暂时不可用，当前展示的是今天最后一次成功刷新数据",
	}
}

// endOfDay returns the exclusive end of the business calendar day: next midnight
// in Asia/Shanghai (or the fixed CST fallback).
func endOfDay(t time.Time) time.Time {
	t = t.In(businessLocation)
	year, month, day := t.Date()
	return time.Date(year, month, day+1, 0, 0, 0, 0, businessLocation)
}
