package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvDoesNotOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("QC_FROM_FILE=file\nQC_ALREADY=from-file\n# comment\n\nexport QC_QUOTED=\"hello\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("QC_ALREADY", "from-env")
	t.Setenv("QC_FROM_FILE", "")
	os.Unsetenv("QC_FROM_FILE")
	os.Unsetenv("QC_QUOTED")
	if err := loadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("QC_FROM_FILE") != "file" {
		t.Fatalf("file value: %q", os.Getenv("QC_FROM_FILE"))
	}
	if os.Getenv("QC_ALREADY") != "from-env" {
		t.Fatalf("override: %q", os.Getenv("QC_ALREADY"))
	}
	if os.Getenv("QC_QUOTED") != "hello" {
		t.Fatalf("quoted: %q", os.Getenv("QC_QUOTED"))
	}
}

func TestLoadRequiresDatabaseAndSecret(t *testing.T) {
	t.Setenv("QC_ENV_FILE", filepath.Join(t.TempDir(), "missing.env"))
	t.Setenv("QC_DATABASE_URL", "")
	t.Setenv("QC_JWT_SECRET", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected missing database to fail")
	}
	t.Setenv("QC_DATABASE_URL", "postgres://qc@127.0.0.1/quality_control")
	t.Setenv("QC_JWT_SECRET", "short")
	if _, err := Load(); err == nil {
		t.Fatal("expected short secret to fail")
	}
	t.Setenv("QC_JWT_SECRET", "0123456789abcdef0123456789abcdef")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != "127.0.0.1:8080" {
		t.Fatalf("addr: %s", cfg.HTTPAddr)
	}
}
