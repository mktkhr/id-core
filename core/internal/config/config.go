// Package config は core サーバーの実行時設定 (環境変数) の読み込みとバリデーションを担う。
//
// M0.1 で CORE_PORT を導入。M0.3 で CORE_DB_* (接続 + プール) を追加。
// 後続マイルストーンで OIDC 鍵パス等が追加される想定。
package config

import (
	"fmt"
	"os"
	"strconv"
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
	// Port は HTTP サーバーが Listen するポート番号。
	Port int

	// Database は PostgreSQL 接続および pgxpool 設定 (M0.3 で導入)。
	Database DatabaseConfig
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

// Load は環境変数から Config を構築して返す。
//
// 取り扱い:
//   - CORE_PORT 未設定または空文字 → DefaultPort (8080)
//   - 数値変換できない / 範囲外 (1〜65535 を逸脱) → error
//   - CORE_DB_HOST / PORT / USER / PASSWORD / NAME → 必須、未設定はエラー
//   - CORE_DB_SSLMODE → 未設定なら disable、不正値はエラー
//   - CORE_DB_POOL_* → 未設定なら Q11 既定値、不正値・負値はエラー
//
// テスト容易性のため log.Fatal は呼ばず、error として呼び出し元 (main) に返す。
func Load() (*Config, error) {
	port, err := loadPort()
	if err != nil {
		return nil, err
	}
	db, err := loadDatabase()
	if err != nil {
		return nil, err
	}
	return &Config{Port: port, Database: db}, nil
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
