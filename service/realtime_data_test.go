package service

import (
	"BUPT_EC/cache"
	"BUPT_EC/config"
	"BUPT_EC/logs"
	"context"
	"os"
	"testing"
)

func init() {
	logs.Init(false)
	config.InitConfig()
	cache.InitCache()
}

func requireJWCredentials(t *testing.T) {
	t.Helper()
	if os.Getenv(LoginUsernameKey) == "" || os.Getenv(LoginPasswordKey) == "" {
		t.Skip("JW_USERNAME and JW_PASSWORD are required for integration login/query tests")
	}
}

func TestEncryptJWPassword(t *testing.T) {
	encrypted, err := encryptJWPassword("test-password")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == "" {
		t.Fatal("encrypted password should not be empty")
	}
	if encrypted == "test-password" {
		t.Fatal("encrypted password should not equal plaintext")
	}
}

func TestParseRoom(t *testing.T) {
	building, room, displayName, capacity, ok := parseRoom("教学实验综合楼-N104(229)")
	if !ok {
		t.Fatal("expected room to parse")
	}
	if building != "教学实验综合楼" || room != "N104" || displayName != "教学实验综合楼-N104" || capacity != 229 {
		t.Fatalf("unexpected parsed room: %q %q %q %d", building, room, displayName, capacity)
	}

	building, room, displayName, capacity, ok = parseRoom("未来学习大楼-202-203(60)")
	if !ok {
		t.Fatal("expected merged room to parse")
	}
	if building != "未来学习大楼" || room != "202-203" || displayName != "未来学习大楼-202-203" || capacity != 60 {
		t.Fatalf("unexpected parsed merged room: %q %q %q %d", building, room, displayName, capacity)
	}
}

func TestLogin(t *testing.T) {
	requireJWCredentials(t)
	ResetRuntimeStateForTest()
	err := Login(context.Background())
	if err != nil {
		t.Error(err)
	}
}

func TestQueryOne(t *testing.T) {
	requireJWCredentials(t)
	ResetRuntimeStateForTest()
	err := Login(context.Background())
	if err != nil {
		t.Error(err)
	}
	rows, err := QueryOne(context.Background(), "01")
	if err != nil {
		t.Error(err)
	}
	if len(rows) == 0 {
		t.Error("expected classroom rows")
	}
}

func TestQueryAll(t *testing.T) {
	requireJWCredentials(t)
	ResetRuntimeStateForTest()
	err := Login(context.Background())
	if err != nil {
		t.Error(err)
	}
	ans, err := QueryAll(context.Background())
	if err != nil {
		t.Error(err)
	}
	if ans == nil {
		t.Fatal("expected response")
	}
	if len(ans.Campuses) != 2 {
		t.Fatalf("expected 2 campuses, got %d", len(ans.Campuses))
	}
}
