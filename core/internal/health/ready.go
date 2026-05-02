package health

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mktkhr/id-core/core/internal/logger"
)

// readyTimeout は /health/ready から DB Ping にかける timeout (F-7、Q6)。
//
// LB / k8s readinessProbe は通常 5〜10 秒の interval で polling するため、
// それより十分短い 2 秒で固定する (応答遅延で probe が滞留しないため)。
const readyTimeout = 2 * time.Second

// pingPool は ready handler 内で pool.Ping を呼び出すための最小 interface。
// テストで pgxpool 起動なしに mock 差し替えできるよう抽出している。
type pingPool interface {
	Ping(ctx context.Context) error
}

// statusUnavailable は /health/ready の DB 不可応答ボディ。
//
// F-7 公開粒度下限: どの依存先 / どのバージョン / どの host / どのエラー詳細かは
// レスポンスに含めない。Body は status のみ。
type statusUnavailable struct {
	Status string `json:"status"`
}

// NewReadyHandler は GET /health/ready を処理する http.HandlerFunc を返す (M0.3 Q6 / F-7 / F-10)。
//
// 動作:
//  1. 受信 req.Context() に 2 秒 timeout を被せる
//  2. pool.Ping(ctx) を呼ぶ
//  3. nil なら 200 + {"status":"ok"}
//  4. error なら 503 + {"status":"unavailable"} + 内部 ERROR ログ (host/dbname のみ)
//
// pool == nil または l == nil の場合は契約違反として panic する。
func NewReadyHandler(pool *pgxpool.Pool, l *logger.Logger) http.HandlerFunc {
	if pool == nil {
		panic("health.NewReadyHandler: pool must not be nil")
	}
	if l == nil {
		panic("health.NewReadyHandler: logger must not be nil")
	}
	return readyHandler(pool, l)
}

// readyHandler は pingPool interface を受け取る内部実装 (テスト容易性のため抽出)。
func readyHandler(pool pingPool, l *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), readyTimeout)
		defer cancel()

		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		if err := pool.Ping(ctx); err != nil {
			// F-10 (秘匿担保): logger.Error には err 自体を渡すが、本コードベースが
			// 依存する pgx v5 の Ping エラーは DSN / password を error 文字列に含めない
			// 設計 (jackc/pgx の error 構造)。本 handler は cfg / DSN を保持しないため、
			// 仮に未来のライブラリ更新で err 文字列が DSN を含むよう変更されても、
			// 本 handler 単独では DSN を生成しない。
			// host / dbname の特定情報は db.Open / db.SafeRepr が起動シーケンス時に既にログ済み。
			// → ログ漏洩のリスクは pgx ライブラリ仕様が変わった場合に限定され、
			//   その担保は health/ready_test.go T-98 で sentinel pattern 検査を行っている。
			//
			// errors.Is で context.DeadlineExceeded を分けて記録すると運用診断が容易。
			msg := "DB readiness check failed"
			if errors.Is(err, context.DeadlineExceeded) {
				msg = "DB readiness check timed out"
			}
			l.Error(ctx, msg, err, slog.Duration("timeout", readyTimeout))

			w.WriteHeader(http.StatusServiceUnavailable)
			if encErr := json.NewEncoder(w).Encode(statusUnavailable{Status: "unavailable"}); encErr != nil {
				l.Warn(ctx, "/health/ready (503) 応答の書き込みに失敗しました",
					slog.String("error", encErr.Error()),
				)
			}
			return
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(statusOK{Status: "ok"}); err != nil {
			l.Warn(ctx, "/health/ready (200) 応答の書き込みに失敗しました",
				slog.String("error", err.Error()),
			)
		}
	}
}
