package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mktkhr/id-core/core/internal/config"
	"github.com/mktkhr/id-core/core/internal/logger"
	"github.com/mktkhr/id-core/core/internal/server"
)

func newTestLogger() *logger.Logger {
	return logger.New(logger.FormatJSON, &bytes.Buffer{})
}

// TestNew_AddrAndHandler は server.New が cfg.Port を反映した *http.Server を返すことを検証する。
func TestNew_AddrAndHandler(t *testing.T) {
	cfg := &config.Config{Port: 9090}
	srv := server.New(cfg, newTestLogger())

	if srv == nil {
		t.Fatal("server.New が nil を返した")
	}
	if srv.Addr != ":9090" {
		t.Errorf("srv.Addr = %q, want %q", srv.Addr, ":9090")
	}
	if srv.Handler == nil {
		t.Fatal("srv.Handler が nil")
	}
}

// TestNew_HealthRoute は server.New が返すサーバーで /health が解決できることを検証する
// (M0.1 外形互換: HTTP 200 + Content-Type prefix application/json + status=ok)。
func TestNew_HealthRoute(t *testing.T) {
	cfg := &config.Config{Port: config.DefaultPort}
	srv := server.New(cfg, newTestLogger())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("/health の status = %d, want %d", res.StatusCode, http.StatusOK)
	}
	ct := res.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("/health の Content-Type = %q, want prefix %q", ct, "application/json")
	}
	// M0.2 で middleware チェーンが組み込まれたため、X-Request-Id が必ず付くこと。
	if rid := rec.Header().Get("X-Request-Id"); rid == "" {
		t.Errorf("/health のレスポンスに X-Request-Id が含まれない")
	}
	// M0.1 外形互換: body の {"status":"ok"} を厳密確認 (回帰検知)。
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("body decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("body.status = %v, want ok", body["status"])
	}
}

// nil logger を渡すと server.New が契約違反として panic することを直接検証する。
func TestNew_NilLogger_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("server.New(cfg, nil) should panic, got no panic")
		}
	}()
	_ = server.New(&config.Config{Port: 1}, nil)
}
