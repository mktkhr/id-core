//go:build integration

package dbmigrate_test

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mktkhr/id-core/core/internal/dbmigrate"
	"github.com/mktkhr/id-core/core/internal/logger"
	"github.com/mktkhr/id-core/core/internal/testutil/dbtest"
)

// migrationsURL は test 実行時の migrations ディレクトリへの絶対 file URL。
func migrationsURL(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../../db/migrations")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	return "file://" + abs
}

// T-83: RunUp で smoke table が作成される。
// T-84: RunDown で smoke table が削除される。
func TestRunUpDown_Smoke_T83T84(t *testing.T) {
	dsn := dbtest.DatabaseURL()
	ctx, pool := dbtest.NewPool(t)
	src := migrationsURL(t)
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)

	// 初期状態を整える: schema_migrations を含む全テーブル drop は test-integration target で完了済
	// 本テストは「Up でテーブルが現れる / Down で消える」のみを検証する。

	if err := dbmigrate.RunUp(ctx, dsn, src, l); err != nil {
		t.Fatalf("RunUp: %v", err)
	}
	// Up 後: schema_smoke が存在
	if !tableExists(t, ctx, pool, "schema_smoke") {
		t.Errorf("Up 後に schema_smoke が存在しない")
	}

	if err := dbmigrate.RunDown(ctx, dsn, src, l); err != nil {
		t.Fatalf("RunDown: %v", err)
	}
	// Down 後: schema_smoke が削除済
	if tableExists(t, ctx, pool, "schema_smoke") {
		t.Errorf("Down 後に schema_smoke が残存している")
	}

	// 後続テストのため Up 状態に戻す
	if err := dbmigrate.RunUp(ctx, dsn, src, l); err != nil {
		t.Fatalf("RunUp (restore): %v", err)
	}
}

// T-85, T-86, T-87: F-14 double-roundtrip
func TestDoubleRoundTrip_F14_T85T86T87(t *testing.T) {
	dsn := dbtest.DatabaseURL()
	ctx, _ := dbtest.NewPool(t)
	src := migrationsURL(t)
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)

	// Up → Down → Up → Down を順次実施し、全ステップで no error を assert (T-87)
	steps := []struct {
		name string
		fn   func() error
	}{
		{"Up #1", func() error { return dbmigrate.RunUp(ctx, dsn, src, l) }},
		{"Down #1", func() error { return dbmigrate.RunDown(ctx, dsn, src, l) }},
		{"Up #2", func() error { return dbmigrate.RunUp(ctx, dsn, src, l) }},
		{"Down #2", func() error { return dbmigrate.RunDown(ctx, dsn, src, l) }},
	}
	for _, s := range steps {
		if err := s.fn(); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
	}

	// T-86: double-roundtrip 後の AssertClean が nil (clean state)
	if err := dbmigrate.AssertClean(ctx, dsn, src, l); err != nil {
		t.Errorf("AssertClean after double-roundtrip: %v", err)
	}

	// 後続テストのため Up 状態に戻す
	if err := dbmigrate.RunUp(ctx, dsn, src, l); err != nil {
		t.Fatalf("RunUp (restore): %v", err)
	}
}

// T-90: 正常 migrate up 後の AssertClean が nil
func TestAssertClean_CleanReturnsNil_T90(t *testing.T) {
	dsn := dbtest.DatabaseURL()
	ctx, _ := dbtest.NewPool(t)
	src := migrationsURL(t)
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)

	if err := dbmigrate.RunUp(ctx, dsn, src, l); err != nil {
		t.Fatalf("RunUp: %v", err)
	}
	if err := dbmigrate.AssertClean(ctx, dsn, src, l); err != nil {
		t.Errorf("AssertClean (clean state): %v", err)
	}
}

// T-89: dirty 状態で AssertClean が ErrDirty を返す
func TestAssertClean_DirtyReturnsErrDirty_T89(t *testing.T) {
	dsn := dbtest.DatabaseURL()
	ctx, pool := dbtest.NewPool(t)
	src := migrationsURL(t)
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)

	// 状態を up に揃え、その上で dirty フラグを SQL で立てる
	if err := dbmigrate.RunUp(ctx, dsn, src, l); err != nil {
		t.Fatalf("RunUp: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE schema_migrations SET dirty=true WHERE version=1"); err != nil {
		t.Fatalf("force dirty: %v", err)
	}
	defer func() {
		// recover for subsequent tests
		_, _ = pool.Exec(ctx, "UPDATE schema_migrations SET dirty=false WHERE version=1")
	}()

	err := dbmigrate.AssertClean(ctx, dsn, src, l)
	if !errors.Is(err, dbmigrate.ErrDirty) {
		t.Errorf("AssertClean = %v, want errors.Is(_, ErrDirty)", err)
	}
}

// tableExists は information_schema.tables から指定テーブルの有無を返す。
func tableExists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string) bool {
	t.Helper()
	var exists bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)`, name).Scan(&exists)
	if err != nil {
		t.Fatalf("tableExists query: %v", err)
	}
	return exists
}
