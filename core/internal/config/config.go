// Package config は core サーバーの実行時設定 (環境変数) の読み込みとバリデーションを担う。
//
// M0.1 で CORE_PORT を導入。M0.3 で CORE_DB_* (接続 + プール) を追加。
// M1.1 で CORE_ENV (strict 3 値) と CORE_OIDC_* (OIDC OP メタデータ + 鍵管理 + Cache-Control max-age)
// を追加。後続マイルストーンで OIDC 関連の追加 env が増える想定。
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultPort は CORE_PORT 未設定時に使用するデフォルトポート。
	DefaultPort = 8080
	// MinPort / MaxPort はリッスンポートとして許容する範囲 (TCP の有効レンジ)。
	MinPort = 1
	MaxPort = 65535

	// プール既定値 (Q11)。
	defaultPoolMaxConns          int32         = 10
	defaultPoolMinConns          int32         = 1
	defaultPoolMaxConnLifetime   time.Duration = 5 * time.Minute
	defaultPoolMaxConnIdleTime   time.Duration = 2 * time.Minute
	defaultPoolHealthCheckPeriod time.Duration = 30 * time.Second

	// SSLMODE 既定値 (Q10、開発環境向け)。
	defaultSSLMode = "disable"

	// OIDC max-age 関連の既定値 / 範囲 (M1.1 設計 #32 確定値)。
	// JWKS は RP 側ライブラリの再取得頻度抑制のため既定 5 分。
	// Discovery は no-cache を既定とし、明示 override 時のみ public, max-age を採用する。
	DefaultJWKSMaxAge      = 300
	DefaultDiscoveryMaxAge = 0
	OIDCMinMaxAge          = 0
	OIDCMaxMaxAge          = 86400
)

// EnvName は CORE_ENV の strict 3 値を表す型 (Q7、論点 #14)。
//
// 不正値や空文字 / unset は config.Load() 段階で起動失敗とする。
// dev / staging / prod 以外を絶対に通さない (例: production / PROD / local 等)。
type EnvName string

const (
	EnvProd    EnvName = "prod"
	EnvStaging EnvName = "staging"
	EnvDev     EnvName = "dev"
)

// validSSLModes は libpq 互換の SSLMODE 6 種ホワイトリスト (Q10)。
var validSSLModes = map[string]struct{}{
	"disable":     {},
	"allow":       {},
	"prefer":      {},
	"require":     {},
	"verify-ca":   {},
	"verify-full": {},
}

// Config は core サーバーの実行時設定を表す。
type Config struct {
	// Env は実行環境識別子 (M1.1 で追加、Q7)。
	Env EnvName

	// Port は HTTP サーバーが Listen するポート番号。
	Port int

	// Database は PostgreSQL 接続および pgxpool 設定 (M0.3 で導入)。
	Database DatabaseConfig

	// OIDC は OIDC OP のメタデータ / 鍵管理 / Cache-Control 設定 (M1.1 で導入)。
	OIDC OIDCConfig
}

// DatabaseConfig は PostgreSQL 接続および pgxpool 設定を表す。
//
// 接続パラメータ (CORE_DB_HOST/PORT/USER/PASSWORD/NAME/SSLMODE) と
// プールパラメータ (CORE_DB_POOL_*) を分離して保持する。
type DatabaseConfig struct {
	// 接続パラメータ
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string

	// プールパラメータ (pgxpool 設定値、Q11 既定値)
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

// OIDCConfig は OIDC OP メタデータ + 鍵管理 + Cache-Control 設定を表す (M1.1 で導入)。
//
// Issuer は末尾スラッシュ strip 済みの正規化値。各 endpoint URL は
//   - env で個別 override されていればその値
//   - 未指定なら url.URL.JoinPath で issuer + 既定 path を組み立てた値
//
// として保持する。Discovery handler は本構造体を読むだけでメタデータを構築できる。
//
// KeyFile と DevGenerateKey は keystore.Init で利用される。
// KeyFile は CORE_ENV=prod で必須、staging/dev では DevGenerateKey=true で代替可能。
// 両方未指定 / 両方指定はいずれも config.Load() 段階で失敗。
type OIDCConfig struct {
	Issuer string

	// 鍵ソース (どちらか一方が必ず有効)
	KeyFile        string // K8s Secret マウントパス等の絶対パス
	DevGenerateKey bool   // 起動時 RSA 2048 bit 鍵生成 (メモリ保持、prod では禁止)

	// kid override (空なら keystore で公開鍵 DER SHA-256 先頭 24 hex を自動算出)
	KeyID string

	// Cache-Control max-age (秒)
	JWKSMaxAge      int
	DiscoveryMaxAge int

	// Discovery レスポンスに広告する各エンドポイント URL (issuer から構築 or env override)
	AuthorizationEndpoint string
	TokenEndpoint         string
	UserInfoEndpoint      string
	JWKSURI               string
}

// Load は環境変数から Config を構築して返す。
//
// 取り扱い:
//   - CORE_ENV → strict 3 値 (prod / staging / dev) 必須、それ以外は起動失敗
//   - CORE_PORT 未設定または空文字 → DefaultPort (8080)
//   - 数値変換できない / 範囲外 (1〜65535 を逸脱) → error
//   - CORE_DB_HOST / PORT / USER / PASSWORD / NAME → 必須、未設定はエラー
//   - CORE_DB_SSLMODE → 未設定なら disable、不正値はエラー
//   - CORE_DB_POOL_* → 未設定なら Q11 既定値、不正値・負値はエラー
//   - CORE_OIDC_ISSUER → 必須。prod/staging で https://、dev のみ http:// 許可、末尾 / を strip
//   - CORE_OIDC_KEY_FILE / CORE_OIDC_DEV_GENERATE_KEY → env 別の必須/代替ルール (M1.1 F-7/F-9)
//   - CORE_OIDC_KEY_ID → 任意 (空なら keystore 側で自動算出)
//   - CORE_OIDC_JWKS_MAX_AGE / CORE_OIDC_DISCOVERY_MAX_AGE → 既定値 / 範囲 0〜86400
//   - CORE_OIDC_AUTHORIZATION_ENDPOINT / CORE_OIDC_TOKEN_ENDPOINT /
//     CORE_OIDC_USERINFO_ENDPOINT / CORE_OIDC_JWKS_URI → 任意 (未設定なら issuer から組み立て)
//
// テスト容易性のため log.Fatal は呼ばず、error として呼び出し元 (main) に返す。
func Load() (*Config, error) {
	env, err := loadEnv()
	if err != nil {
		return nil, err
	}
	port, err := loadPort()
	if err != nil {
		return nil, err
	}
	db, err := loadDatabase()
	if err != nil {
		return nil, err
	}
	oidc, err := loadOIDC(env)
	if err != nil {
		return nil, err
	}
	return &Config{Env: env, Port: port, Database: db, OIDC: oidc}, nil
}

func loadEnv() (EnvName, error) {
	raw, ok := os.LookupEnv("CORE_ENV")
	if !ok || raw == "" {
		return "", fmt.Errorf("CORE_ENV が未設定です (必須、許容値: prod / staging / dev)")
	}
	switch EnvName(raw) {
	case EnvProd, EnvStaging, EnvDev:
		return EnvName(raw), nil
	default:
		return "", fmt.Errorf("CORE_ENV が不正です: %q (許容値: prod / staging / dev)", raw)
	}
}

func loadPort() (int, error) {
	raw, ok := os.LookupEnv("CORE_PORT")
	// 未設定または明示的に空文字が設定されている場合はデフォルト
	if !ok || raw == "" {
		return DefaultPort, nil
	}

	port, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("CORE_PORT が不正です: %q (%d-%d の整数を指定してください): %w", raw, MinPort, MaxPort, err)
	}

	if port < MinPort || port > MaxPort {
		return 0, fmt.Errorf("CORE_PORT が不正です: %d (許容範囲 %d-%d を逸脱しています)", port, MinPort, MaxPort)
	}

	return port, nil
}

func loadDatabase() (DatabaseConfig, error) {
	host, err := requiredString("CORE_DB_HOST")
	if err != nil {
		return DatabaseConfig{}, err
	}
	port, err := requiredPort("CORE_DB_PORT")
	if err != nil {
		return DatabaseConfig{}, err
	}
	user, err := requiredString("CORE_DB_USER")
	if err != nil {
		return DatabaseConfig{}, err
	}
	password, err := requiredString("CORE_DB_PASSWORD")
	if err != nil {
		return DatabaseConfig{}, err
	}
	dbName, err := requiredString("CORE_DB_NAME")
	if err != nil {
		return DatabaseConfig{}, err
	}
	sslMode, err := loadSSLMode()
	if err != nil {
		return DatabaseConfig{}, err
	}
	maxConns, err := positiveInt32("CORE_DB_POOL_MAX_CONNS", defaultPoolMaxConns)
	if err != nil {
		return DatabaseConfig{}, err
	}
	minConns, err := nonNegativeInt32("CORE_DB_POOL_MIN_CONNS", defaultPoolMinConns)
	if err != nil {
		return DatabaseConfig{}, err
	}
	if minConns > maxConns {
		return DatabaseConfig{}, fmt.Errorf("CORE_DB_POOL_MIN_CONNS (%d) は CORE_DB_POOL_MAX_CONNS (%d) 以下である必要があります", minConns, maxConns)
	}
	lifetime, err := nonNegativeDuration("CORE_DB_POOL_MAX_CONN_LIFETIME", defaultPoolMaxConnLifetime)
	if err != nil {
		return DatabaseConfig{}, err
	}
	idle, err := nonNegativeDuration("CORE_DB_POOL_MAX_CONN_IDLE_TIME", defaultPoolMaxConnIdleTime)
	if err != nil {
		return DatabaseConfig{}, err
	}
	hc, err := nonNegativeDuration("CORE_DB_POOL_HEALTH_CHECK_PERIOD", defaultPoolHealthCheckPeriod)
	if err != nil {
		return DatabaseConfig{}, err
	}
	return DatabaseConfig{
		Host:              host,
		Port:              port,
		User:              user,
		Password:          password,
		DBName:            dbName,
		SSLMode:           sslMode,
		MaxConns:          maxConns,
		MinConns:          minConns,
		MaxConnLifetime:   lifetime,
		MaxConnIdleTime:   idle,
		HealthCheckPeriod: hc,
	}, nil
}

func loadOIDC(env EnvName) (OIDCConfig, error) {
	issuer, err := loadIssuer(env)
	if err != nil {
		return OIDCConfig{}, err
	}
	keyFile, devGen, err := loadKeySource(env)
	if err != nil {
		return OIDCConfig{}, err
	}
	keyID := os.Getenv("CORE_OIDC_KEY_ID") // 空なら keystore で自動算出 (F-11)

	jwksMaxAge, err := loadMaxAge("CORE_OIDC_JWKS_MAX_AGE", DefaultJWKSMaxAge)
	if err != nil {
		return OIDCConfig{}, err
	}
	discoveryMaxAge, err := loadMaxAge("CORE_OIDC_DISCOVERY_MAX_AGE", DefaultDiscoveryMaxAge)
	if err != nil {
		return OIDCConfig{}, err
	}

	auth, err := resolveEndpoint(issuer, "CORE_OIDC_AUTHORIZATION_ENDPOINT", "/authorize")
	if err != nil {
		return OIDCConfig{}, err
	}
	token, err := resolveEndpoint(issuer, "CORE_OIDC_TOKEN_ENDPOINT", "/token")
	if err != nil {
		return OIDCConfig{}, err
	}
	userinfo, err := resolveEndpoint(issuer, "CORE_OIDC_USERINFO_ENDPOINT", "/userinfo")
	if err != nil {
		return OIDCConfig{}, err
	}
	jwksURI, err := resolveEndpoint(issuer, "CORE_OIDC_JWKS_URI", "/jwks")
	if err != nil {
		return OIDCConfig{}, err
	}

	return OIDCConfig{
		Issuer:                issuer,
		KeyFile:               keyFile,
		DevGenerateKey:        devGen,
		KeyID:                 keyID,
		JWKSMaxAge:            jwksMaxAge,
		DiscoveryMaxAge:       discoveryMaxAge,
		AuthorizationEndpoint: auth,
		TokenEndpoint:         token,
		UserInfoEndpoint:      userinfo,
		JWKSURI:               jwksURI,
	}, nil
}

// loadIssuer は CORE_OIDC_ISSUER を読み取り、scheme 検証と末尾 / strip を行う (Q13)。
func loadIssuer(env EnvName) (string, error) {
	raw, ok := os.LookupEnv("CORE_OIDC_ISSUER")
	if !ok || raw == "" {
		return "", fmt.Errorf("CORE_OIDC_ISSUER が未設定です (必須)")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("CORE_OIDC_ISSUER が URL として解析できません: %q: %w", raw, err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("CORE_OIDC_ISSUER のスキームは https または http (dev のみ) である必要があります: %q", raw)
	}
	if u.Scheme == "http" && env != EnvDev {
		return "", fmt.Errorf("CORE_OIDC_ISSUER は CORE_ENV=%s では https:// が必須です: %q", env, raw)
	}
	if u.Host == "" {
		return "", fmt.Errorf("CORE_OIDC_ISSUER のホストが空です: %q", raw)
	}
	return strings.TrimRight(raw, "/"), nil
}

// loadKeySource は KEY_FILE / DEV_GENERATE_KEY の組み合わせを env 別に検証する (F-7 / F-9)。
//
//	prod         : KEY_FILE 必須、DEV_GENERATE_KEY=1 は禁止
//	staging/dev  : KEY_FILE か DEV_GENERATE_KEY=1 のいずれか必須 (両方は禁止)
func loadKeySource(env EnvName) (string, bool, error) {
	keyFile := os.Getenv("CORE_OIDC_KEY_FILE")
	devGenRaw, devGenSet := os.LookupEnv("CORE_OIDC_DEV_GENERATE_KEY")
	devGen := false
	if devGenSet && devGenRaw != "" {
		switch devGenRaw {
		case "0":
			devGen = false
		case "1":
			devGen = true
		default:
			return "", false, fmt.Errorf("CORE_OIDC_DEV_GENERATE_KEY が不正です: %q (0 または 1 を指定してください)", devGenRaw)
		}
	}

	if env == EnvProd {
		if devGen {
			return "", false, fmt.Errorf("CORE_OIDC_DEV_GENERATE_KEY=1 は CORE_ENV=prod では許容されません (本番では K8s Secret 経由の鍵ファイル必須)")
		}
		if keyFile == "" {
			return "", false, fmt.Errorf("CORE_OIDC_KEY_FILE が未設定です (CORE_ENV=prod では必須)")
		}
		return keyFile, false, nil
	}

	// staging / dev
	if keyFile == "" && !devGen {
		return "", false, fmt.Errorf("CORE_OIDC_KEY_FILE または CORE_OIDC_DEV_GENERATE_KEY=1 のいずれかを指定してください (CORE_ENV=%s)", env)
	}
	if keyFile != "" && devGen {
		return "", false, fmt.Errorf("CORE_OIDC_KEY_FILE と CORE_OIDC_DEV_GENERATE_KEY=1 は同時指定できません (どちらか一方を選択してください)")
	}
	return keyFile, devGen, nil
}

// loadMaxAge は Cache-Control max-age 系の env を読み取り、範囲 0〜86400 を強制する。
func loadMaxAge(key string, def int) (int, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s が不正です: %q (整数を指定してください): %w", key, raw, err)
	}
	if v < OIDCMinMaxAge || v > OIDCMaxMaxAge {
		return 0, fmt.Errorf("%s が不正です: %d (許容範囲 %d〜%d 秒)", key, v, OIDCMinMaxAge, OIDCMaxMaxAge)
	}
	return v, nil
}

// resolveEndpoint は env override が指定されていればそれを採用し、未指定なら issuer + defaultPath を返す。
//
// url.URL.JoinPath を用いて issuer に path が含まれるケース (例: https://example.com/id-core)
// でも正しく組み立たることを保証する (F-3 / F-17)。
func resolveEndpoint(issuer, key, defaultPath string) (string, error) {
	if raw, ok := os.LookupEnv(key); ok && raw != "" {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return "", fmt.Errorf("%s が URL として不正です: %q (絶対 URL を指定してください)", key, raw)
		}
		return raw, nil
	}
	base, err := url.Parse(issuer)
	if err != nil {
		// loadIssuer 段階で検証済みのため、通常ここには到達しない。
		return "", fmt.Errorf("issuer URL の再解析に失敗しました: %q: %w", issuer, err)
	}
	return base.JoinPath(defaultPath).String(), nil
}

func requiredString(key string) (string, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return "", fmt.Errorf("%s が未設定です (必須)", key)
	}
	return v, nil
}

func requiredPort(key string) (int, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return 0, fmt.Errorf("%s が未設定です (必須)", key)
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s が不正です: %q (%d-%d の整数を指定してください): %w", key, raw, MinPort, MaxPort, err)
	}
	if v < MinPort || v > MaxPort {
		return 0, fmt.Errorf("%s が不正です: %d (許容範囲 %d-%d を逸脱しています)", key, v, MinPort, MaxPort)
	}
	return v, nil
}

func loadSSLMode() (string, error) {
	raw, ok := os.LookupEnv("CORE_DB_SSLMODE")
	if !ok || raw == "" {
		return defaultSSLMode, nil
	}
	if _, ok := validSSLModes[raw]; !ok {
		return "", fmt.Errorf("CORE_DB_SSLMODE が不正です: %q (許容値: disable / allow / prefer / require / verify-ca / verify-full)", raw)
	}
	return raw, nil
}

// positiveInt32 は >0 の int32 値を要求する。
func positiveInt32(key string, def int32) (int32, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return def, nil
	}
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s が不正です: %q (整数を指定してください): %w", key, raw, err)
	}
	if v <= 0 {
		return 0, fmt.Errorf("%s が不正です: %d (1 以上を指定してください)", key, v)
	}
	return int32(v), nil
}

// nonNegativeInt32 は >=0 の int32 値を要求する (MinConns 用)。
func nonNegativeInt32(key string, def int32) (int32, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return def, nil
	}
	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s が不正です: %q (整数を指定してください): %w", key, raw, err)
	}
	if v < 0 {
		return 0, fmt.Errorf("%s が不正です: %d (0 以上を指定してください)", key, v)
	}
	return int32(v), nil
}

// nonNegativeDuration は >=0 の time.Duration 値を要求する。
func nonNegativeDuration(key string, def time.Duration) (time.Duration, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return def, nil
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s が不正です: %q (例: 5m, 30s): %w", key, raw, err)
	}
	if v < 0 {
		return 0, fmt.Errorf("%s が不正です: %v (負数は許容しません)", key, v)
	}
	return v, nil
}
