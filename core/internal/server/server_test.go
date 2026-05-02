package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mktkhr/id-core/core/internal/config"
	"github.com/mktkhr/id-core/core/internal/logger"
	"github.com/mktkhr/id-core/core/internal/middleware"
	"github.com/mktkhr/id-core/core/internal/server"
)

func newTestLogger() *logger.Logger {
	return logger.New(logger.FormatJSON, &bytes.Buffer{})
}

// newTestPool は server.New に渡せる遅延接続済 *pgxpool.Pool を返す。
// pgxpool.New は parse のみで接続は遅延されるため、DB なしでも作成可能。
// /health 系の正常パスは pool を呼び出さないので Ping しない限り問題ない。
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	p, err := pgxpool.New(context.Background(), "postgres://test:test@127.0.0.1:1/test?sslmode=disable")
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(p.Close)
	return p
}

// TestNew_AddrAndHandler は server.New が cfg.Port を反映した *http.Server を返すことを検証する。
func TestNew_AddrAndHandler(t *testing.T) {
	cfg := &config.Config{Port: 9090}
	srv := server.New(cfg, newTestLogger(), newTestPool(t))

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
	srv := server.New(cfg, newTestLogger(), newTestPool(t))

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
			t.Fatalf("server.New(cfg, nil, pool) should panic, got no panic")
		}
	}()
	_ = server.New(&config.Config{Port: 1}, nil, newTestPool(t))
}

// nil pool を渡すと server.New が契約違反として panic する (M0.3 で追加した契約)。
func TestNew_NilPool_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("server.New(cfg, l, nil) should panic, got no panic")
		}
	}()
	_ = server.New(&config.Config{Port: 1}, newTestLogger(), nil)
}

// ===== 統合テスト (T-53..T-57): middleware チェーン全体組み立てた状態の検証 =====

// newIntegratedServer は test 用に buffer-backed logger を注入した server を返す。
// /panic と /authheader の追加 endpoint をローカルにマウントして統合シナリオを試す。
func newIntegratedServerWithBuf(t *testing.T) (*http.Server, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	l := logger.New(logger.FormatJSON, buf)
	cfg := &config.Config{Port: config.DefaultPort}
	return server.New(cfg, l, newTestPool(t)), buf
}

// T-53: middleware チェーン全体を組み立てて GET /health を叩く。
// 200 OK + JSON status:ok + Content-Type + ヘッダ X-Request-Id (M0.1 互換 + M0.2 追加要件)。
func TestIntegration_HealthRoute_OK(t *testing.T) {
	srv, _ := newIntegratedServerWithBuf(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Result().StatusCode)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want prefix application/json", ct)
	}
	if rec.Header().Get("X-Request-Id") == "" {
		t.Errorf("X-Request-Id header missing")
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Result().Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("body.status = %v, want ok", body["status"])
	}
}

// T-54: 統合チェーン経由で出力されたアクセスログ JSON が T-40 と同じスキーマであること。
func TestIntegration_AccessLogSchema(t *testing.T) {
	srv, buf := newIntegratedServerWithBuf(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	access := findFirstLogWithMsg(t, buf, "access")
	if access == nil {
		t.Fatalf("access log not emitted: %q", buf.String())
	}
	for _, key := range []string{"request_id", "method", "path", "status", "duration_ms"} {
		if _, ok := access[key]; !ok {
			t.Errorf("access log missing field %q: %v", key, access)
		}
	}
	if access["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", access["level"])
	}
	if access["method"] != http.MethodGet {
		t.Errorf("method = %v, want GET", access["method"])
	}
	if access["path"] != "/health" {
		t.Errorf("path = %v, want /health", access["path"])
	}
	if status, _ := access["status"].(float64); int(status) != http.StatusOK {
		t.Errorf("status = %v, want 200", access["status"])
	}
	if d, ok := access["duration_ms"].(float64); !ok || d < 0 {
		t.Errorf("duration_ms invalid: %T %v", access["duration_ms"], access["duration_ms"])
	}
}

// T-55: panic endpoint を組み込んで叩く統合シナリオ。
// 500 + F-7 基本形 JSON + body にスタック非含 + アクセスログは status=500/level=ERROR。
func TestIntegration_PanicRoute_500Schema(t *testing.T) {
	buf := &bytes.Buffer{}
	l := logger.New(logger.FormatJSON, buf)

	// server.New は /health 以外を mount しないため、middleware チェーンを直接組み立てて
	// /panic endpoint を試す (D1 順序を再現)。
	mux := http.NewServeMux()
	mux.HandleFunc("GET /panic", func(w http.ResponseWriter, r *http.Request) {
		panic("integration-panic")
	})
	var wrapped http.Handler = mux
	wrapped = middleware.Recover(l, wrapped)
	wrapped = middleware.AccessLog(l, wrapped)
	wrapped = middleware.RequestID(wrapped)

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Result().StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Result().StatusCode)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json; charset=utf-8 (F-7)", ct)
	}
	body := rec.Body.String()
	for _, forbidden := range []string{"goroutine ", "runtime/panic", "integration-panic", ".go:"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("body should not contain %q (F-10 stack 漏洩防止), got: %q", forbidden, body)
		}
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	// F-7 必須キー: code / message / request_id を全て検証 (details は optional)。
	if resp["code"] != "INTERNAL_ERROR" {
		t.Errorf("code = %v, want INTERNAL_ERROR", resp["code"])
	}
	if msg, _ := resp["message"].(string); msg == "" {
		t.Errorf("message missing in panic response (F-7 必須キー)")
	}
	if rid, _ := resp["request_id"].(string); rid == "" {
		t.Errorf("request_id missing in panic response")
	}

	access := findFirstLogWithMsg(t, buf, "access")
	if access == nil {
		t.Fatalf("access log missing in panic flow: %q", buf.String())
	}
	if status, _ := access["status"].(float64); int(status) != http.StatusInternalServerError {
		t.Errorf("access log status = %v, want 500 (D1 順序の検証)", access["status"])
	}
	if access["level"] != "ERROR" {
		t.Errorf("access log level = %v, want ERROR", access["level"])
	}
}

// T-56: クライアント Authorization: Bearer xxx 付きで叩く。
// アクセスログ (現状は path / method のみ記録、ヘッダは載せない) の挙動と、
// もしヘッダを記録する将来拡張を行う場合の redact 設計を担保する。
//
// 現実装は header をログ出力していないため secrets が漏れないことを直接確認する
// (= access ログの全フィールド値に "Bearer " 文字列が含まれない)。
func TestIntegration_AuthorizationHeader_NoLeakInLog(t *testing.T) {
	srv, buf := newIntegratedServerWithBuf(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Authorization", "Bearer super-secret-token")
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if strings.Contains(buf.String(), "Bearer ") {
		t.Errorf("ログに Authorization ヘッダ値が漏洩している (F-13): %q", buf.String())
	}
	if strings.Contains(buf.String(), "super-secret-token") {
		t.Errorf("ログにシークレット値が漏洩している (F-13): %q", buf.String())
	}
}

// T-57: クライアント X-Request-Id に改行入りで叩く。
// レスポンスヘッダには新規 UUID v7 が付き、ログには client_request_id (サニタイズ済み) が残る。
func TestIntegration_BadXRequestID_RegeneratedAndLogged(t *testing.T) {
	srv, buf := newIntegratedServerWithBuf(t)

	bad := "abc\nbad"
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Request-Id", bad)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	rid := rec.Header().Get("X-Request-Id")
	if rid == bad {
		t.Errorf("不正な X-Request-Id がそのまま採用された: %q", rid)
	}
	parsed, err := uuid.Parse(rid)
	if err != nil || parsed.Version() != 7 {
		t.Errorf("レスポンスヘッダの X-Request-Id が UUID v7 でない: %q (err=%v)", rid, err)
	}
	access := findFirstLogWithMsg(t, buf, "access")
	if access == nil {
		t.Fatalf("access log missing: %q", buf.String())
	}
	c, ok := access["client_request_id"].(string)
	if !ok || c == "" {
		t.Errorf("ログに client_request_id (サニタイズ済) が残っていない: %v", access["client_request_id"])
	}
	if strings.ContainsAny(c, "\n\r\t") {
		t.Errorf("client_request_id に生の制御文字が残存 (F-1 ログインジェクション対策違反): %q", c)
	}
	// 同値性検証: 元の不正値 ("abc\nbad") が "abc\u000abad" にサニタイズされること。
	const wantSanitized = `abc\u000abad`
	if c != wantSanitized {
		t.Errorf("client_request_id = %q, want %q (元値 %q の Unicode escape 変換結果)", c, wantSanitized, bad)
	}
}

// findFirstLogWithMsg は buf 内の JSON Lines から msg=msgWanted の最初のレコードを返す。
func findFirstLogWithMsg(t *testing.T, buf *bytes.Buffer, msgWanted string) map[string]any {
	t.Helper()
	for _, line := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}
		if m["msg"] == msgWanted {
			return m
		}
	}
	return nil
}
