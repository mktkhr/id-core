package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/mktkhr/id-core/core/internal/config"
)

// 注: t.Setenv は t.Parallel と併用不可なため、各サブテストは直列実行する。
//     CORE_PORT / CORE_DB_* 環境変数を扱う性質上、テスト間の干渉を防ぐ目的でも直列が適切。

// setValidDBEnv は CORE_DB_* の必須項目を有効値で設定する。
// CORE_PORT 系テストなど DB 検証が主眼でないテストで利用する。
func setValidDBEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CORE_DB_HOST", "localhost")
	t.Setenv("CORE_DB_PORT", "5432")
	t.Setenv("CORE_DB_USER", "idcore")
	t.Setenv("CORE_DB_PASSWORD", "idcore")
	t.Setenv("CORE_DB_NAME", "idcore")
	// 任意項目は未設定 (既定値が利用される) を想定して空文字を入れる。
	t.Setenv("CORE_DB_SSLMODE", "")
	t.Setenv("CORE_DB_POOL_MAX_CONNS", "")
	t.Setenv("CORE_DB_POOL_MIN_CONNS", "")
	t.Setenv("CORE_DB_POOL_MAX_CONN_LIFETIME", "")
	t.Setenv("CORE_DB_POOL_MAX_CONN_IDLE_TIME", "")
	t.Setenv("CORE_DB_POOL_HEALTH_CHECK_PERIOD", "")
}

func TestLoad_Success(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		setEnv   bool
		wantPort int
	}{
		// T-5: CORE_PORT 未設定 → デフォルト 8080
		{name: "T-5: CORE_PORT 未設定 → デフォルト 8080", setEnv: false, wantPort: 8080},
		// T-6: CORE_PORT=9000 → Port=9000
		{name: "T-6: CORE_PORT=9000 → Port=9000", envValue: "9000", setEnv: true, wantPort: 9000},
		// T-10: 境界値 (CORE_PORT=1)
		{name: "T-10: CORE_PORT=1 (下限) → 正常", envValue: "1", setEnv: true, wantPort: 1},
		// T-10: 境界値 (CORE_PORT=65535)
		{name: "T-10: CORE_PORT=65535 (上限) → 正常", envValue: "65535", setEnv: true, wantPort: 65535},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidDBEnv(t)
			if tt.setEnv {
				t.Setenv("CORE_PORT", tt.envValue)
			} else {
				// 未設定状態を作るため空文字を設定 (Load 側で空文字をデフォルト扱い)
				t.Setenv("CORE_PORT", "")
			}

			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("config.Load() でエラーが返ってはいけない: %v", err)
			}
			if cfg == nil {
				t.Fatalf("config.Load() が nil を返した")
			}
			if cfg.Port != tt.wantPort {
				t.Errorf("cfg.Port = %d, want %d", cfg.Port, tt.wantPort)
			}
		})
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
	}{
		// T-7: 非数値
		{name: "T-7: CORE_PORT=abc → エラー", envValue: "abc"},
		// T-8: 0 (下限未満)
		{name: "T-8: CORE_PORT=0 → エラー", envValue: "0"},
		// T-9: 65536 (上限超)
		{name: "T-9: CORE_PORT=65536 → エラー", envValue: "65536"},
		// 追加: 負数
		{name: "CORE_PORT=-1 → エラー", envValue: "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidDBEnv(t)
			t.Setenv("CORE_PORT", tt.envValue)

			cfg, err := config.Load()
			if err == nil {
				t.Fatalf("config.Load() がエラーを返さなかった: cfg=%+v", cfg)
			}
			// 設計書: メッセージに「CORE_PORT が不正」を含む
			if !strings.Contains(err.Error(), "CORE_PORT が不正") {
				t.Errorf("エラーメッセージに 'CORE_PORT が不正' を含むべき: got %q", err.Error())
			}
		})
	}
}

// T-72: CORE_DB_POOL_* 全て空文字 → 既定値反映 (Q11)
func TestLoad_DatabaseDefaults(t *testing.T) {
	setValidDBEnv(t)
	t.Setenv("CORE_PORT", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() でエラー: %v", err)
	}

	db := cfg.Database
	if db.Host != "localhost" {
		t.Errorf("Host = %q, want %q", db.Host, "localhost")
	}
	if db.Port != 5432 {
		t.Errorf("Port = %d, want 5432", db.Port)
	}
	if db.User != "idcore" {
		t.Errorf("User = %q, want %q", db.User, "idcore")
	}
	if db.Password != "idcore" {
		t.Errorf("Password = %q, want %q", db.Password, "idcore")
	}
	if db.DBName != "idcore" {
		t.Errorf("DBName = %q, want %q", db.DBName, "idcore")
	}
	if db.SSLMode != "disable" {
		t.Errorf("SSLMode = %q, want %q (default)", db.SSLMode, "disable")
	}
	if db.MaxConns != 10 {
		t.Errorf("MaxConns = %d, want 10 (default)", db.MaxConns)
	}
	if db.MinConns != 1 {
		t.Errorf("MinConns = %d, want 1 (default)", db.MinConns)
	}
	if db.MaxConnLifetime != 5*time.Minute {
		t.Errorf("MaxConnLifetime = %v, want 5m (default)", db.MaxConnLifetime)
	}
	if db.MaxConnIdleTime != 2*time.Minute {
		t.Errorf("MaxConnIdleTime = %v, want 2m (default)", db.MaxConnIdleTime)
	}
	if db.HealthCheckPeriod != 30*time.Second {
		t.Errorf("HealthCheckPeriod = %v, want 30s (default)", db.HealthCheckPeriod)
	}
}

// T-73: CORE_DB_POOL_MAX_CONNS=20 → 既定値上書き
func TestLoad_DatabasePoolOverride(t *testing.T) {
	setValidDBEnv(t)
	t.Setenv("CORE_PORT", "")
	t.Setenv("CORE_DB_POOL_MAX_CONNS", "20")
	t.Setenv("CORE_DB_POOL_MIN_CONNS", "5")
	t.Setenv("CORE_DB_POOL_MAX_CONN_LIFETIME", "10m")
	t.Setenv("CORE_DB_POOL_MAX_CONN_IDLE_TIME", "1m")
	t.Setenv("CORE_DB_POOL_HEALTH_CHECK_PERIOD", "15s")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() でエラー: %v", err)
	}

	db := cfg.Database
	if db.MaxConns != 20 {
		t.Errorf("MaxConns = %d, want 20", db.MaxConns)
	}
	if db.MinConns != 5 {
		t.Errorf("MinConns = %d, want 5", db.MinConns)
	}
	if db.MaxConnLifetime != 10*time.Minute {
		t.Errorf("MaxConnLifetime = %v, want 10m", db.MaxConnLifetime)
	}
	if db.MaxConnIdleTime != 1*time.Minute {
		t.Errorf("MaxConnIdleTime = %v, want 1m", db.MaxConnIdleTime)
	}
	if db.HealthCheckPeriod != 15*time.Second {
		t.Errorf("HealthCheckPeriod = %v, want 15s", db.HealthCheckPeriod)
	}
}

// T-68: CORE_DB_SSLMODE=invalid_value → Load() がエラー
func TestLoad_InvalidSSLMode(t *testing.T) {
	setValidDBEnv(t)
	t.Setenv("CORE_PORT", "")
	t.Setenv("CORE_DB_SSLMODE", "invalid_value")

	_, err := config.Load()
	if err == nil {
		t.Fatalf("config.Load() がエラーを返さなかった")
	}
	if !strings.Contains(err.Error(), "CORE_DB_SSLMODE") {
		t.Errorf("エラーメッセージに 'CORE_DB_SSLMODE' を含むべき: got %q", err.Error())
	}
}

// SSLMODE 6 種を全て許容することを確認
func TestLoad_AllValidSSLModes(t *testing.T) {
	modes := []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}
	for _, m := range modes {
		t.Run("sslmode="+m, func(t *testing.T) {
			setValidDBEnv(t)
			t.Setenv("CORE_PORT", "")
			t.Setenv("CORE_DB_SSLMODE", m)

			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("config.Load() でエラー: %v", err)
			}
			if cfg.Database.SSLMode != m {
				t.Errorf("SSLMode = %q, want %q", cfg.Database.SSLMode, m)
			}
		})
	}
}

// T-69: CORE_DB_PORT 不正値
func TestLoad_InvalidDBPort(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
	}{
		{name: "非数値", envValue: "abc"},
		{name: "0 (下限未満)", envValue: "0"},
		{name: "65536 (上限超)", envValue: "65536"},
		{name: "負数", envValue: "-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidDBEnv(t)
			t.Setenv("CORE_PORT", "")
			t.Setenv("CORE_DB_PORT", tt.envValue)

			_, err := config.Load()
			if err == nil {
				t.Fatalf("config.Load() がエラーを返さなかった")
			}
			if !strings.Contains(err.Error(), "CORE_DB_PORT") {
				t.Errorf("エラーメッセージに 'CORE_DB_PORT' を含むべき: got %q", err.Error())
			}
		})
	}
}

// T-70: CORE_DB_POOL_MAX_CONNS=-1 (負数) → error
func TestLoad_InvalidPoolMaxConns(t *testing.T) {
	tests := []struct {
		name string
		env  string
		val  string
	}{
		{name: "MAX_CONNS 負数", env: "CORE_DB_POOL_MAX_CONNS", val: "-1"},
		{name: "MAX_CONNS 0", env: "CORE_DB_POOL_MAX_CONNS", val: "0"},
		{name: "MIN_CONNS 負数", env: "CORE_DB_POOL_MIN_CONNS", val: "-1"},
		{name: "MAX_CONNS 非数値", env: "CORE_DB_POOL_MAX_CONNS", val: "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidDBEnv(t)
			t.Setenv("CORE_PORT", "")
			t.Setenv(tt.env, tt.val)

			_, err := config.Load()
			if err == nil {
				t.Fatalf("config.Load() がエラーを返さなかった")
			}
			if !strings.Contains(err.Error(), tt.env) {
				t.Errorf("エラーメッセージに %q を含むべき: got %q", tt.env, err.Error())
			}
		})
	}
}

// T-71: CORE_DB_POOL_MAX_CONN_LIFETIME=invalid (Duration parse エラー) + 負 Duration
func TestLoad_InvalidPoolDuration(t *testing.T) {
	tests := []struct {
		name string
		env  string
		val  string
	}{
		{name: "LIFETIME parse error", env: "CORE_DB_POOL_MAX_CONN_LIFETIME", val: "invalid"},
		{name: "IDLE_TIME parse error", env: "CORE_DB_POOL_MAX_CONN_IDLE_TIME", val: "abc"},
		{name: "HEALTH_CHECK parse error", env: "CORE_DB_POOL_HEALTH_CHECK_PERIOD", val: "5x"},
		{name: "LIFETIME 負数", env: "CORE_DB_POOL_MAX_CONN_LIFETIME", val: "-5m"},
		{name: "IDLE_TIME 負数", env: "CORE_DB_POOL_MAX_CONN_IDLE_TIME", val: "-1s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidDBEnv(t)
			t.Setenv("CORE_PORT", "")
			t.Setenv(tt.env, tt.val)

			_, err := config.Load()
			if err == nil {
				t.Fatalf("config.Load() がエラーを返さなかった")
			}
			if !strings.Contains(err.Error(), tt.env) {
				t.Errorf("エラーメッセージに %q を含むべき: got %q", tt.env, err.Error())
			}
		})
	}
}

// 必須 env 未設定でエラー
func TestLoad_MissingRequiredDBEnv(t *testing.T) {
	required := []string{"CORE_DB_HOST", "CORE_DB_PORT", "CORE_DB_USER", "CORE_DB_PASSWORD", "CORE_DB_NAME"}
	for _, key := range required {
		t.Run(key+" 未設定", func(t *testing.T) {
			setValidDBEnv(t)
			t.Setenv("CORE_PORT", "")
			t.Setenv(key, "")

			_, err := config.Load()
			if err == nil {
				t.Fatalf("config.Load() がエラーを返さなかった")
			}
			if !strings.Contains(err.Error(), key) {
				t.Errorf("エラーメッセージに %q を含むべき: got %q", key, err.Error())
			}
		})
	}
}
