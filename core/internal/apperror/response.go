package apperror

import (
	"encoding/json"
	"net/http"
)

// Response は F-7 基本形のエラーレスポンス JSON。
//
// request_id は middleware 層で context から取得して詰める責務 (logger.RequestIDFrom)。
type Response struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	RequestID string         `json:"request_id"`
}

// MessageInternalError は CodeInternalError 用のデフォルトメッセージ。
// nil フォールバック時に F-7 の必須フィールド message を空にしないため。
const MessageInternalError = "内部エラーが発生しました"

// ToResponse は CodedError を Response に変換する。
//
// e が nil の場合は INTERNAL_ERROR を返す (recover ミドルウェアの最終フォールバック)。
// その際 message も既定値を埋めて F-7 必須フィールド要件を満たす。
func ToResponse(e *CodedError, requestID string) Response {
	if e == nil {
		return Response{
			Code:      CodeInternalError,
			Message:   MessageInternalError,
			RequestID: requestID,
		}
	}
	return Response{
		Code:      e.code,
		Message:   e.message,
		Details:   cloneDetails(e.details),
		RequestID: requestID,
	}
}

// WriteJSON は w にエラーレスポンスを書き込む。
//
// Content-Type を "application/json; charset=utf-8" に設定し、json.Encoder で
// 改行付き 1 行を書き込む。RFC 8259 で JSON は UTF-8 必須のため charset を付与
// することでブラウザでの安全な解釈を保証する。
//
// 仕様 F-1 のログインジェクション対策として、本関数は string を直接連結せず必ず
// encoding/json 経由で出力する。
func WriteJSON(w http.ResponseWriter, status int, e *CodedError, requestID string) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(ToResponse(e, requestID))
}
