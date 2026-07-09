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

	LoginUsernameKey = "JW_USERNAME"
	LoginPasswordKey = "JW_PASSWORD"
	LoginTokenKey    = "JW_TOKEN"

	TodayCacheKey = "TODAY_CLASSROOMS_CACHE"

	jwRequestTimeout      = 12 * time.Second
	classroomFreshTTL     = 5 * time.Minute
	classroomRefreshLimit = 30 * time.Second
	staleRefreshWait      = 300 * time.Millisecond
	staleRefreshBackoff   = 30 * time.Second
	warmupDayJitter       = time.Second
)

var ErrNoTodayCache = errors.New("no today classroom cache")

const partialCampusErrorMessage = "部分校区数据刷新失败，已展示可用数据"

func (s *ClassroomService) Login(ctx context.Context) error {
	_, err := s.tokenManager.EnsureToken(ctx, true)
	return err
}

func (s *ClassroomService) QueryOne(ctx context.Context, id string) ([]model.JWClassInfo, error) {
	return s.queryCampus(ctx, id, false)
}

func (s *ClassroomService) QueryAll(ctx context.Context) (*model.TodayClassrooms, error) {
	return s.refreshTodayClassrooms(ctx)
}

// StartWarmup kicks an immediate background refresh, then re-warms after each
// Asia/Shanghai midnight so long-lived processes do not stay cold across days.
func (s *ClassroomService) StartWarmup() {
	go s.warmupLoop()
}

func (s *ClassroomService) warmupLoop() {
	s.runWarmupOnce()
	for {
		now := s.now()
		nextMidnight := endOfDay(now)
		wait := nextMidnight.Sub(now) + warmupDayJitter
		if wait < time.Second {
			wait = time.Second
		}
		time.Sleep(wait)
		s.runWarmupOnce()
	}
}

func (s *ClassroomService) runWarmupOnce() {
	attempt, started := s.startClassroomRefresh(s.now())
	if !started {
		return
	}
	<-attempt.done
}

func (s *ClassroomService) GetTodayClassrooms(ctx context.Context) (*model.TodayClassrooms, error) {
	now := s.now()
	if cached, ok := s.getCachedTodayClassrooms(); ok {
		if !cached.ExpiresAt.Before(now) {
			return classroomResponse(cached, false, cached.Error), nil
		}
		if now.Before(cached.StaleUntil) {
			return s.getStaleTodayClassrooms(ctx, cached, now), nil
		}
	}

	attempt, started := s.startClassroomRefresh(now)
	if !started {
		if err := s.getLastRefreshError(); err != nil {
			return nil, err
		}
		return nil, ErrNoTodayCache
	}
	select {
	case <-attempt.done:
		return classroomResponseFromRefresh(attempt.result)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *ClassroomService) queryCampus(ctx context.Context, campusID string, forceRefresh bool) ([]model.JWClassInfo, error) {
	token, err := s.tokenManager.EnsureToken(ctx, forceRefresh)
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

	s.tokenManager.clearTokenIfCurrent(token)
	token, refreshErr := s.tokenManager.EnsureToken(ctx, true)
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

func (s *ClassroomService) refreshTodayClassrooms(ctx context.Context) (*model.TodayClassrooms, error) {
	startedAt := time.Now()
	today, err := s.doRefreshTodayClassrooms(ctx)
	if err != nil {
		s.recordRefreshFailure(err)
		slog.WarnContext(ctx, "classroom refresh failed", "elapsed", time.Since(startedAt), "err", err)
		return nil, err
	}
	s.recordRefreshSuccess(time.Now())
	slog.InfoContext(ctx, "classroom refresh succeeded", "elapsed", time.Since(startedAt))
	return today, nil
}

type campusQueryResult struct {
	info model.CampusInfo
	err  error
	ok   bool
}

func (s *ClassroomService) doRefreshTodayClassrooms(ctx context.Context) (*model.TodayClassrooms, error) {
	now := s.now()
	results := make([]campusQueryResult, len(s.campuses))

	var group errgroupNoCancel
	for i, campusConfig := range s.campuses {
		i, campusConfig := i, campusConfig
		group.Go(func() {
			jwRows, err := s.queryCampus(ctx, campusConfig.ID, false)
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
	var errs []error
	for _, result := range results {
		if result.ok {
			successCount++
			continue
		}
		if result.err != nil {
			errs = append(errs, result.err)
		}
	}
	if successCount == 0 {
		if len(errs) == 0 {
			return nil, newJWError(jwErrorQuery, "classroom refresh", nil, "all campus queries failed")
		}
		return nil, errors.Join(errs...)
	}

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
	if successCount < len(s.campuses) {
		apiErr = &model.APIError{
			Type:    string(jwErrorQuery),
			Message: partialCampusErrorMessage,
		}
	}

	today := &model.TodayClassrooms{
		Date:       now.Format("2006-01-02"),
		UpdatedAt:  now,
		ExpiresAt:  now.Add(classroomFreshTTL),
		StaleUntil: endOfDay(now),
		Stale:      false,
		Campuses:   campuses,
		Error:      apiErr,
	}

	s.cache.Set(TodayCacheKey, today, time.Until(today.StaleUntil))
	return classroomResponse(today, false, apiErr), nil
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
	raw, ok := s.cache.Get(TodayCacheKey)
	if !ok || raw == nil {
		return nil, false
	}
	cached, ok := raw.(*model.TodayClassrooms)
	if !ok || cached == nil {
		return nil, false
	}
	if cached.Date != s.now().Format("2006-01-02") {
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
