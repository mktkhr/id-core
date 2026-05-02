//go:build integration

package db_test

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mktkhr/id-core/core/internal/config"
	"github.com/mktkhr/id-core/core/internal/db"
	"github.com/mktkhr/id-core/core/internal/logger"
)

// configFromTestEnv は TEST_DATABASE_URL を parse して DatabaseConfig を組み立てる。
// fallback として CORE_DB_* env を直接読む。
func configFromTestEnv(t *testing.T) *config.DatabaseConfig {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://core:core_dev_pw@localhost:5432/id_core_test?sslmode=disable"
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	pw, _ := u.User.Password()
	host := u.Hostname()
	port := 5432
	if p := u.Port(); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}
	return &config.DatabaseConfig{
		Host:              host,
		Port:              port,
		User:              u.User.Username(),
		Password:          pw,
		DBName:            strings.TrimPrefix(u.Path, "/"),
		SSLMode:           u.Query().Get("sslmode"),
		MaxConns:          5,
		MinConns:          0,
		MaxConnLifetime:   1 * time.Minute,
		MaxConnIdleTime:   30 * time.Second,
		HealthCheckPeriod: 30 * time.Second,
	}
}

// T-74: 接続成功 (compose の PostgreSQL 18.3 に Open + Ping 成功)。
func TestOpen_Success_T74(t *testing.T) {
	cfg := configFromTestEnv(t)
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)

	pool, err := db.Open(context.Background(), cfg, l)
	if err != nil {
		t.Fatalf("db.Open failed: %v\nlog=%s", err, buf.String())
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		t.Errorf("Ping after Open: %v", err)
	}
}

// T-75: 接続失敗 (host 不正)。
func TestOpen_HostFailure_T75(t *testing.T) {
	cfg := configFromTestEnv(t)
	cfg.Host = "nonexistent.invalid.example"
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := db.Open(ctx, cfg, l)
	if err == nil {
		pool.Close()
		t.Fatalf("不正 host で Open が成功した")
	}
	// ログに password が漏出していないこと (F-10 / T-78)
	if strings.Contains(buf.String(), cfg.Password) {
		t.Errorf("ログに password が漏出: %s", buf.String())
	}
	if strings.Contains(buf.String(), "postgres://") {
		t.Errorf("ログに DSN フルダンプが漏出: %s", buf.String())
	}
}

// T-76: 接続失敗 (password 不正)。
func TestOpen_PasswordFailure_T76(t *testing.T) {
	cfg := configFromTestEnv(t)
	cfg.Password = "wrong_password_42"
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)

	pool, err := db.Open(context.Background(), cfg, l)
	if err == nil {
		pool.Close()
		t.Fatalf("不正 password で Open が成功した")
	}
	// 設計仕様 F-10 / T-78: ログに password 文字列が漏出しないこと
	if strings.Contains(buf.String(), "wrong_password_42") {
		t.Errorf("ログに password が漏出: %s", buf.String())
	}
}

// T-79: プール設定反映 (MaxConns=5 で Pool.Stat 確認)。
func TestOpen_PoolConfig_T79(t *testing.T) {
	cfg := configFromTestEnv(t)
	cfg.MaxConns = 5
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)

	pool, err := db.Open(context.Background(), cfg, l)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer pool.Close()

	if got := pool.Config().MaxConns; got != 5 {
		t.Errorf("MaxConns = %d, want 5", got)
	}
}

// T-80 強化: ctx cancel で Open がキャンセルされる (DeadlineExceeded)。
func TestOpen_CtxCancel_T80(t *testing.T) {
	cfg := configFromTestEnv(t)
	cfg.Host = "10.255.255.1" // unroutable
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	pool, err := db.Open(ctx, cfg, l)
	if err == nil {
		pool.Close()
		t.Fatalf("unroutable host で Open が成功した")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		// pgx は内部で違う error type を wrap することがあるため、メッセージマッチも許容
		if !strings.Contains(err.Error(), "context") {
			t.Errorf("err = %v, context.{Canceled,DeadlineExceeded} 系を期待", err)
		}
	}
}
