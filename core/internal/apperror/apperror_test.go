package apperror_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mktkhr/id-core/core/internal/apperror"
)

// T-47: New を JSON シリアライズ。details が無いケース。
func TestToResponse_BasicForm(t *testing.T) {
	e := apperror.New("INVALID_PARAMETER", "不正なパラメータです")

	resp := apperror.ToResponse(e, "req-1")
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got["code"] != "INVALID_PARAMETER" {
		t.Errorf("code = %v, want INVALID_PARAMETER", got["code"])
	}
	if got["message"] != "不正なパラメータです" {
		t.Errorf("message = %v", got["message"])
	}
	if got["request_id"] != "req-1" {
		t.Errorf("request_id = %v, want req-1", got["request_id"])
	}
	if _, ok := got["details"]; ok {
		t.Errorf("details should be omitted when nil, got %v", got["details"])
	}
}

// T-48: WithDetails を付けて JSON シリアライズ。
func TestToResponse_WithDetails(t *testing.T) {
	e := apperror.New("INVALID_PARAMETER", "不正").WithDetails(map[string]any{
		"field":  "email",
		"reason": "format",
	})

	resp := apperror.ToResponse(e, "req-2")
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	details, ok := got["details"].(map[string]any)
	if !ok {
		t.Fatalf("details should be JSON object, got %T", got["details"])
	}
	if details["field"] != "email" {
		t.Errorf("details.field = %v, want email", details["field"])
	}
	if details["reason"] != "format" {
		t.Errorf("details.reason = %v, want format", details["reason"])
	}
}

// T-50: details の型制約を文書化する。
// WithDetails は map[string]any しか受け取らないため、string / number / bool / array
// を直接渡すと compile error になる。本テストは正の振る舞いを検証することで
// 型シグネチャによる制約を成立させていることを文書化する。
func TestWithDetails_TypeConstraint_AcceptsObjectOnly(t *testing.T) {
	// object (map[string]any) は受け取れる。
	e := apperror.New("X", "x").WithDetails(map[string]any{"k": "v"})
	if got := e.Details()["k"]; got != "v" {
		t.Errorf("Details()[k] = %v, want v", got)
	}

	// 配列を入れたい場合は object のキーにネストすることで仕様 F-7 (object / array) の
	// "array" 表現を満たす設計とする。
	e2 := apperror.New("X", "x").WithDetails(map[string]any{
		"errors": []any{"a", "b"},
	})
	arr, ok := e2.Details()["errors"].([]any)
	if !ok || len(arr) != 2 {
		t.Errorf("nested array not preserved: %v", e2.Details())
	}
}

// 補助テスト: immutable 契約。WithDetails / Details() で deep-copy が効いていること。
func TestCodedError_Immutable_DeepCopy(t *testing.T) {
	src := map[string]any{
		"top":    "v",
		"nested": map[string]any{"k": "n"},
		"arr":    []any{map[string]any{"x": 1}},
	}
	e := apperror.New("X", "x").WithDetails(src)

	// 1. 入力 src を変更しても CodedError 内部状態は不変。
	src["top"] = "tampered"
	src["nested"].(map[string]any)["k"] = "tampered"
	src["arr"].([]any)[0].(map[string]any)["x"] = 999

	got := e.Details()
	if got["top"] != "v" {
		t.Errorf("WithDetails: top mutated by external src: got %v", got["top"])
	}
	if got["nested"].(map[string]any)["k"] != "n" {
		t.Errorf("WithDetails: nested map mutated: got %v", got["nested"])
	}
	if got["arr"].([]any)[0].(map[string]any)["x"] != 1 {
		t.Errorf("WithDetails: array element mutated: got %v", got["arr"])
	}

	// 2. Details() 戻り値を変更しても CodedError 内部状態は不変。
	got["top"] = "tampered2"
	got["nested"].(map[string]any)["k"] = "tampered2"

	got2 := e.Details()
	if got2["top"] != "v" {
		t.Errorf("Details(): top mutated by returned map: got %v", got2["top"])
	}
	if got2["nested"].(map[string]any)["k"] != "n" {
		t.Errorf("Details(): nested map mutated: got %v", got2["nested"])
	}
}

// T-51: errors.Is / errors.As 互換性 (Unwrap)。
func TestCodedError_Unwrap(t *testing.T) {
	root := errors.New("ルート原因")
	wrapped := apperror.New("INTERNAL_ERROR", "内部エラー").Wrap(root)

	if !errors.Is(wrapped, root) {
		t.Errorf("errors.Is(wrapped, root) = false, want true")
	}

	var target *apperror.CodedError
	if !errors.As(wrapped, &target) {
		t.Fatalf("errors.As(wrapped, *CodedError) = false")
	}
	if target.Code() != "INTERNAL_ERROR" {
		t.Errorf("target.Code() = %v, want INTERNAL_ERROR", target.Code())
	}
}

// 補助テスト: ToResponse(nil, ...) が INTERNAL_ERROR + 既定メッセージを返す。
func TestToResponse_NilFallback(t *testing.T) {
	resp := apperror.ToResponse(nil, "req-nil")
	if resp.Code != apperror.CodeInternalError {
		t.Errorf("Code = %v, want %v", resp.Code, apperror.CodeInternalError)
	}
	if resp.Message != apperror.MessageInternalError {
		t.Errorf("Message = %v, want default", resp.Message)
	}
	if resp.RequestID != "req-nil" {
		t.Errorf("RequestID = %v", resp.RequestID)
	}
}

// T-52: request_id が空文字でも JSON は valid。
func TestToResponse_EmptyRequestID(t *testing.T) {
	e := apperror.New("INTERNAL_ERROR", "内部エラー")
	resp := apperror.ToResponse(e, "")

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	v, ok := got["request_id"]
	if !ok {
		t.Errorf("request_id field should be present (mandatory), got missing")
	}
	if v != "" {
		t.Errorf("request_id = %v, want empty string", v)
	}
}

// 補助テスト: WriteJSON が status / Content-Type / body を正しく書き込む。
func TestWriteJSON(t *testing.T) {
	e := apperror.New("NOT_FOUND", "見つかりません")
	rec := httptest.NewRecorder()

	if err := apperror.WriteJSON(rec, http.StatusNotFound, e, "req-w"); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json; charset=utf-8", ct)
	}

	var got map[string]any
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got["code"] != "NOT_FOUND" || got["request_id"] != "req-w" {
		t.Errorf("body unexpected: %v", got)
	}
}
