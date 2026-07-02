package service

import (
	"BUPT_EC/service/model"
	"context"
	"time"
)

type classroomRefreshAttempt struct {
	done   chan struct{}
	result classroomRefreshResult
}

type classroomRefreshResult struct {
	value *model.TodayClassrooms
	err   error
}

func (s *ClassroomService) startClassroomRefresh(now time.Time) (*classroomRefreshAttempt, bool) {
	s.refreshMu.Lock()
	if s.refreshInFlight {
		attempt := s.refreshAttempt
		s.refreshMu.Unlock()
		return attempt, true
	}

	if !s.nextRefreshAllowed.IsZero() && now.Before(s.nextRefreshAllowed) {
		s.refreshMu.Unlock()
		return nil, false
	}

	s.refreshInFlight = true
	attempt := &classroomRefreshAttempt{done: make(chan struct{})}
	s.refreshAttempt = attempt
	s.refreshWorkers.Add(1)
	s.refreshMu.Unlock()

	go func() {
		defer s.refreshWorkers.Done()
		refreshCtx, cancel := context.WithTimeout(context.Background(), classroomRefreshLimit)
		defer cancel()

		today, err := s.refreshTodayClassrooms(refreshCtx)
		s.finishClassroomRefresh(attempt, classroomRefreshResult{value: today, err: err})
	}()
	return attempt, true
}

func (s *ClassroomService) finishClassroomRefresh(attempt *classroomRefreshAttempt, result classroomRefreshResult) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	attempt.result = result
	if s.refreshAttempt == attempt {
		s.refreshInFlight = false
		s.refreshAttempt = nil
	}
	if result.err != nil {
		s.lastRefreshErr = result.err
		s.nextRefreshAllowed = s.now().Add(staleRefreshBackoff)
	} else {
		s.lastRefreshErr = nil
		s.nextRefreshAllowed = time.Time{}
	}
	close(attempt.done)
}

func (s *ClassroomService) getLastRefreshError() error {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	return s.lastRefreshErr
}

// WaitWarmup blocks until in-flight background refresh workers finish,
// or until ctx is done. Used to drain work during graceful shutdown.
func (s *ClassroomService) WaitWarmup(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		s.refreshWorkers.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *ClassroomService) getStaleTodayClassrooms(ctx context.Context, cached *model.TodayClassrooms, now time.Time) *model.TodayClassrooms {
	attempt, started := s.startClassroomRefresh(now)
	if !started {
		return classroomResponse(cached, true, staleAPIError(s.getLastRefreshError()))
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
