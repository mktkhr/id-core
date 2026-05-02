// Package health は /health エンドポイントの HTTP ハンドラを提供する。
//
// M0.1 では `{"status":"ok"}` を返す最小ハンドラのみ。
// 後続マイルストーンで version / dependencies の状態などを追加する想定。
package health

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/mktkhr/id-core/core/internal/logger"
)

// statusOK は /health の正常応答ボディ。
//
// 設計書 Q2 の決定により M0.1 では status のみ返却する。
type statusOK struct {
	Status string `json:"status"`
}

// Handler は GET /health を処理する http.HandlerFunc。
//
// 応答仕様 (設計書 §API 設計 / GET /health):
//   - HTTP 200 OK
//   - Content-Type: application/json; charset=utf-8
//   - Body: {"status":"ok"}
//
// 注意: 本シグネチャは write エラーをログに残さない最小実装。
// production 経路は logger 統合済みの NewHandler を使うこと (server.New 経由)。
func Handler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	// 書き込み失敗 (クライアント切断等) はこの最小版では受動的に握りつぶす。
	// production 経路では NewHandler 経由で logger に WARN 記録する。
	_ = json.NewEncoder(w).Encode(statusOK{Status: "ok"})
}

// NewHandler は logger を受け取り、書き込み失敗時に WARN ログを残す
// production 用 /health ハンドラを返す。
//
// l == nil の場合は契約違反として panic する (シングルポイント設計の前提)。
func NewHandler(l *logger.Logger) http.HandlerFunc {
	if l == nil {
		panic("health.NewHandler: logger must not be nil")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(statusOK{Status: "ok"}); err != nil {
			// レスポンスは既に WriteHeader 済みなので回復不能。受動的にログだけ残す
			// (request_id は logger が context から自動付与する)。
			l.Warn(r.Context(), "/health 応答の書き込みに失敗しました",
				slog.String("error", err.Error()),
			)
		}
	}
}
