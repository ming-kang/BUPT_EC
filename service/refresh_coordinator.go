package service

import (
	"BUPT_EC/service/model"
	"context"
	"sync"
	"time"
)

var (
	refreshStateMu     sync.Mutex
	refreshInFlight    bool
	refreshAttempt     *classroomRefreshAttempt
	nextRefreshAllowed time.Time
	lastRefreshError   error
	refreshWorkers     sync.WaitGroup
)

type classroomRefreshAttempt struct {
	done   chan struct{}
	result classroomRefreshResult
}

type classroomRefreshResult struct {
	value *model.TodayClassrooms
	err   error
}

func startClassroomRefresh(now time.Time) (*classroomRefreshAttempt, bool) {
	refreshStateMu.Lock()
	if refreshInFlight {
		attempt := refreshAttempt
		refreshStateMu.Unlock()
		return attempt, true
	}

	if !nextRefreshAllowed.IsZero() && now.Before(nextRefreshAllowed) {
		refreshStateMu.Unlock()
		return nil, false
	}

	refreshInFlight = true
	attempt := &classroomRefreshAttempt{done: make(chan struct{})}
	refreshAttempt = attempt
	refreshWorkers.Add(1)
	refreshStateMu.Unlock()

	go func() {
		defer refreshWorkers.Done()
		refreshCtx, cancel := context.WithTimeout(context.Background(), classroomRefreshLimit)
		defer cancel()

		today, err := refreshTodayClassrooms(refreshCtx)
		finishClassroomRefresh(attempt, classroomRefreshResult{value: today, err: err})
	}()
	return attempt, true
}

func finishClassroomRefresh(attempt *classroomRefreshAttempt, result classroomRefreshResult) {
	refreshStateMu.Lock()
	defer refreshStateMu.Unlock()

	attempt.result = result
	if refreshAttempt == attempt {
		refreshInFlight = false
		refreshAttempt = nil
	}
	if result.err != nil {
		lastRefreshError = result.err
		nextRefreshAllowed = nowFunc().Add(staleRefreshBackoff)
	} else {
		lastRefreshError = nil
		nextRefreshAllowed = time.Time{}
	}
	close(attempt.done)
}

func resetRefreshState() {
	refreshStateMu.Lock()
	defer refreshStateMu.Unlock()
	refreshInFlight = false
	refreshAttempt = nil
	nextRefreshAllowed = time.Time{}
	lastRefreshError = nil
}

func getLastRefreshError() error {
	refreshStateMu.Lock()
	defer refreshStateMu.Unlock()
	return lastRefreshError
}

func getStaleTodayClassrooms(ctx context.Context, cached *model.TodayClassrooms, now time.Time) *model.TodayClassrooms {
	attempt, started := startClassroomRefresh(now)
	if !started {
		return classroomResponse(cached, true, staleAPIError(getLastRefreshError()))
	}

	timer := time.NewTimer(staleRefreshWait)
	defer timer.Stop()

	select {
	case <-attempt.done:
		if attempt.result.err == nil {
			fresh, err := classroomResponseFromRefresh(attempt.result)
			if err == nil {
				return fresh
			}
			return classroomResponse(cached, true, staleAPIError(err))
		}
		return classroomResponse(cached, true, staleAPIError(attempt.result.err))
	case <-timer.C:
		return classroomResponse(cached, true, nil)
	case <-ctx.Done():
		return classroomResponse(cached, true, nil)
	}
}
