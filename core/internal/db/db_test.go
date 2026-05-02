package db_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mktkhr/id-core/core/internal/config"
	"github.com/mktkhr/id-core/core/internal/db"
	"github.com/mktkhr/id-core/core/internal/logger"
)

// T-67: DSN 組み立て / 接続失敗ログに password が含まれない。
//
// 接続できない host/port を指定して Open を失敗させ、buffer-backed logger に
// 出力されたログ全体に password 文字列が含まれていないことを確認する。
func TestOpen_FailureLog_DoesNotLeakPassword(t *testing.T) {
	const sentinel = "VERY_SENSITIVE_TEST_PASSWORD_42"
	cfg := &config.DatabaseConfig{
		// 0.0.0.0:1 は使用不可ポート、すぐに connection refused になる。
		Host:              "127.0.0.1",
		Port:              1,
		User:              "u",
		Password:          sentinel,
		DBName:            "d",
		SSLMode:           "disable",
		MaxConns:          1,
		MinConns:          0,
		MaxConnLifetime:   1 * time.Minute,
		MaxConnIdleTime:   1 * time.Minute,
		HealthCheckPeriod: 1 * time.Minute,
	}

	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pool, err := db.Open(ctx, cfg, l)
	if err == nil {
		// 想定外: 万が一 Open が成功してしまった場合は close
		pool.Close()
		t.Fatalf("接続不能なはずが Open に成功した")
	}

	logged := buf.String()
	if strings.Contains(logged, sentinel) {
		t.Errorf("ログに password sentinel が漏出している:\n%s", logged)
	}
	// 完全な DSN フォーマットが漏れていないか (postgres://...:sentinel@... 等)
	if strings.Contains(logged, "postgres://") {
		t.Errorf("ログに DSN フルダンプが含まれている:\n%s", logged)
	}
}

// T-80: ctx cancel で Open がキャンセル伝播する。
//
// すでに cancel された context を渡し、Open が context.Canceled で返却することを確認する。
func TestOpen_ContextCanceled(t *testing.T) {
	cfg := &config.DatabaseConfig{
		Host:              "127.0.0.1",
		Port:              1,
		User:              "u",
		Password:          "p",
		DBName:            "d",
		SSLMode:           "disable",
		MaxConns:          1,
		MinConns:          0,
		MaxConnLifetime:   1 * time.Minute,
		MaxConnIdleTime:   1 * time.Minute,
		HealthCheckPeriod: 1 * time.Minute,
	}
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 先にキャンセル

	pool, err := db.Open(ctx, cfg, l)
	if err == nil {
		pool.Close()
		t.Fatalf("cancel 済 ctx で Open が成功した")
	}
	// pgx 内部で context.Canceled が wrap される (DeadlineExceeded のケースもありうる)。
	// errors.Is で厳密にチェックし、無関係な別エラーで通過しないようにする。
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, context.Canceled または context.DeadlineExceeded であるべき", err)
	}
}

// 統合テスト (DB 必須): 本 P1 では skip 対応。P4 で実機検証。
func TestOpen_Success_Integration(t *testing.T) {
	t.Skip("DB 統合テストは P4 (test-integration) で実装する")
}
