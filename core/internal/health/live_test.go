package health_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mktkhr/id-core/core/internal/health"
	"github.com/mktkhr/id-core/core/internal/logger"
)

// T-93: GET /health/live は DB 状態に依存せず 200 + {"status":"ok"} を返す。
func TestLiveHandler_Returns200(t *testing.T) {
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()
	health.NewLiveHandler(l)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("body status = %q, want %q", body["status"], "ok")
	}
}

// nil logger を渡すと panic する (契約違反)。
func TestLiveHandler_NilLoggerPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("NewLiveHandler(nil) で panic を期待したが発生しなかった")
		}
	}()
	_ = health.NewLiveHandler(nil)
}
