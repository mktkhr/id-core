package db_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/mktkhr/id-core/core/internal/config"
	"github.com/mktkhr/id-core/core/internal/db"
)

// T-65: DSN 組み立て (各 SSLMODE 値で正しい DSN 生成)
func TestBuildDSN_VariousSSLModes(t *testing.T) {
	modes := []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}
	for _, m := range modes {
		t.Run("sslmode="+m, func(t *testing.T) {
			cfg := &config.DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "idcore",
				Password: "secret",
				DBName:   "idcore",
				SSLMode:  m,
			}
			dsn := db.BuildDSN(cfg)
			// url.Parse で DSN として valid か検証
			u, err := url.Parse(dsn)
			if err != nil {
				t.Fatalf("url.Parse(%q): %v", dsn, err)
			}
			if u.Scheme != "postgres" {
				t.Errorf("scheme = %q, want postgres", u.Scheme)
			}
			if got := u.Query().Get("sslmode"); got != m {
				t.Errorf("sslmode = %q, want %q", got, m)
			}
			if u.Host != "localhost:5432" {
				t.Errorf("host = %q, want localhost:5432", u.Host)
			}
		})
	}
}

// T-66: user / password に特殊文字 (`@` `:` `/` `?` `#` `%`) を含めても
// url.QueryEscape で適切にエスケープされ、parse 後に元値が復元される。
func TestBuildDSN_EscapeSpecialChars(t *testing.T) {
	cfg := &config.DatabaseConfig{
		Host:     "db.example",
		Port:     5432,
		User:     "user@with:special/chars?#%",
		Password: "p@ss:word/with?#%",
		DBName:   "id_core_dev",
		SSLMode:  "require",
	}
	dsn := db.BuildDSN(cfg)

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("url.Parse: %v (dsn=%q)", err, dsn)
	}
	if u.User == nil {
		t.Fatalf("u.User is nil")
	}
	if u.User.Username() != cfg.User {
		t.Errorf("user = %q, want %q", u.User.Username(), cfg.User)
	}
	pw, ok := u.User.Password()
	if !ok {
		t.Fatalf("password missing in DSN")
	}
	if pw != cfg.Password {
		t.Errorf("password = %q, want %q", pw, cfg.Password)
	}
}

// T-67 (前段): SafeRepr が password を含まないこと。
func TestSafeRepr_DoesNotIncludePassword(t *testing.T) {
	cfg := &config.DatabaseConfig{
		Host:     "h",
		Port:     5432,
		User:     "u",
		Password: "VERY_SENSITIVE_PASSWORD",
		DBName:   "d",
		SSLMode:  "disable",
	}
	repr := db.SafeRepr(cfg)
	if _, ok := repr["password"]; ok {
		t.Errorf("SafeRepr に password キーが含まれていてはいけない")
	}
	for k, v := range repr {
		if s, ok := v.(string); ok && strings.Contains(s, "VERY_SENSITIVE_PASSWORD") {
			t.Errorf("SafeRepr の値 %s=%q に password が含まれている", k, s)
		}
	}
	// host / dbname / sslmode / user / port は含まれること
	wantKeys := []string{"host", "port", "user", "dbname", "sslmode"}
	for _, k := range wantKeys {
		if _, ok := repr[k]; !ok {
			t.Errorf("SafeRepr に %q キーが含まれているべき", k)
		}
	}
}
