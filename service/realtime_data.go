package service

import (
	"BUPT_EC/cache"
	"BUPT_EC/config"
	"BUPT_EC/service/model"
	"context"
	"errors"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"
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
)

var (
	ErrNoTodayCache = errors.New("no today classroom cache")

	tokenManager = &TokenManager{}

	nowFunc                  = time.Now
	queryCampusForRefresh    = queryCampus
	queryCampusWithTokenFunc = queryCampusWithToken
	refreshTokenFunc         = (*TokenManager).refreshToken
)

func ResetRuntimeStateForTest() {
	refreshWorkers.Wait()
	tokenManager = &TokenManager{}
	resetRefreshState()
	runtimeStatusMu.Lock()
	runtimeStatus = RuntimeStatus{}
	runtimeStatusMu.Unlock()
	cache.DeleteCache(TodayCacheKey)
}

func Login(ctx context.Context) error {
	_, err := tokenManager.EnsureToken(ctx, true)
	return err
}

func QueryOne(ctx context.Context, id string) ([]model.JWClassInfo, error) {
	return queryCampus(ctx, id, false)
}

func QueryAll(ctx context.Context) (*model.TodayClassrooms, error) {
	return refreshTodayClassrooms(ctx)
}

func StartClassroomWarmup() {
	go func() {
		attempt, started := startClassroomRefresh(nowFunc())
		if !started {
			return
		}
		<-attempt.done
	}()
}

func GetTodayClassrooms(ctx context.Context) (*model.TodayClassrooms, error) {
	now := nowFunc()
	if cached, ok := getCachedTodayClassrooms(); ok {
		if !cached.ExpiresAt.Before(now) {
			return classroomResponse(cached, false, nil), nil
		}
		if now.Before(cached.StaleUntil) {
			return getStaleTodayClassrooms(ctx, cached, now), nil
		}
	}

	attempt, started := startClassroomRefresh(now)
	if !started {
		if err := getLastRefreshError(); err != nil {
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

func classroomResponseFromRefresh(result classroomRefreshResult) (*model.TodayClassrooms, error) {
	if result.err != nil {
		return nil, result.err
	}
	if result.value == nil {
		return nil, newJWError(jwErrorParse, "classroom refresh", nil, "unexpected refresh result")
	}
	return classroomResponse(result.value, false, nil), nil
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

func refreshTodayClassrooms(ctx context.Context) (*model.TodayClassrooms, error) {
	startedAt := time.Now()
	today, err := doRefreshTodayClassrooms(ctx)
	if err != nil {
		recordRefreshFailure(err)
		slog.WarnContext(ctx, "classroom refresh failed", "elapsed", time.Since(startedAt), "err", err)
		return nil, err
	}
	recordRefreshSuccess(time.Now())
	slog.InfoContext(ctx, "classroom refresh succeeded", "elapsed", time.Since(startedAt))
	return today, nil
}

func doRefreshTodayClassrooms(ctx context.Context) (*model.TodayClassrooms, error) {
	now := nowFunc()
	campuses := config.GetConfig().Campuses
	today := &model.TodayClassrooms{
		Date:       now.Format("2006-01-02"),
		UpdatedAt:  now,
		ExpiresAt:  now.Add(classroomFreshTTL),
		StaleUntil: endOfDay(now),
		Stale:      false,
		Campuses:   make([]model.CampusInfo, len(campuses)),
		Error:      nil,
	}

	group, groupCtx := errgroup.WithContext(ctx)
	for i, campusConfig := range campuses {
		i, campusConfig := i, campusConfig
		group.Go(func() error {
			jwRows, err := queryCampusForRefresh(groupCtx, campusConfig.ID, false)
			if err != nil {
				return err
			}
			today.Campuses[i] = buildCampusInfo(campusConfig, jwRows)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	cache.SetCache(TodayCacheKey, today, time.Until(today.StaleUntil))
	return classroomResponse(today, false, nil), nil
}

func getCachedTodayClassrooms() (*model.TodayClassrooms, bool) {
	raw, ok := cache.GetCache(TodayCacheKey)
	if !ok || raw == nil {
		return nil, false
	}
	cached, ok := raw.(*model.TodayClassrooms)
	if !ok || cached == nil {
		return nil, false
	}
	if cached.Date != nowFunc().Format("2006-01-02") {
		return nil, false
	}
	return cached, true
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

func endOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 23, 59, 59, 0, t.Location())
}
