package service

import (
	"strings"
	"testing"

	"BUPT_EC/cache"
	"BUPT_EC/config"
)

func TestNewClassroomServiceValidatesDependenciesWithoutLeakingOverride(t *testing.T) {
	secretOverride := "constructor-secret-token"
	options := ClassroomServiceOptions{
		Campuses:      []config.CampusConfig{{ID: "01", Name: "西土城"}},
		TokenOverride: secretOverride,
	}
	store := cache.New()
	client := &mockJWClient{}
	var typedNilStore *cache.TodayClassroomsStore
	var typedNilClient *mockJWClient

	tests := []struct {
		name    string
		options ClassroomServiceOptions
		store   TodayClassroomCache
		client  JWClient
	}{
		{name: "missing cache", options: options, client: client},
		{name: "typed nil cache", options: options, store: typedNilStore, client: client},
		{name: "missing JW client", options: options, store: store},
		{name: "typed nil JW client", options: options, store: store, client: typedNilClient},
		{name: "missing campuses", options: ClassroomServiceOptions{TokenOverride: secretOverride}, store: store, client: client},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClassroomService(tt.options, tt.store, tt.client)
			if err == nil {
				t.Fatal("NewClassroomService() expected constructor error")
			}
			if strings.Contains(err.Error(), secretOverride) {
				t.Fatalf("NewClassroomService() error leaked token override: %v", err)
			}
		})
	}
}

func TestNewClassroomServiceCopiesCampusOptions(t *testing.T) {
	campuses := []config.CampusConfig{
		{ID: "01", Name: "西土城"},
		{ID: "04", Name: "沙河"},
	}
	svc, err := NewClassroomService(
		ClassroomServiceOptions{Campuses: campuses},
		cache.New(),
		&mockJWClient{},
	)
	if err != nil {
		t.Fatalf("NewClassroomService() error = %v", err)
	}
	campuses[0].Name = "changed"
	if svc.campuses[0].Name != "西土城" {
		t.Fatal("ClassroomService retained a mutable caller campus slice")
	}
}
