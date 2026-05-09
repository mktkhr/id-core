// Package notimpl は M1.1 で広告するが M1.2-1.4 で実装される OIDC エンドポイント
// (`/authorize`, `/token`, `/userinfo`) の **503 Service Unavailable** stub を提供する (F-23)。
//
// OIDC Discovery 1.0 で REQUIRED な endpoint を Discovery 広告と一致させつつ、
// 実 endpoint は HTTP 503 + 機械可読 JSON `{"error":"endpoint_not_implemented","available_at":"M1.x"}`
// を返すことで、RP が「広告と現状の不一致」を確実に判別できる (Codex HIGH 反映)。
//
// 設計判断 (論点 / Codex 反映):
//   - `Retry-After` ヘッダは付けない: RFC 7231 §7.1.3 で値は HTTP-date / delta-seconds のみ
//     許容され、`M1.2` のような文字列は規約違反 (doc-review HIGH 2)
//   - `Cache-Control: no-store`: stub は将来 200 に切り替わるため絶対にキャッシュさせない
//   - body の `error` フィールドは snake_case (`endpoint_not_implemented`、論点 #8 二重化方針)
//   - apperror.WriteJSON は使わない: apperror は `code/message/request_id` 形式で OIDC 標準と
//     異なる。本 stub は OIDC レスポンス慣習に近い `error` / `available_at` 形式
package notimpl

import (
	"encoding/json"
	"net/http"
)

// errorMessage は 503 stub 全 endpoint で共通の error フィールド値 (snake_case 固定)。
const errorMessage = "endpoint_not_implemented"

// Handler は milestone (`M1.2` / `M1.3` / `M1.4`) を埋め込んだ 503 stub HTTP ハンドラを返す (F-23)。
//
// 同じ stub を複数 endpoint で使い回すため、milestone を引数で受けてクロージャを返すファクトリ。
//
// メソッドは不問 (GET / POST / PUT / DELETE すべて同じ 503 を返す)。
// 本実装が来たら router 側で method ごとに別 handler を登録するため、stub 段階では
// method 制限を入れない (RP 側ライブラリが POST を試した場合も明確な未実装通知を返したい)。
func Handler(milestone string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = r // メソッド・パスは見ない (notimpl は何が来ても 503)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		// Retry-After ヘッダは付与しない (RFC 7231 §7.1.3 違反回避、doc-review HIGH 2)

		w.WriteHeader(http.StatusServiceUnavailable)

		// json.NewEncoder + Encode は最後に改行を付ける (RFC 7159 互換)。
		// プレーン json.Marshal でも良いが、stream 書き込みで一貫性を確保。
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":        errorMessage,
			"available_at": milestone,
		})
	}
}
