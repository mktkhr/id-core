package logger_test

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/mktkhr/id-core/core/internal/apperror"
	"github.com/mktkhr/id-core/core/internal/logger"
)

// T-10: HTTP ヘッダ Authorization: Bearer xxx を redact。
func TestRedactHeaders_Authorization(t *testing.T) {
	h := http.Header{}
	h.Set("Authorization", "Bearer secret-token")
	h.Set("X-Trace-Id", "trace-1")

	got := logger.RedactHeaders(h)

	if v := got.Get("Authorization"); v != logger.RedactedValue {
		t.Errorf("Authorization = %q, want %q", v, logger.RedactedValue)
	}
	if v := got.Get("X-Trace-Id"); v != "trace-1" {
		t.Errorf("X-Trace-Id should not be redacted, got %q", v)
	}
}

// T-11: ヘッダ照合は case-insensitive。
func TestRedactHeaders_CaseInsensitive(t *testing.T) {
	// http.Header.Set は標準的に Title-Case 化するので、Add で生 case を入れる。
	h := http.Header{
		"authorization":       []string{"bearer xxx"},
		"AUTHORIZATION":       []string{"BEARER yyy"},
		"Proxy-Authorization": []string{"Basic zzz"},
	}

	got := logger.RedactHeaders(h)

	for k, vs := range got {
		for _, v := range vs {
			if v != logger.RedactedValue {
				t.Errorf("header %q value %q should be redacted", k, v)
			}
		}
	}
}

// T-16: redact 対象キーの全 6 ヘッダを網羅 (case 違いも含めて)。
func TestRedactHeaders_AllDenyListed(t *testing.T) {
	cases := []string{
		"Authorization",
		"Cookie",
		"Set-Cookie",
		"Proxy-Authorization",
		"X-Api-Key",
		"X-Auth-Token",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			h := http.Header{}
			h.Set(name, "secret-value")
			got := logger.RedactHeaders(h)
			if v := got.Get(name); v != logger.RedactedValue {
				t.Errorf("%s = %q, want %q", name, v, logger.RedactedValue)
			}
		})
	}
}

// T-12: JSON body {"password": "secret"} を redact。
func TestRedactMap_PasswordTopLevel(t *testing.T) {
	in := map[string]any{"password": "secret", "username": "alice"}
	got := logger.RedactMap(in)

	if got["password"] != logger.RedactedValue {
		t.Errorf("password should be redacted, got %v", got["password"])
	}
	if got["username"] != "alice" {
		t.Errorf("username should not be touched, got %v", got["username"])
	}

	// 元の map は変更されていない (immutable)。
	if in["password"] != "secret" {
		t.Errorf("source map mutated: %v", in)
	}
}

// T-13: ネストした JSON で client_secret が走査される。
func TestRedactMap_NestedObject(t *testing.T) {
	in := map[string]any{
		"outer": map[string]any{
			"client_secret": "shh",
			"client_id":     "public",
		},
	}
	got := logger.RedactMap(in)

	outer, ok := got["outer"].(map[string]any)
	if !ok {
		t.Fatalf("outer should be map[string]any, got %T", got["outer"])
	}
	if outer["client_secret"] != logger.RedactedValue {
		t.Errorf("client_secret should be redacted, got %v", outer["client_secret"])
	}
	if outer["client_id"] != "public" {
		t.Errorf("client_id should be untouched, got %v", outer["client_id"])
	}
}

// T-14: 配列内の object も再帰走査される。
func TestRedactMap_NestedArray(t *testing.T) {
	in := map[string]any{
		"creds": []any{
			map[string]any{"access_token": "a"},
			map[string]any{"refresh_token": "b"},
			"raw-string-element",
		},
	}
	got := logger.RedactMap(in)

	arr, ok := got["creds"].([]any)
	if !ok || len(arr) != 3 {
		t.Fatalf("creds should be []any of len 3, got %T", got["creds"])
	}
	if arr[0].(map[string]any)["access_token"] != logger.RedactedValue {
		t.Errorf("creds[0].access_token should be redacted, got %v", arr[0])
	}
	if arr[1].(map[string]any)["refresh_token"] != logger.RedactedValue {
		t.Errorf("creds[1].refresh_token should be redacted, got %v", arr[1])
	}
	if arr[2] != "raw-string-element" {
		t.Errorf("scalar element mutated: %v", arr[2])
	}
}

// T-15: redact 対象キー全 16 フィールドを網羅 (case-insensitive 完全一致)。
func TestRedactMap_AllDenyListedFields(t *testing.T) {
	keys := []string{
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
	for _, k := range keys {
		t.Run(k, func(t *testing.T) {
			in := map[string]any{k: "value-of-" + k}
			got := logger.RedactMap(in)
			if got[k] != logger.RedactedValue {
				t.Errorf("%s should be redacted, got %v", k, got[k])
			}
		})
		// case 違い (大文字化) でも同じ。
		t.Run(k+" (UPPER)", func(t *testing.T) {
			upperKey := upper(k)
			in := map[string]any{upperKey: "value"}
			got := logger.RedactMap(in)
			if got[upperKey] != logger.RedactedValue {
				t.Errorf("%s (upper) should be redacted, got %v", upperKey, got[upperKey])
			}
		})
	}
}

// T-17: 部分一致禁止 — accessusername は access_token に誤検知しない。
func TestRedactMap_NoSubstringMatch(t *testing.T) {
	in := map[string]any{
		"accessusername":  "alice",  // access_token に部分一致しない
		"mypassword":      "secret", // password に部分一致しない
		"client_secret_x": "kept",   // client_secret に部分一致しない
	}
	got := logger.RedactMap(in)

	for k, want := range map[string]any{
		"accessusername":  "alice",
		"mypassword":      "secret",
		"client_secret_x": "kept",
	} {
		if got[k] != want {
			t.Errorf("substring match should not redact %q: got %v, want %v", k, got[k], want)
		}
	}
}

// T-18: 対象キーが存在しないとき出力は元の構造のまま (不要な置換なし)。
func TestRedactMap_NoTarget_NoChange(t *testing.T) {
	in := map[string]any{
		"username": "alice",
		"email":    "a@example.com",
		"meta": map[string]any{
			"locale": "ja",
		},
	}
	got := logger.RedactMap(in)

	if !reflect.DeepEqual(in, got) {
		t.Errorf("map should be unchanged when no deny key matches.\n in=%v\n got=%v", in, got)
	}
}

// 補助テスト: RedactHeaders の immutable 保証 — 元の Header と値スライスが
// 変更されない (deep copy)。
func TestRedactHeaders_Immutable(t *testing.T) {
	values := []string{"Bearer one", "Bearer two"}
	h := http.Header{"Authorization": values}

	got := logger.RedactHeaders(h)

	// 戻り値の値スライスを変更しても元は影響なし。
	got["Authorization"][0] = "tampered"
	if values[0] != "Bearer one" {
		t.Errorf("RedactHeaders: source slice mutated by returned slice: got %v", values)
	}

	// 元の Header の値を変更しても戻り値は影響なし。
	values[1] = "tampered-too"
	if got["Authorization"][1] != logger.RedactedValue {
		t.Errorf("RedactHeaders: returned slice changed when source mutated, got %v", got["Authorization"])
	}
}

// 補助テスト: RedactMap の immutable 保証 — ネスト map / 配列も非破壊。
func TestRedactMap_Immutable(t *testing.T) {
	nested := map[string]any{"client_id": "public"}
	arr := []any{map[string]any{"k": "v"}}
	in := map[string]any{
		"nested": nested,
		"arr":    arr,
	}
	got := logger.RedactMap(in)

	// 戻り値の nested を変更しても元は影響なし。
	got["nested"].(map[string]any)["client_id"] = "tampered"
	if nested["client_id"] != "public" {
		t.Errorf("RedactMap: source nested map mutated by returned map: %v", nested)
	}

	// 戻り値の配列要素を変更しても元は影響なし。
	got["arr"].([]any)[0].(map[string]any)["k"] = "tampered"
	if arr[0].(map[string]any)["k"] != "v" {
		t.Errorf("RedactMap: source array element mutated: %v", arr)
	}
}

// 補助テスト: nil 入力安全性。
func TestRedactHeaders_Nil(t *testing.T) {
	if got := logger.RedactHeaders(nil); got != nil {
		t.Errorf("RedactHeaders(nil) = %v, want nil", got)
	}
}

func TestRedactMap_Nil(t *testing.T) {
	if got := logger.RedactMap(nil); got != nil {
		t.Errorf("RedactMap(nil) = %v, want nil", got)
	}
}

// T-49: apperror.CodedError の details にシークレット (password) を入れた場合、
// logger.RedactMap 経由で [REDACTED] に置換される (apperror ↔ logger の連携確認)。
func TestRedactMap_AppErrorDetailsIntegration(t *testing.T) {
	e := apperror.New("INVALID_PARAMETER", "x").WithDetails(map[string]any{
		"password": "leaked",
		"field":    "email",
	})

	redacted := logger.RedactMap(e.Details())

	if redacted["password"] != logger.RedactedValue {
		t.Errorf("password in CodedError.Details should be redacted by logger, got %v", redacted["password"])
	}
	if redacted["field"] != "email" {
		t.Errorf("non-secret field should be preserved, got %v", redacted["field"])
	}
}

// upper は ASCII 範囲で大文字化 (テスト用簡易実装)。
func upper(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		b[i] = c
	}
	return string(b)
}
