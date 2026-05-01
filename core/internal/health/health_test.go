package health_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mktkhr/id-core/core/internal/health"
)

// newServeMux はテスト用に health ハンドラだけを登録した ServeMux を返す。
//
// パターン文字列 ("GET /health" 等) は server パッケージの登録方法に揃え、
// 405 Method Not Allowed や Allow ヘッダの挙動を ServeMux ごと再現できるようにする。
func newServeMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health.Handler)
	return mux
}

// T-1: GET /health → 200 OK
// T-2: Content-Type: application/json から始まる
// T-3: ボディ JSON の status が "ok"
func TestHandler_GET_Success(t *testing.T) {
	mux := newServeMux()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	// T-1: 200 OK
	if res.StatusCode != http.StatusOK {
		t.Errorf("status code = %d, want %d", res.StatusCode, http.StatusOK)
	}

	// T-2: Content-Type: application/json から始まる
	ct := res.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want prefix %q", ct, "application/json")
	}

	// T-3: ボディ JSON の status が "ok"
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("レスポンスボディの JSON decode に失敗: %v", err)
	}
	status, ok := body["status"].(string)
	if !ok {
		t.Fatalf("レスポンスに status (string) フィールドが含まれていない: body=%+v", body)
	}
	if status != "ok" {
		t.Errorf("status = %q, want %q", status, "ok")
	}
}

// T-4: POST /health → 405 Method Not Allowed + Allow ヘッダに GET を含む
//
// Go 1.22+ ServeMux の標準挙動: メソッド指定パターン (GET /health) で他メソッドが来た場合、
// 405 とともに Allow ヘッダで許可メソッドを返す。
func TestHandler_POST_MethodNotAllowed(t *testing.T) {
	mux := newServeMux()

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status code = %d, want %d", res.StatusCode, http.StatusMethodNotAllowed)
	}

	allow := res.Header.Get("Allow")
	if !strings.Contains(allow, http.MethodGet) {
		t.Errorf("Allow ヘッダに GET を含むべき: got %q", allow)
	}
}
