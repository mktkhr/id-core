package dbmigrate_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mktkhr/id-core/core/internal/dbmigrate"
)

// dbDSN は統合テストで利用する PostgreSQL 接続 DSN を取得する。
//
// 環境変数 CORE_TEST_DB_URL が未設定 (= ローカル `make test` 等の DB なし環境) の場合、
// 統合テストは t.Skip で skip する。CI / 開発者の `make test-integration` (P4 で導入)
// では本変数を設定して実行する。
//
//nolint:unused // P4 で利用予定
func dbDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("CORE_TEST_DB_URL")
	if dsn == "" {
		t.Skip("CORE_TEST_DB_URL 未設定のため DB 統合テストを skip (P4 の make test-integration で実行)")
	}
	return dsn
}

// migrationsSourceURL は test 実行ディレクトリを起点に migrations ディレクトリへの file:// URL を返す。
//
//nolint:unused // P4 で利用予定
func migrationsSourceURL(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../../db/migrations")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	return "file://" + abs
}

// T-83: RunUp でスモークテーブルが作成される。
// T-84: RunDown でスモークテーブルが削除される。
func TestRunUpDown_Smoke(t *testing.T) {
	t.Skip("DB 統合テストは P4 (make test-integration) で実装する。本テストは skeleton のみ。")
}

// T-85, T-86, T-87: F-14 double-roundtrip
//   - (a) 各 up/down で object 出現/消失
//   - (b) double-roundtrip 後の schema_migrations が initial と一致 (version=null, dirty=false)
//   - (c) 全工程で no-error
func TestDoubleRoundTrip_F14(t *testing.T) {
	t.Skip("DB 統合テストは P4 で実装する。skeleton のみ。")
}

// T-88: 不正 SQL fixture で RunUp がエラー + dirty 立つ
func TestRunUp_InvalidSQL_LeavesDirty(t *testing.T) {
	t.Skip("DB 統合テストは P4 で実装する。skeleton のみ。")
}

// T-89: dirty 状態で AssertClean が ErrDirty を返す
func TestAssertClean_DirtyReturnsErrDirty(t *testing.T) {
	t.Skip("DB 統合テストは P4 で実装する。skeleton のみ。")
}

// T-90: 正常 migrate up 後の AssertClean が nil を返す
func TestAssertClean_CleanReturnsNil(t *testing.T) {
	t.Skip("DB 統合テストは P4 で実装する。skeleton のみ。")
}

// T-91: BEGIN; ... COMMIT; を含む .up.sql で RunUp がエラー (Q5 (b) 不可)
//
// 設計: golang-migrate は 1 ファイル 1 TX を暗黙的に実施するため、
// SQL 内で BEGIN/COMMIT を書くと PostgreSQL 側でネスト TX エラーになる。
// この観点は double-roundtrip テストとは別 fixture で検証する (P4 で実装)。
func TestRunUp_NestedTransaction_ReturnsError_Q5b(t *testing.T) {
	t.Skip("DB 統合テストは P4 で実装する。skeleton のみ。")
}

// 単体テスト: ErrDirty が単独 sentinel として errors.Is 可能。
func TestErrDirty_IsSentinel(t *testing.T) {
	wrapped := fmt.Errorf("up failed: %w", dbmigrate.ErrDirty)
	if !errors.Is(wrapped, dbmigrate.ErrDirty) {
		t.Errorf("errors.Is(wrapped, ErrDirty) = false, want true")
	}
	// メッセージに recovery hint が含まれること (運用者向け)
	if !strings.Contains(dbmigrate.ErrDirty.Error(), "make migrate-force") {
		t.Errorf("ErrDirty メッセージに 'make migrate-force' を含むべき: got %q", dbmigrate.ErrDirty.Error())
	}
}

// 単体テスト: 関数シグネチャ (F-18) コンパイル時検証 + nil 引数の早期エラー。
func TestSignatures_RequireContextAndLogger(t *testing.T) {
	// 空 dsn / nil logger は AssertClean 内でバリデーションされ、DB 接続前に error 返却すべき。
	err := dbmigrate.AssertClean(context.Background(), "", "file:///tmp/nonexistent", nil)
	if err == nil {
		t.Errorf("AssertClean に空 dsn / nil logger を渡しても error を返さなかった")
	}
}
