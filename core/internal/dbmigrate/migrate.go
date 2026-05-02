// Package dbmigrate は golang-migrate v4 ライブラリ API のラッパーで、
// マイグレーションの up / down / 整合性チェック (AssertClean) を提供する。
//
// 起動シーケンスの「F-13 start gate」(dirty 検出時に server 起動を中止) として
// `AssertClean` が cmd/core/main.go から呼ばれる (P3 で接続)。
//
// CLI 経由の migrate (`make migrate-up` 等) と本パッケージの library API は同じ
// schema_migrations テーブルを共有するため、運用上 CLI と library を併用しても整合する。
//
// M0.3 で導入。M0.3 のスコープでは smoke table (00000001_smoke_initial) のみが
// migrations 配下に存在する想定で、本格的な実テーブルは M2.x 以降のマイルストーンで追加する。
package dbmigrate

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // postgres driver
	_ "github.com/golang-migrate/migrate/v4/source/file"      // file:// source

	"github.com/mktkhr/id-core/core/internal/logger"
)

// ErrDirty は AssertClean が schema_migrations.dirty=true を検出した時に返す sentinel。
//
// recovery 手順 (運用者向け):
//   - dirty を引き起こした migration を特定 (logs / `migrate version`)
//   - 状態を手動修復 (DDL 巻き戻し、CLI: `make migrate-force VERSION=<n>` で強制リセット)
//   - その上で `make migrate-up` を再実行
var ErrDirty = errors.New("dbmigrate: schema_migrations is dirty (use 'make migrate-force VERSION=<n>' to recover)")

// RunUp は migrate up を library API 経由で実行する (`make migrate-up` の library 等価)。
//
// 主に F-14 double-roundtrip 整合テスト用。サーバー起動経路では使わない (Q9: 起動と migrate を分離)。
//
// 引数:
//   - ctx: cancel 伝播用 context (F-18)。golang-migrate v4 自体は context を直接 honor しないが、
//     プロセス停止と他 stage の整合性のため受け取る。
//   - dsn: PostgreSQL DSN (`postgres://...?sslmode=...`)
//   - sourceURL: マイグレーション SQL ディレクトリへの file:// URL (例: file:///abs/path/to/migrations)
//   - l: 構造化ログ用 logger。nil 不可。
//
// 戻り値:
//   - nil = 全 pending マイグレーションが適用済 (ErrNoChange は no-op として nil 扱い)
//   - error = 接続失敗 / SQL エラー / dirty 状態など
func RunUp(ctx context.Context, dsn string, sourceURL string, l *logger.Logger) error {
	if err := validateArgs(dsn, sourceURL, l); err != nil {
		return err
	}
	m, err := newMigrator(sourceURL, dsn, l)
	if err != nil {
		return err
	}
	defer closeMigrator(ctx, m, l)

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		l.Error(ctx, "migrate up に失敗しました", err)
		return fmt.Errorf("dbmigrate.RunUp: %w", err)
	}
	return nil
}

// RunDown は 1 ステップ分の down を library API 経由で実行する。
//
// 主にテスト用 (F-14 double-roundtrip)。本番運用では `make migrate-down` CLI を利用する。
func RunDown(ctx context.Context, dsn string, sourceURL string, l *logger.Logger) error {
	if err := validateArgs(dsn, sourceURL, l); err != nil {
		return err
	}
	m, err := newMigrator(sourceURL, dsn, l)
	if err != nil {
		return err
	}
	defer closeMigrator(ctx, m, l)

	if err := m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		l.Error(ctx, "migrate down に失敗しました", err)
		return fmt.Errorf("dbmigrate.RunDown: %w", err)
	}
	return nil
}

// AssertClean は schema_migrations.dirty=true を検出した場合に ErrDirty を wrap して返す。
//
// 起動シーケンス (F-13 start gate) で `cmd/core/main.go` から呼ばれる:
//   - clean (dirty=false) → nil
//   - dirty=true → ErrDirty
//   - schema_migrations が存在しない (= 初回起動) → migrate.ErrNilVersion を nil 扱い
//   - その他の接続/library エラー → そのまま wrap して返す
//
// Q9 (起動と migrate 実行分離) のため、本関数は migrate up を**呼ばない**。
// 起動者はあらかじめ `make migrate-up` でマイグレーションを適用しておく必要がある。
func AssertClean(ctx context.Context, dsn string, sourceURL string, l *logger.Logger) error {
	if err := validateArgs(dsn, sourceURL, l); err != nil {
		return err
	}
	m, err := newMigrator(sourceURL, dsn, l)
	if err != nil {
		return err
	}
	defer closeMigrator(ctx, m, l)

	_, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			// schema_migrations が存在しない (初回起動など)。dirty 概念がないので clean 扱い。
			return nil
		}
		l.Error(ctx, "schema_migrations の version 取得に失敗しました", err)
		return fmt.Errorf("dbmigrate.AssertClean: %w", err)
	}
	if dirty {
		return ErrDirty
	}
	return nil
}

// validateArgs は dsn / sourceURL / logger の早期検証を行う (DB 接続前に error を返す)。
func validateArgs(dsn, sourceURL string, l *logger.Logger) error {
	if l == nil {
		return errors.New("dbmigrate: logger は nil にできません")
	}
	if dsn == "" {
		return errors.New("dbmigrate: dsn は空にできません")
	}
	if sourceURL == "" {
		return errors.New("dbmigrate: sourceURL は空にできません")
	}
	return nil
}

// newMigrator は migrate.New を呼び、エラー時にログを記録する薄いラッパー。
// dsn のフルダンプはログに出さない (F-10)。
func newMigrator(sourceURL, dsn string, _ *logger.Logger) (*migrate.Migrate, error) {
	m, err := migrate.New(sourceURL, dsn)
	if err != nil {
		return nil, fmt.Errorf("dbmigrate: migrate.New: %w", err)
	}
	return m, nil
}

// closeMigrator は migrate.Migrate の Close を呼び、エラーはログに記録するのみで panic しない。
func closeMigrator(ctx context.Context, m *migrate.Migrate, l *logger.Logger) {
	srcErr, dbErr := m.Close()
	if srcErr != nil {
		l.Warn(ctx, "dbmigrate: source の close でエラー", "err", srcErr.Error())
	}
	if dbErr != nil {
		l.Warn(ctx, "dbmigrate: database の close でエラー", "err", dbErr.Error())
	}
}
