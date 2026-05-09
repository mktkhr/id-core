package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/mktkhr/id-core/core/internal/config"
)

// 注: t.Setenv は t.Parallel と併用不可なため、各サブテストは直列実行する。
//     CORE_PORT / CORE_DB_* 環境変数を扱う性質上、テスト間の干渉を防ぐ目的でも直列が適切。

// setValidDBEnv は config.Load() が成功するために必須となる「ベース env」を全て設定する。
//
// 含むもの:
//   - DB 必須 (CORE_DB_HOST/PORT/USER/PASSWORD/NAME)
//   - DB 任意 (CORE_DB_SSLMODE / POOL_*) は空文字 (= 既定値採用)
//   - CORE_ENV=dev (M1.1 で必須化)
//   - CORE_OIDC_ISSUER (M1.1 で必須化、dev は http:// 許可)
//   - CORE_OIDC_DEV_GENERATE_KEY=1 (M1.1 で staging/dev に必須、prod では禁止)
//   - その他 OIDC 任意 env は空文字 (既定値 / 自動算出)
//
// 名前は M0.3 由来 (DB セットアップが起源) のままだが、M1.1 以降は「Load 成功の最低限 env」
// を提供する汎用ヘルパー。OIDC 個別検証テストは本ヘルパー呼び出し後に t.Setenv で上書きする。
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

	// M1.1 で必須化された env (CORE_ENV / CORE_OIDC_*)。
	// dev + 起動時鍵生成モードで Load() が成功する最小組み合わせ。
	t.Setenv("CORE_ENV", "dev")
	t.Setenv("CORE_OIDC_ISSUER", "http://localhost:8080")
	t.Setenv("CORE_OIDC_KEY_FILE", "")
	t.Setenv("CORE_OIDC_DEV_GENERATE_KEY", "1")
	t.Setenv("CORE_OIDC_KEY_ID", "")
	t.Setenv("CORE_OIDC_JWKS_MAX_AGE", "")
	t.Setenv("CORE_OIDC_DISCOVERY_MAX_AGE", "")
	t.Setenv("CORE_OIDC_AUTHORIZATION_ENDPOINT", "")
	t.Setenv("CORE_OIDC_TOKEN_ENDPOINT", "")
	t.Setenv("CORE_OIDC_USERINFO_ENDPOINT", "")
	t.Setenv("CORE_OIDC_JWKS_URI", "")
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

// MinConns > MaxConns でバリデーションエラー
func TestLoad_MinConnsExceedsMaxConns(t *testing.T) {
	setValidDBEnv(t)
	t.Setenv("CORE_PORT", "")
	t.Setenv("CORE_DB_POOL_MAX_CONNS", "5")
	t.Setenv("CORE_DB_POOL_MIN_CONNS", "10")

	_, err := config.Load()
	if err == nil {
		t.Fatalf("config.Load() がエラーを返さなかった")
	}
	if !strings.Contains(err.Error(), "CORE_DB_POOL_MIN_CONNS") {
		t.Errorf("エラーメッセージに 'CORE_DB_POOL_MIN_CONNS' を含むべき: got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "CORE_DB_POOL_MAX_CONNS") {
		t.Errorf("エラーメッセージに 'CORE_DB_POOL_MAX_CONNS' を含むべき: got %q", err.Error())
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

// ----------------------------------------------------------------------------
// M1.1 (#32): CORE_ENV strict + CORE_OIDC_* テスト群
// ----------------------------------------------------------------------------

// CORE_ENV strict: prod / staging / dev のみ許容。
func TestLoad_EnvStrict_AcceptsValidValues(t *testing.T) {
	cases := []struct {
		envValue string
		want     config.EnvName
		// prod では CORE_OIDC_DEV_GENERATE_KEY が禁止のため、prod の場合だけ
		// CORE_OIDC_KEY_FILE をダミー値で設定する。
		isProd bool
	}{
		{envValue: "dev", want: config.EnvDev},
		{envValue: "staging", want: config.EnvStaging},
		{envValue: "prod", want: config.EnvProd, isProd: true},
	}
	for _, tc := range cases {
		t.Run("CORE_ENV="+tc.envValue, func(t *testing.T) {
			setValidDBEnv(t)
			t.Setenv("CORE_PORT", "")
			t.Setenv("CORE_ENV", tc.envValue)
			if tc.isProd {
				// prod では https:// + KEY_FILE 必須、DEV_GENERATE_KEY=0 必須。
				t.Setenv("CORE_OIDC_ISSUER", "https://id.example.com")
				t.Setenv("CORE_OIDC_KEY_FILE", "/etc/id-core/keys/signing.pem")
				t.Setenv("CORE_OIDC_DEV_GENERATE_KEY", "0")
			} else if tc.envValue == "staging" {
				// staging も https:// 必須。
				t.Setenv("CORE_OIDC_ISSUER", "https://id-staging.example.com")
			}

			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("config.Load() でエラー: %v", err)
			}
			if cfg.Env != tc.want {
				t.Errorf("Env = %q, want %q", cfg.Env, tc.want)
			}
		})
	}
}

func TestLoad_EnvStrict_RejectsInvalidValues(t *testing.T) {
	cases := []string{
		"",           // 空文字
		"production", // 紛らわしい
		"PROD",       // 大文字
		"Dev",        // mixed case
		"local",      // 未定義値
		"staging ",   // trailing space
	}
	for _, v := range cases {
		t.Run("CORE_ENV="+v, func(t *testing.T) {
			setValidDBEnv(t)
			t.Setenv("CORE_PORT", "")
			t.Setenv("CORE_ENV", v)

			_, err := config.Load()
			if err == nil {
				t.Fatalf("config.Load() がエラーを返さなかった")
			}
			if !strings.Contains(err.Error(), "CORE_ENV") {
				t.Errorf("エラーメッセージに 'CORE_ENV' を含むべき: got %q", err.Error())
			}
		})
	}
}

// CORE_OIDC_ISSUER scheme + 末尾 / strip。
func TestLoad_Issuer_NormalizationAndScheme(t *testing.T) {
	cases := []struct {
		name      string
		env       string
		issuer    string
		wantValue string // 空 = エラー想定
		wantErr   bool
	}{
		{name: "dev: http 許可", env: "dev", issuer: "http://localhost:8080", wantValue: "http://localhost:8080"},
		{name: "dev: https 許可", env: "dev", issuer: "https://localhost:8080", wantValue: "https://localhost:8080"},
		{name: "dev: 末尾 / strip", env: "dev", issuer: "http://localhost:8080/", wantValue: "http://localhost:8080"},
		{name: "dev: subpath + 末尾 / strip", env: "dev", issuer: "http://localhost:8080/id-core/", wantValue: "http://localhost:8080/id-core"},
		{name: "staging: https 必須", env: "staging", issuer: "https://id-staging.example.com", wantValue: "https://id-staging.example.com"},
		{name: "staging: http は失敗", env: "staging", issuer: "http://id-staging.example.com", wantErr: true},
		{name: "prod: https 必須", env: "prod", issuer: "https://id.example.com", wantValue: "https://id.example.com"},
		{name: "prod: http は失敗", env: "prod", issuer: "http://id.example.com", wantErr: true},
		{name: "scheme 不正 (ftp)", env: "dev", issuer: "ftp://example.com", wantErr: true},
		{name: "scheme なし", env: "dev", issuer: "example.com", wantErr: true},
		{name: "host 空", env: "dev", issuer: "https://", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setValidDBEnv(t)
			t.Setenv("CORE_PORT", "")
			t.Setenv("CORE_ENV", tc.env)
			t.Setenv("CORE_OIDC_ISSUER", tc.issuer)
			if tc.env == "prod" {
				t.Setenv("CORE_OIDC_KEY_FILE", "/etc/id-core/keys/signing.pem")
				t.Setenv("CORE_OIDC_DEV_GENERATE_KEY", "0")
			}

			cfg, err := config.Load()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("config.Load() がエラーを返さなかった (cfg=%+v)", cfg)
				}
				if !strings.Contains(err.Error(), "CORE_OIDC_ISSUER") {
					t.Errorf("エラーメッセージに 'CORE_OIDC_ISSUER' を含むべき: got %q", err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("config.Load() でエラー: %v", err)
			}
			if cfg.OIDC.Issuer != tc.wantValue {
				t.Errorf("Issuer = %q, want %q", cfg.OIDC.Issuer, tc.wantValue)
			}
		})
	}
}

func TestLoad_Issuer_MissingFails(t *testing.T) {
	setValidDBEnv(t)
	t.Setenv("CORE_PORT", "")
	t.Setenv("CORE_OIDC_ISSUER", "")

	_, err := config.Load()
	if err == nil {
		t.Fatalf("config.Load() がエラーを返さなかった")
	}
	if !strings.Contains(err.Error(), "CORE_OIDC_ISSUER") {
		t.Errorf("エラーメッセージに 'CORE_OIDC_ISSUER' を含むべき: got %q", err.Error())
	}
}

// 鍵ソース (KEY_FILE / DEV_GENERATE_KEY) の env 別ルール (F-7 / F-9)。
func TestLoad_KeySource_PerEnvRules(t *testing.T) {
	cases := []struct {
		name       string
		env        string
		issuer     string
		keyFile    string
		devGen     string
		wantErr    bool
		wantFile   string
		wantDevGen bool
	}{
		// dev
		{name: "dev: 両方未設定 → 失敗", env: "dev", issuer: "http://localhost:8080", wantErr: true},
		{name: "dev: KEY_FILE のみ → OK", env: "dev", issuer: "http://localhost:8080", keyFile: "/tmp/dev.pem", wantFile: "/tmp/dev.pem"},
		{name: "dev: DEV_GENERATE_KEY=1 のみ → OK", env: "dev", issuer: "http://localhost:8080", devGen: "1", wantDevGen: true},
		{name: "dev: 両方指定 → 失敗", env: "dev", issuer: "http://localhost:8080", keyFile: "/tmp/dev.pem", devGen: "1", wantErr: true},
		{name: "dev: DEV_GENERATE_KEY 不正値 (2) → 失敗", env: "dev", issuer: "http://localhost:8080", devGen: "2", wantErr: true},
		// staging
		{name: "staging: 両方未設定 → 失敗", env: "staging", issuer: "https://staging.example.com", wantErr: true},
		{name: "staging: KEY_FILE のみ → OK", env: "staging", issuer: "https://staging.example.com", keyFile: "/etc/keys/signing.pem", wantFile: "/etc/keys/signing.pem"},
		{name: "staging: DEV_GENERATE_KEY=1 のみ → OK", env: "staging", issuer: "https://staging.example.com", devGen: "1", wantDevGen: true},
		// prod
		{name: "prod: KEY_FILE 未設定 → 失敗", env: "prod", issuer: "https://id.example.com", wantErr: true},
		{name: "prod: DEV_GENERATE_KEY=1 → 失敗", env: "prod", issuer: "https://id.example.com", devGen: "1", wantErr: true},
		{name: "prod: KEY_FILE のみ → OK", env: "prod", issuer: "https://id.example.com", keyFile: "/etc/keys/signing.pem", wantFile: "/etc/keys/signing.pem"},
		{name: "prod: KEY_FILE + DEV_GENERATE_KEY=0 → OK", env: "prod", issuer: "https://id.example.com", keyFile: "/etc/keys/signing.pem", devGen: "0", wantFile: "/etc/keys/signing.pem"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setValidDBEnv(t)
			t.Setenv("CORE_PORT", "")
			t.Setenv("CORE_ENV", tc.env)
			t.Setenv("CORE_OIDC_ISSUER", tc.issuer)
			t.Setenv("CORE_OIDC_KEY_FILE", tc.keyFile)
			t.Setenv("CORE_OIDC_DEV_GENERATE_KEY", tc.devGen)

			cfg, err := config.Load()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("config.Load() がエラーを返さなかった (cfg=%+v)", cfg)
				}
				return
			}
			if err != nil {
				t.Fatalf("config.Load() でエラー: %v", err)
			}
			if cfg.OIDC.KeyFile != tc.wantFile {
				t.Errorf("KeyFile = %q, want %q", cfg.OIDC.KeyFile, tc.wantFile)
			}
			if cfg.OIDC.DevGenerateKey != tc.wantDevGen {
				t.Errorf("DevGenerateKey = %v, want %v", cfg.OIDC.DevGenerateKey, tc.wantDevGen)
			}
		})
	}
}

// CORE_OIDC_KEY_ID の任意 override。
func TestLoad_KeyID_Optional(t *testing.T) {
	t.Run("未設定 → 空 (keystore で自動算出)", func(t *testing.T) {
		setValidDBEnv(t)
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("config.Load() でエラー: %v", err)
		}
		if cfg.OIDC.KeyID != "" {
			t.Errorf("KeyID = %q, want empty (auto-derive)", cfg.OIDC.KeyID)
		}
	})
	t.Run("設定 → そのまま採用", func(t *testing.T) {
		setValidDBEnv(t)
		t.Setenv("CORE_OIDC_KEY_ID", "custom-kid-2026-05")
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("config.Load() でエラー: %v", err)
		}
		if cfg.OIDC.KeyID != "custom-kid-2026-05" {
			t.Errorf("KeyID = %q, want %q", cfg.OIDC.KeyID, "custom-kid-2026-05")
		}
	})
}

// max-age 既定値と範囲検証。
func TestLoad_MaxAge_Defaults(t *testing.T) {
	setValidDBEnv(t)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() でエラー: %v", err)
	}
	if cfg.OIDC.JWKSMaxAge != config.DefaultJWKSMaxAge {
		t.Errorf("JWKSMaxAge = %d, want %d (default)", cfg.OIDC.JWKSMaxAge, config.DefaultJWKSMaxAge)
	}
	if cfg.OIDC.DiscoveryMaxAge != config.DefaultDiscoveryMaxAge {
		t.Errorf("DiscoveryMaxAge = %d, want %d (default)", cfg.OIDC.DiscoveryMaxAge, config.DefaultDiscoveryMaxAge)
	}
}

func TestLoad_MaxAge_RangeValidation(t *testing.T) {
	cases := []struct {
		name    string
		key     string
		val     string
		wantErr bool
	}{
		{name: "JWKS 0 (下限) → OK", key: "CORE_OIDC_JWKS_MAX_AGE", val: "0"},
		{name: "JWKS 86400 (上限) → OK", key: "CORE_OIDC_JWKS_MAX_AGE", val: "86400"},
		{name: "JWKS -1 → エラー", key: "CORE_OIDC_JWKS_MAX_AGE", val: "-1", wantErr: true},
		{name: "JWKS 86401 → エラー", key: "CORE_OIDC_JWKS_MAX_AGE", val: "86401", wantErr: true},
		{name: "JWKS 非数値 → エラー", key: "CORE_OIDC_JWKS_MAX_AGE", val: "abc", wantErr: true},
		{name: "Discovery 0 (下限) → OK", key: "CORE_OIDC_DISCOVERY_MAX_AGE", val: "0"},
		{name: "Discovery 86400 (上限) → OK", key: "CORE_OIDC_DISCOVERY_MAX_AGE", val: "86400"},
		{name: "Discovery -1 → エラー", key: "CORE_OIDC_DISCOVERY_MAX_AGE", val: "-1", wantErr: true},
		{name: "Discovery 86401 → エラー", key: "CORE_OIDC_DISCOVERY_MAX_AGE", val: "86401", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setValidDBEnv(t)
			t.Setenv(tc.key, tc.val)

			_, err := config.Load()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("config.Load() がエラーを返さなかった")
				}
				if !strings.Contains(err.Error(), tc.key) {
					t.Errorf("エラーメッセージに %q を含むべき: got %q", tc.key, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("config.Load() でエラー: %v", err)
			}
		})
	}
}

// endpoint URL: 未指定なら issuer から JoinPath で構築 (subpath 透過、F-3 / F-17)。
func TestLoad_EndpointResolution_DefaultsFromIssuer(t *testing.T) {
	cases := []struct {
		name             string
		issuer           string
		wantAuthorize    string
		wantToken        string
		wantUserInfo     string
		wantJWKSURI      string
		setupExtraDevEnv bool
	}{
		{
			name:          "標準 (path なし)",
			issuer:        "https://id.example.com",
			wantAuthorize: "https://id.example.com/authorize",
			wantToken:     "https://id.example.com/token",
			wantUserInfo:  "https://id.example.com/userinfo",
			wantJWKSURI:   "https://id.example.com/jwks",
		},
		{
			name:          "subpath あり",
			issuer:        "https://example.com/id-core",
			wantAuthorize: "https://example.com/id-core/authorize",
			wantToken:     "https://example.com/id-core/token",
			wantUserInfo:  "https://example.com/id-core/userinfo",
			wantJWKSURI:   "https://example.com/id-core/jwks",
		},
		{
			name:             "dev 非標準ポート",
			issuer:           "http://localhost:8080",
			wantAuthorize:    "http://localhost:8080/authorize",
			wantToken:        "http://localhost:8080/token",
			wantUserInfo:     "http://localhost:8080/userinfo",
			wantJWKSURI:      "http://localhost:8080/jwks",
			setupExtraDevEnv: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setValidDBEnv(t)
			if !tc.setupExtraDevEnv {
				// dev 以外のテスト (https) は CORE_ENV を維持して KEY_FILE を入れる。
				t.Setenv("CORE_ENV", "staging")
			}
			t.Setenv("CORE_OIDC_ISSUER", tc.issuer)

			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("config.Load() でエラー: %v", err)
			}
			if cfg.OIDC.AuthorizationEndpoint != tc.wantAuthorize {
				t.Errorf("AuthorizationEndpoint = %q, want %q", cfg.OIDC.AuthorizationEndpoint, tc.wantAuthorize)
			}
			if cfg.OIDC.TokenEndpoint != tc.wantToken {
				t.Errorf("TokenEndpoint = %q, want %q", cfg.OIDC.TokenEndpoint, tc.wantToken)
			}
			if cfg.OIDC.UserInfoEndpoint != tc.wantUserInfo {
				t.Errorf("UserInfoEndpoint = %q, want %q", cfg.OIDC.UserInfoEndpoint, tc.wantUserInfo)
			}
			if cfg.OIDC.JWKSURI != tc.wantJWKSURI {
				t.Errorf("JWKSURI = %q, want %q", cfg.OIDC.JWKSURI, tc.wantJWKSURI)
			}
		})
	}
}

func TestLoad_EndpointResolution_OverrideTakesPrecedence(t *testing.T) {
	setValidDBEnv(t)
	t.Setenv("CORE_OIDC_AUTHORIZATION_ENDPOINT", "https://auth.example.com/authorize")
	t.Setenv("CORE_OIDC_TOKEN_ENDPOINT", "https://token.example.com/token")
	t.Setenv("CORE_OIDC_USERINFO_ENDPOINT", "https://userinfo.example.com/userinfo")
	t.Setenv("CORE_OIDC_JWKS_URI", "https://jwks.example.com/jwks.json")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() でエラー: %v", err)
	}
	if cfg.OIDC.AuthorizationEndpoint != "https://auth.example.com/authorize" {
		t.Errorf("AuthorizationEndpoint override 不適用: got %q", cfg.OIDC.AuthorizationEndpoint)
	}
	if cfg.OIDC.TokenEndpoint != "https://token.example.com/token" {
		t.Errorf("TokenEndpoint override 不適用: got %q", cfg.OIDC.TokenEndpoint)
	}
	if cfg.OIDC.UserInfoEndpoint != "https://userinfo.example.com/userinfo" {
		t.Errorf("UserInfoEndpoint override 不適用: got %q", cfg.OIDC.UserInfoEndpoint)
	}
	if cfg.OIDC.JWKSURI != "https://jwks.example.com/jwks.json" {
		t.Errorf("JWKSURI override 不適用: got %q", cfg.OIDC.JWKSURI)
	}
}

func TestLoad_EndpointResolution_InvalidOverride(t *testing.T) {
	cases := []struct {
		name string
		key  string
		val  string
	}{
		{name: "scheme なし", key: "CORE_OIDC_AUTHORIZATION_ENDPOINT", val: "/authorize"},
		{name: "host なし", key: "CORE_OIDC_TOKEN_ENDPOINT", val: "https://"},
		{name: "URL parse 不能", key: "CORE_OIDC_USERINFO_ENDPOINT", val: "://invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setValidDBEnv(t)
			t.Setenv(tc.key, tc.val)

			_, err := config.Load()
			if err == nil {
				t.Fatalf("config.Load() がエラーを返さなかった")
			}
			if !strings.Contains(err.Error(), tc.key) {
				t.Errorf("エラーメッセージに %q を含むべき: got %q", tc.key, err.Error())
			}
		})
	}
}
