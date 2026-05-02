//go:build integration

package health_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mktkhr/id-core/core/internal/health"
	"github.com/mktkhr/id-core/core/internal/logger"
	"github.com/mktkhr/id-core/core/internal/testutil/dbtest"
)

// T-94: 実機 DB に対する /health/ready の Ping 成功 → 200 + status=ok。
func TestReadyHandler_Integration_PingSuccess_T94(t *testing.T) {
	_, pool := dbtest.NewPool(t)
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	health.NewReadyHandler(pool, l)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (実機 DB Ping 成功想定)", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("body.status = %q, want %q", body["status"], "ok")
	}
}
