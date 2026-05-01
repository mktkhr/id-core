package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mktkhr/id-core/core/internal/config"
	"github.com/mktkhr/id-core/core/internal/server"
)

// TestNew_AddrAndHandler は server.New が cfg.Port を反映した *http.Server を返すことを検証する。
func TestNew_AddrAndHandler(t *testing.T) {
	cfg := &config.Config{Port: 9090}
	srv := server.New(cfg)

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

// TestNew_HealthRoute は server.New が返すサーバーで /health が解決できることを検証する。
//
// ListenAndServe は呼ばず、Handler に直接リクエストを投げる (シンプル & 高速)。
func TestNew_HealthRoute(t *testing.T) {
	cfg := &config.Config{Port: config.DefaultPort}
	srv := server.New(cfg)

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
}
