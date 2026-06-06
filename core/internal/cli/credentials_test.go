package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestFallbackDeepgramAPIKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := saveFallbackDeepgramAPIKey(" test-key ")
	if err != nil {
		t.Fatalf("save fallback key: %v", err)
	}

	expectedPath := filepath.Join(home, configDirName, configFileName)
	if path != expectedPath {
		t.Fatalf("fallback path = %q, want %q", path, expectedPath)
	}

	configDirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat config dir: %v", err)
	}
	if got := configDirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("config dir permissions = %04o, want 0700", got)
	}

	configInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config file: %v", err)
	}
	if got := configInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("config file permissions = %04o, want 0600", got)
	}

	key, err := loadFallbackDeepgramAPIKey()
	if err != nil {
		t.Fatalf("load fallback key: %v", err)
	}
	if key != "test-key" {
		t.Fatalf("fallback key = %q, want %q", key, "test-key")
	}

	removed, removedPath, err := deleteFallbackDeepgramAPIKey()
	if err != nil {
		t.Fatalf("delete fallback key: %v", err)
	}
	if !removed {
		t.Fatal("delete fallback key removed = false, want true")
	}
	if removedPath != expectedPath {
		t.Fatalf("removed path = %q, want %q", removedPath, expectedPath)
	}

	_, err = loadFallbackDeepgramAPIKey()
	if !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("load deleted fallback key error = %v, want ErrNotFound", err)
	}
}

func TestLoadFallbackDeepgramAPIKeyMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	_, err := loadFallbackDeepgramAPIKey()
	if !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("load missing fallback key error = %v, want ErrNotFound", err)
	}
}

func TestIsKeyringUnavailable(t *testing.T) {
	err := errors.New("The name org.freedesktop.secrets was not provided by any .service files")
	if !isKeyringUnavailable(err) {
		t.Fatal("expected org.freedesktop.secrets error to be treated as unavailable")
	}

	if isKeyringUnavailable(keyring.ErrNotFound) {
		t.Fatal("ErrNotFound should not be treated as unavailable")
	}
}
