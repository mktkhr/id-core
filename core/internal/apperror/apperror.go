// Package apperror は内部 API のエラーレスポンス型と JSON シリアライズを提供する。
//
// 仕様 F-7: { "code": string, "message": string, "details"?: object, "request_id": string }
package apperror

import "fmt"

// CodeInternalError は予期しないエラー (panic 含む) の固定 code。
// HTTP middleware の recover からの最終受け皿として使う。
const CodeInternalError = "INTERNAL_ERROR"

// CodedError は内部 API のエラーレスポンスを表す型。
//
// immutable: WithDetails / Wrap は新しいインスタンスを返し、レシーバを変更しない。
// errors.Is / errors.As は Unwrap 経由で cause を辿る。
type CodedError struct {
	code    string
	message string
	details map[string]any
	cause   error
}

// New は CodedError を新規作成する。
//
// code は SCREAMING_SNAKE_CASE を推奨 (例: "INVALID_PARAMETER")。
// message は人間可読の本文 (本スコープでは日本語)。
func New(code, message string) *CodedError {
	return &CodedError{code: code, message: message}
}

// WithDetails は details を付与した新しいインスタンスを返す。
//
// 仕様 F-7 は details を "object / array" に限定するが、本実装は top-level を
// object (map[string]any) に統一する。配列を入れたい場合は object のキー配下に
// ネストする (例: {"errors": [...]})。
//
// immutable 契約のため details は deep-copy する: 呼び出し元が後から map を変更しても
// CodedError 内部状態には影響しない。
func (e *CodedError) WithDetails(details map[string]any) *CodedError {
	cp := *e
	cp.details = cloneDetails(details)
	return &cp
}

// Wrap は原因 error をラップして error chain を作る。
func (e *CodedError) Wrap(cause error) *CodedError {
	cp := *e
	cp.cause = cause
	return &cp
}

// Code は error code を返す。
func (e *CodedError) Code() string { return e.code }

// Message は人間可読メッセージを返す。
func (e *CodedError) Message() string { return e.message }

// Details は details オブジェクトのコピーを返す (nil 可)。
//
// immutable 契約のため deep-copy を返す: 取得側が変更しても CodedError 内部状態には
// 影響しない。シリアライズ目的では Response 経由が推奨。
func (e *CodedError) Details() map[string]any { return cloneDetails(e.details) }

// Error は error インターフェースを満たす。
//
// 注意: クライアント返却用の文字列ではなく、サーバー内部のログ・スタックトレース用。
// 公開エラーレスポンスには Response 経由で出力する。
func (e *CodedError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.code, e.message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.code, e.message)
}

// Unwrap は errors.Is / errors.As のために cause を返す。
func (e *CodedError) Unwrap() error { return e.cause }

// cloneDetails は map[string]any を再帰的にディープコピーする。
// 配下に map[string]any / []any を含むケースを再帰走査する。プリミティブ値は値渡し。
func cloneDetails(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = cloneValue(v)
	}
	return out
}

func cloneValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return cloneDetails(t)
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = cloneValue(item)
		}
		return out
	default:
		return v
	}
}
