package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestLoadEnvFileMissingIsOK(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := loadEnvFile(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadEnvFileDoesNotOverride(t *testing.T) {
	dir := t.TempDir()
	keep := "I0007_KEEP_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	fresh := "I0007_NEW_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	content := keep + "=from-file\n" + fresh + "=from-file\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv(keep, "from-env")
	t.Cleanup(func() { _ = os.Unsetenv(fresh) })

	if err := loadEnvFile(); err != nil {
		t.Fatal(err)
	}
	if os.Getenv(keep) != "from-env" {
		t.Fatalf("%s = %q, want from-env", keep, os.Getenv(keep))
	}
	if os.Getenv(fresh) != "from-file" {
		t.Fatalf("%s = %q, want from-file", fresh, os.Getenv(fresh))
	}
}
