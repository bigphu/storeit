package database

import (
	"os"
	"path/filepath"
	"testing"
)

// Docker secret có dòng mới ở cuối; mật khẩu không được gồm "\n"
func writeSecret(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "pg_app_pw.txt")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseConfig_PasswordFile(t *testing.T) {
	t.Setenv("PGPASSWORD", "")
	t.Setenv("PGPASSWORD_FILE", writeSecret(t, "s3cret\n"))

	cfg, err := parseConfig("postgres://app_user@localhost:5432/storeit")
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.ConnConfig.Password; got != "s3cret" {
		t.Errorf("password = %q, want s3cret", got)
	}
}

// Mật khẩu ghi rõ (trong URL hay PGPASSWORD) thắng file
func TestParseConfig_ExplicitPasswordWins(t *testing.T) {
	t.Setenv("PGPASSWORD_FILE", writeSecret(t, "from-file"))

	t.Run("url", func(t *testing.T) {
		t.Setenv("PGPASSWORD", "")
		cfg, err := parseConfig("postgres://app_user:from-url@localhost/storeit")
		if err != nil {
			t.Fatal(err)
		}
		if got := cfg.ConnConfig.Password; got != "from-url" {
			t.Errorf("password = %q, want from-url", got)
		}
	})
	t.Run("PGPASSWORD", func(t *testing.T) {
		t.Setenv("PGPASSWORD", "from-env")
		cfg, err := parseConfig("postgres://app_user@localhost/storeit")
		if err != nil {
			t.Fatal(err)
		}
		if got := cfg.ConnConfig.Password; got != "from-env" {
			t.Errorf("password = %q, want from-env", got)
		}
	})
}

func TestParseConfig_MissingPasswordFile(t *testing.T) {
	t.Setenv("PGPASSWORD", "")
	t.Setenv("PGPASSWORD_FILE", filepath.Join(t.TempDir(), "nope.txt"))

	if _, err := parseConfig("postgres://app_user@localhost/storeit"); err == nil {
		t.Error("want error for missing PGPASSWORD_FILE")
	}
}
