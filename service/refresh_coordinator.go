package service

import (
	"BUPT_EC/service/model"
	"context"
	"errors"
	"fmt"
	"time"
)

type refreshKind int

const (
	refreshFull refreshKind = iota
	refreshPartial
	refreshFailed
)

type campusRefreshFailure struct {
	CampusID string
	Err      error
}

type classroomRefreshAttempt struct {
	done   chan struct{}
	result classroomRefreshResult
}

type classroomRefreshResult struct {
	value    *model.TodayClassrooms
	kind     refreshKind
	failures []campusRefreshFailure
	err      error
}

func (s *ClassroomService) startClassroomRefresh(ctx context.Context, now time.Time) (*classroomRefreshAttempt, bool) {
	s.backgroundMu.Lock()
	if s.backgroundStopping {
		s.backgroundMu.Unlock()
		return nil, false
	}

	s.refreshMu.Lock()
	if s.refreshInFlight {
		attempt := s.refreshAttempt
		s.refreshMu.Unlock()
		s.backgroundMu.Unlock()
		return attempt, true
	}

	if !s.nextRefreshAllowed.IsZero() && now.Before(s.nextRefreshAllowed) {
		s.refreshMu.Unlock()
		s.backgroundMu.Unlock()
		return nil, false
	}

	s.refreshInFlight = true
	attempt := &classroomRefreshAttempt{done: make(chan struct{})}
	s.refreshAttempt = attempt
	s.refreshWorkers.Add(1)
	s.refreshMu.Unlock()
	s.backgroundMu.Unlock()

	// Keep request values such as log_id, but never inherit client cancellation
	// or deadlines — shared workers outlive any single waiter.
	parent := context.WithoutCancel(nonNilContext(ctx))
	go func() {
		defer s.refreshWorkers.Done()
		refreshCtx, cancel := context.WithTimeout(parent, ClassroomRefreshLimit)
		defer cancel()

		result := s.refreshTodayClassrooms(refreshCtx)
		s.finishClassroomRefresh(attempt, result)
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
	if result.kind == refreshFailed || result.err != nil {
		s.lastRefreshErr = result.err
		s.nextRefreshAllowed = s.now().Add(staleRefreshBackoff)
	} else if result.kind == refreshPartial {
		s.lastRefreshErr = nil
		// Partial campus success is usable, but still backs off so soft-stale
		// requests do not hammer JW while a campus is unavailable.
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

func (s *ClassroomService) nextRefreshAllowedAt() time.Time {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	return s.nextRefreshAllowed
}

func (s *ClassroomService) getStaleTodayClassrooms(ctx context.Context, cached *model.TodayClassrooms, now time.Time) *model.TodayClassrooms {
	attempt, started := s.startClassroomRefresh(ctx, now)
	if !started {
		if err := s.getLastRefreshError(); err != nil {
			return classroomResponse(cached, true, staleAPIError(err))
		}
		return classroomResponse(cached, true, cached.Error)
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
		return classroomResponse(cached, true, cached.Error)
	case <-ctx.Done():
		return classroomResponse(cached, true, cached.Error)
	}
}

func failedCampusIDs(failures []campusRefreshFailure) []string {
	ids := make([]string, 0, len(failures))
	for _, failure := range failures {
		ids = append(ids, failure.CampusID)
	}
	return ids
}

func joinCampusRefreshFailures(failures []campusRefreshFailure) error {
	if len(failures) == 0 {
		return newJWError(jwErrorQuery, "classroom refresh", nil, "all campus queries failed")
	}
	errs := make([]error, 0, len(failures))
	for _, failure := range failures {
		errs = append(errs, fmt.Errorf("campus %s: %w", failure.CampusID, failure.Err))
	}
	return errors.Join(errs...)
}
