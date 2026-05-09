package notimpl_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mktkhr/id-core/core/internal/oidc/notimpl"
)

// 503 + 必須ヘッダ + body 構造の検証 (F-23)。
func TestHandler_503Response(t *testing.T) {
	cases := []struct {
		name      string
		milestone string
	}{
		{name: "M1.2 (authorize)", milestone: "M1.2"},
		{name: "M1.3 (token)", milestone: "M1.3"},
		{name: "M1.4 (userinfo)", milestone: "M1.4"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			h := notimpl.Handler(tc.milestone)
			req := httptest.NewRequest(http.MethodGet, "/authorize", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			res := rec.Result()
			defer res.Body.Close()

			if res.StatusCode != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want 503", res.StatusCode)
			}
			if got := res.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			if got := res.Header.Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", got)
			}

			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatalf("ReadAll: %v", err)
			}
			var got map[string]string
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("Unmarshal: %v\nbody=%s", err, body)
			}
			if got["error"] != "endpoint_not_implemented" {
				t.Errorf("error = %q, want %q (snake_case)", got["error"], "endpoint_not_implemented")
			}
			if got["available_at"] != tc.milestone {
				t.Errorf("available_at = %q, want %q", got["available_at"], tc.milestone)
			}
		})
	}
}

// Retry-After ヘッダが付かない (RFC 7231 §7.1.3 違反回避、doc-review HIGH 2)。
func TestHandler_NoRetryAfterHeader(t *testing.T) {
	h := notimpl.Handler("M1.2")
	req := httptest.NewRequest(http.MethodGet, "/authorize", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Retry-After"); got != "" {
		t.Errorf("Retry-After header should not be set (RFC 7231 §7.1.3 違反回避), got %q", got)
	}
}

// method 不問 (GET / POST / PUT / DELETE / PATCH すべて同じ 503)。
func TestHandler_AllMethodsReturn503(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	}
	for _, m := range methods {
		t.Run(m, func(t *testing.T) {
			h := notimpl.Handler("M1.2")
			req := httptest.NewRequest(m, "/authorize", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("method %s: status = %d, want 503", m, rec.Code)
			}
		})
	}
}

// body フォーマット詳細: error フィールドは snake_case の固定値。
func TestHandler_BodyErrorFormat(t *testing.T) {
	h := notimpl.Handler("M1.2")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body, _ := io.ReadAll(rec.Result().Body)
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	// 余計なフィールドが入っていない
	if len(got) != 2 {
		t.Errorf("body keys = %d, want 2 (error / available_at), got=%v", len(got), got)
	}
	if _, ok := got["error"].(string); !ok {
		t.Errorf("error field missing or wrong type: %v", got["error"])
	}
	if _, ok := got["available_at"].(string); !ok {
		t.Errorf("available_at field missing or wrong type: %v", got["available_at"])
	}
}
