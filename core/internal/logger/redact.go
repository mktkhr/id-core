package logger

import (
	"net/http"
	"strings"
)

// RedactedValue は redact 対象を置換する固定値。
// 長さや存在情報を漏らさないため "****" 等のマスキングではなく固定文字列で置換する (Q8)。
const RedactedValue = "[REDACTED]"

// headerKeysToRedact は HTTP リクエストヘッダの redact 対象 (Q8)。
// 照合は case-insensitive。
//
// 非公開: 外部から書き換えられて redact が無効化される事故を防ぐ。
// 設定変更が必要になった場合は本パッケージ内で更新する (リリースに含める)。
var headerKeysToRedact = []string{
	"Authorization",
	"Cookie",
	"Set-Cookie",
	"Proxy-Authorization",
	"X-Api-Key",
	"X-Auth-Token",
}

// fieldKeysToRedact は body / query / form / details の redact 対象 (Q8)。
// 照合は case-insensitive かつ完全一致 (部分一致禁止)。
//
// 非公開: 外部から書き換えられて redact が無効化される事故を防ぐ。
var fieldKeysToRedact = []string{
	"password",
	"current_password",
	"new_password",
	"access_token",
	"refresh_token",
	"id_token",
	"code",
	"code_verifier",
	"client_secret",
	"assertion",
	"client_assertion",
	"private_key",
	"secret",
	"api_key",
	"jwt",
	"bearer_token",
}

// 内部で使う照合用 set (lower-case 化済み)。
// init で生成し、以降の Redact* 呼び出し中の I/O を抑える。
var (
	headerKeysSet = buildLowerSet(headerKeysToRedact)
	fieldKeysSet  = buildLowerSet(fieldKeysToRedact)
)

func buildLowerSet(keys []string) map[string]struct{} {
	s := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		s[strings.ToLower(k)] = struct{}{}
	}
	return s
}

// RedactHeaders は HTTP ヘッダのディープコピーを作り、deny-list キーの値を [REDACTED] に
// 置換する (Q8)。元の http.Header は変更しない (immutable)。
//
// IsFieldKeyToRedact は body / query / form / details の deny-list (Q8) に
// 含まれるキーかを case-insensitive 完全一致で判定する。
//
// クエリ文字列 / form パラメータの redact 用に他パッケージ (middleware 等) から
// 呼び出される入口。deny-list の二重管理を避けるため必ず本関数を介する。
func IsFieldKeyToRedact(key string) bool {
	_, hit := fieldKeysSet[strings.ToLower(key)]
	return hit
}

// nil を渡すと nil を返す。
func RedactHeaders(h http.Header) http.Header {
	if h == nil {
		return nil
	}
	out := make(http.Header, len(h))
	for k, v := range h {
		if _, hit := headerKeysSet[strings.ToLower(k)]; hit {
			redacted := make([]string, len(v))
			for i := range v {
				redacted[i] = RedactedValue
			}
			out[k] = redacted
			continue
		}
		// 値スライスのコピー (元 slice の変更が反映されないように)。
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// RedactMap は map[string]any を再帰走査し、deny-list キーの値を [REDACTED] に置換した
// 新しい map を返す (Q8)。元の map は変更しない (immutable)。
//
// 配列内の object も再帰走査する。プリミティブ値はそのまま値渡しで保持。
// キー照合は case-insensitive かつ完全一致 (部分一致は誤検知防止のため禁止)。
//
// 入力前提: encoding/json で Unmarshal した map[string]any (json.Number 含む) を想定。
// このため再帰対象の slice 型は []any のみ ([]string / []int 等の typed slice は対象外)。
// 呼び出し元で typed slice を入れる場合、その要素の参照共有は防がない (実害は小)。
//
// nil を渡すと nil を返す。
func RedactMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if _, hit := fieldKeysSet[strings.ToLower(k)]; hit {
			out[k] = RedactedValue
			continue
		}
		out[k] = redactValue(v)
	}
	return out
}

func redactValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return RedactMap(t)
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = redactValue(item)
		}
		return out
	default:
		return v
	}
}
