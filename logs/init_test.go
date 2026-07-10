package logs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setMainLogDirForTest(t *testing.T, dir string) {
	t.Helper()
	previous := mainLogDir
	mainLogDir = dir
	t.Cleanup(func() {
		mainLogDir = previous
	})
}

func TestInitSucceedsWithoutFileWriter(t *testing.T) {
	if err := Init(false, false); err != nil {
		t.Fatalf("Init(false) error = %v", err)
	}
}

func TestInitFailsWhenLogDirectoryCannotBeCreated(t *testing.T) {
	// Use a regular file path so MkdirAll cannot create the log directory,
	// without touching the repository run_log/ path.
	tmp := t.TempDir()
	blocker := filepath.Join(tmp, "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("setup blocker: %v", err)
	}
	setMainLogDirForTest(t, blocker)

	err := Init(true, false)
	if err == nil {
		t.Fatal("Init(true) expected error when log directory cannot be created")
	}
	if !strings.Contains(err.Error(), "create log directory") {
		t.Fatalf("Init error = %v, want create log directory context", err)
	}
}
