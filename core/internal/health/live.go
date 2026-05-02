package health

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/mktkhr/id-core/core/internal/logger"
)

// NewLiveHandler は GET /health/live を処理する http.HandlerFunc を返す (M0.3 Q6)。
//
// 用途: k8s livenessProbe / プロセス疎通確認。
// DB 状態などの依存先はチェックしない (= プロセスが応答できれば常に 200)。
//
// 応答仕様:
//   - HTTP 200 OK
//   - Content-Type: application/json; charset=utf-8
//   - Body: {"status":"ok"}
//
// l == nil の場合は契約違反として panic する (NewHandler / NewReadyHandler と同じ)。
func NewLiveHandler(l *logger.Logger) http.HandlerFunc {
	if l == nil {
		panic("health.NewLiveHandler: logger must not be nil")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(statusOK{Status: "ok"}); err != nil {
			l.Warn(r.Context(), "/health/live 応答の書き込みに失敗しました",
				slog.String("error", err.Error()),
			)
		}
	}
}
