package service

import (
	"strings"
	"testing"

	"BUPT_EC/config"
)

func TestNewClassroomServiceValidatesDependenciesWithoutLeakingOverride(t *testing.T) {
	secretOverride := "constructor-secret-token"
	options := ClassroomServiceOptions{
		Campuses:      []config.CampusConfig{{ID: "01", Name: "西土城"}},
		TokenOverride: secretOverride,
	}
	client := &mockJWClient{}

	tests := []struct {
		name    string
		options ClassroomServiceOptions
		client  JWClient
	}{
		{name: "missing JW client", options: options},
		{name: "missing campuses", options: ClassroomServiceOptions{TokenOverride: secretOverride}, client: client},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClassroomService(tt.options, tt.client)
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
