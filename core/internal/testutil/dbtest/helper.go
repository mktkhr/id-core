// Package dbtest はバックエンド統合テストの DB 接続ヘルパー。
//
// 想定: `make test-integration` (build tag = integration) で有効化される。
// テスト実行モード:
//   - CI (TEST_DB_REQUIRED=1): DB 接続失敗時に t.Fatal でテスト失敗扱い
//   - ローカル (TEST_DB_REQUIRED 未設定): DB 接続失敗時に t.Skip で skip
//
// 使い方:
//
//	ctx, pool := dbtest.NewPool(t)
//	tx := dbtest.BeginTx(t, ctx, pool)
//	defer dbtest.RollbackTx(t, ctx, tx)
//	// tx 経由でテスト用 INSERT / SELECT
//
// 並列実行 (T-81): `t.Parallel()` を呼ぶサブテストも、互いに別 TX で隔離されているため
// 観測しない。`make test-integration` は `-p 1` を指定して package 単位は順次実行する
// (テーブル truncate 等のグローバル状態を共有するパッケージが将来導入された場合の安全性確保)。
package dbtest

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// defaultTestDatabaseURL は TEST_DATABASE_URL 未設定時の fallback。
// 開発用 docker compose とは別 DB (`id_core_test`) を想定。
const defaultTestDatabaseURL = "postgres://core:core_dev_pw@localhost:5432/id_core_test?sslmode=disable"

// DatabaseURL は統合テスト用の DSN を返す。TEST_DATABASE_URL 優先、未設定なら fallback。
//
// 注意: 本関数は `*testing.T` も `context.Context` も受け取らない。F-18 (全公開関数の
// 第 1 引数は context.Context) はテスト用ヘルパーには厳格適用しない方針:
//   - testutil/dbtest の API は Go の `httptest` 等の慣習に倣い、`*testing.T` を起点として
//     ctx は ヘルパー側が `context.Background()` で生成 + return する形 (NewPool 参照)
//   - 環境変数 / fallback 文字列の単純取得である本関数は I/O / cancel 観測がなく、ctx 不要
//
// この方針は設計書 #21 line 273-289 で `(ctx, *pgxpool.Pool) = NewPool(t)` と定義された
// API シグネチャを優先する判断に基づく (line 537 の F-18 と Go テスト慣習との整合)。
func DatabaseURL() string {
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		return dsn
	}
	return defaultTestDatabaseURL
}

// dbRequired は CI などで DB を必須扱いにするかを判定する。
// CI では `TEST_DB_REQUIRED=1` を設定し、接続失敗を取り逃がさない運用とする。
func dbRequired() bool {
	return os.Getenv("TEST_DB_REQUIRED") == "1"
}

// NewPool は TEST_DATABASE_URL から *pgxpool.Pool を生成し、初回 Ping で接続性を検証する。
//
// 動作モード:
//   - TEST_DB_REQUIRED=1 (CI): 接続失敗時に t.Fatal
//   - TEST_DB_REQUIRED 未設定 (ローカル): 接続失敗時に t.Skip
//
// pool は t.Cleanup で自動 Close される。テスト側は手動で Close しなくてよい。
func NewPool(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, DatabaseURL())
	if err != nil {
		if dbRequired() {
			t.Fatalf("テスト DB 接続に失敗しました (TEST_DB_REQUIRED=1): %v", err)
		}
		t.Skipf("テスト DB に接続できないためスキップします: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		if dbRequired() {
			t.Fatalf("テスト DB Ping に失敗しました (TEST_DB_REQUIRED=1): %v", err)
		}
		t.Skipf("テスト DB Ping 失敗のためスキップします: %v", err)
	}
	t.Cleanup(pool.Close)
	return ctx, pool
}

// BeginTx は pool.Begin(ctx) で TX を開始する。失敗時は t.Fatal。
//
// 利用パターン:
//
//	tx := dbtest.BeginTx(t, ctx, pool)
//	defer dbtest.RollbackTx(t, ctx, tx)
//
// テスト終了時は必ず Rollback されるため、テスト間で残留 state が発生しない (T-82)。
func BeginTx(t *testing.T, ctx context.Context, pool *pgxpool.Pool) pgx.Tx {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	return tx
}

// RollbackTx は tx.Rollback(ctx) を呼び、tx が既に閉じている場合は許容する。
//
// 主に defer 用途。Commit 済の TX に対する Rollback は pgx.ErrTxClosed を返すが、
// これは正常な状態として無視する。
func RollbackTx(t *testing.T, ctx context.Context, tx pgx.Tx) {
	t.Helper()
	if tx == nil {
		return
	}
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		t.Errorf("RollbackTx: %v", err)
	}
}
