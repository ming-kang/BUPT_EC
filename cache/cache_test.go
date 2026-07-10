package cache

import (
	"testing"
	"time"

	"BUPT_EC/service/model"
)

func TestTodayClassroomsStoreIsInstanceLocal(t *testing.T) {
	a := New()
	b := New()
	value := &model.TodayClassrooms{Date: "2026-07-10"}
	a.Store(value, time.Minute)

	if got, ok := a.Load(); !ok || got.Date != "2026-07-10" {
		t.Fatalf("a.Load() = (%v, %v)", got, ok)
	}
	if _, ok := b.Load(); ok {
		t.Fatal("independent cache instances must not share data")
	}
}

func TestTodayClassroomsStoreRejectsWrongTypes(t *testing.T) {
	store := New()
	store.inner.Set(todayKey, "not-a-model", time.Minute)
	if got, ok := store.Load(); ok || got != nil {
		t.Fatalf("Load() = (%v, %v), want miss on wrong type", got, ok)
	}
}
