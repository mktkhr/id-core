// Package health は /health エンドポイントの HTTP ハンドラを提供する。
//
// M0.1 では `{"status":"ok"}` を返す最小ハンドラのみ。
// 後続マイルストーンで version / dependencies の状態などを追加する想定。
package health

import (
	"encoding/json"
	"net/http"
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
func Handler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	// json.Encoder のエラーは書き込み済みヘッダに対しては復旧不能なため、
	// 取りこぼしを避けるべく明示的に握りつぶす (構造化ログ整備は M0.2)。
	_ = json.NewEncoder(w).Encode(statusOK{Status: "ok"})
}
