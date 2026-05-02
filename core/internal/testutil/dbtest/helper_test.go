//go:build integration

package dbtest_test

import (
	"sync/atomic"
	"testing"

	"github.com/mktkhr/id-core/core/internal/testutil/dbtest"
)

// T-81: 並列 TX 隔離。
//
// 同一 pool から 2 つの subtest が `t.Parallel()` で別 TX を開始し、
// 各々で smoke table に INSERT した値が互いに観測されないことを検証する。
//
// 設計上の注意: `t.Run` で `t.Parallel` を呼ぶサブテストは、親テスト関数が return
// した後にスケジューリングされるため、親テスト末尾で集計値を assert すると
// サブテスト未完了の状態で評価されうる (Go testing パッケージの仕様)。
// そのため "group" 親サブテストでサブテスト群を囲み、その内部で `t.Run + t.Parallel`
// を走らせる構造にして、`group` の return 時点で全並列サブテストの完了を保証する。
//
// 各 TX は最後に Rollback されるため、テスト後に DB に残留しない (T-82 と整合)。
func TestParallelTxIsolation_T81(t *testing.T) {
	ctx, pool := dbtest.NewPool(t)

	cases := []struct {
		name  string
		label string
	}{
		{name: "subtest_a", label: "T81-A"},
		{name: "subtest_b", label: "T81-B"},
	}
	var seenOthers atomic.Int32
	t.Run("parallel_group", func(g *testing.T) {
		for _, c := range cases {
			c := c
			g.Run(c.name, func(t *testing.T) {
				t.Parallel()
				tx := dbtest.BeginTx(t, ctx, pool)
				defer dbtest.RollbackTx(t, ctx, tx)

				// 自身の TX で INSERT
				if _, err := tx.Exec(ctx,
					"INSERT INTO schema_smoke (label, note) VALUES ($1, $2)",
					c.label, "isolated"); err != nil {
					t.Fatalf("INSERT (%s): %v", c.name, err)
				}

				// 相手 subtest の label が見えないこと (TX 隔離 = READ COMMITTED デフォルトでも未 Commit は不可視)
				otherLabel := "T81-A"
				if c.label == "T81-A" {
					otherLabel = "T81-B"
				}
				var count int
				if err := tx.QueryRow(ctx,
					"SELECT COUNT(*) FROM schema_smoke WHERE label = $1", otherLabel).
					Scan(&count); err != nil {
					t.Fatalf("SELECT (%s): %v", c.name, err)
				}
				if count > 0 {
					seenOthers.Add(1)
					t.Errorf("自 TX から相手の uncommitted INSERT (%s) が見えた: count=%d", otherLabel, count)
				}
			})
		}
	})
	// parallel_group が return した時点で全並列サブテスト完了が保証されている。
	if seenOthers.Load() > 0 {
		t.Errorf("並列 TX 隔離違反 が %d 件発生", seenOthers.Load())
	}
}

// T-82: 失敗後の残留 state なし。
//
// 1 つ目の subtest で INSERT 後に Rollback (defer)。
// 2 つ目の subtest で同じ label が DB に存在しないことを assert する。
func TestNoResidualStateAfterRollback_T82(t *testing.T) {
	ctx, pool := dbtest.NewPool(t)

	const label = "T82-residual-check-sentinel"

	t.Run("first: insert and rollback", func(t *testing.T) {
		tx := dbtest.BeginTx(t, ctx, pool)
		defer dbtest.RollbackTx(t, ctx, tx)

		if _, err := tx.Exec(ctx,
			"INSERT INTO schema_smoke (label, note) VALUES ($1, $2)",
			label, "should-be-rolled-back"); err != nil {
			t.Fatalf("INSERT: %v", err)
		}
		// Rollback は defer で実行
	})

	t.Run("second: no residual rows", func(t *testing.T) {
		tx := dbtest.BeginTx(t, ctx, pool)
		defer dbtest.RollbackTx(t, ctx, tx)

		var count int
		if err := tx.QueryRow(ctx,
			"SELECT COUNT(*) FROM schema_smoke WHERE label = $1", label).
			Scan(&count); err != nil {
			t.Fatalf("SELECT: %v", err)
		}
		if count != 0 {
			t.Errorf("Rollback 後の残留 state を検出: count=%d (label=%s)", count, label)
		}
	})
}
