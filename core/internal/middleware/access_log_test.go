package middleware_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mktkhr/id-core/core/internal/logger"
	"github.com/mktkhr/id-core/core/internal/middleware"
)

// makeLoggerBuf は test 用に bytes.Buffer に書き込む JSON Lines ロガーを返す。
func makeLoggerBuf(t *testing.T) (*logger.Logger, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	return logger.New(logger.FormatJSON, &buf), &buf
}

// readLastJSONLine は buf の末尾 JSON Lines 1 行を map にデコードして返す。
func readLastJSONLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("no log lines emitted; buf=%q", buf.String())
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &m); err != nil {
		t.Fatalf("Unmarshal last line: %v (line=%q)", err, lines[len(lines)-1])
	}
	return m
}

// T-40: 200 OK の handler → 終了時に 1 行 INFO ログ、必須フィールド完備。
func TestAccessLog_OK_INFO(t *testing.T) {
	l, buf := makeLoggerBuf(t)

	h := middleware.RequestID(middleware.AccessLog(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/health?x=1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	m := readLastJSONLine(t, buf)
	if m["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", m["level"])
	}
	if m["msg"] != "access" {
		t.Errorf("msg = %v, want access", m["msg"])
	}
	if m["method"] != http.MethodGet {
		t.Errorf("method = %v, want GET", m["method"])
	}
	// path には redact 後のクエリも含めて記録 (T-45 で redact を検証する)。
	if path, ok := m["path"].(string); !ok || !strings.HasPrefix(path, "/health") {
		t.Errorf("path = %v, want starts-with /health", m["path"])
	}
	if status, _ := m["status"].(float64); int(status) != http.StatusOK {
		t.Errorf("status = %v, want 200", m["status"])
	}
	if _, ok := m["duration_ms"].(float64); !ok {
		t.Errorf("duration_ms missing or not float, got %T %v", m["duration_ms"], m["duration_ms"])
	}
	if _, ok := m["request_id"].(string); !ok || m["request_id"] == "" {
		t.Errorf("request_id missing")
	}
}

// T-41: 4xx を返す handler → level=WARN。
func TestAccessLog_4xx_WARN(t *testing.T) {
	l, buf := makeLoggerBuf(t)
	h := middleware.RequestID(middleware.AccessLog(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	m := readLastJSONLine(t, buf)
	if m["level"] != "WARN" {
		t.Errorf("level = %v, want WARN", m["level"])
	}
	if status, _ := m["status"].(float64); int(status) != http.StatusBadRequest {
		t.Errorf("status = %v, want 400", m["status"])
	}
}

// T-42: 5xx を返す handler → level=ERROR。
func TestAccessLog_5xx_ERROR(t *testing.T) {
	l, buf := makeLoggerBuf(t)
	h := middleware.RequestID(middleware.AccessLog(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	m := readLastJSONLine(t, buf)
	if m["level"] != "ERROR" {
		t.Errorf("level = %v, want ERROR", m["level"])
	}
}

// T-43 (a): 後段が status=500 を WriteHeader した場合に access_log が ERROR を観測。
// (recover→500 への変換シナリオは Step 3 の統合テスト T-55 でカバーする)
func TestAccessLog_HandlerWriteHeader500_ObserveERROR(t *testing.T) {
	l, buf := makeLoggerBuf(t)
	h := middleware.RequestID(middleware.AccessLog(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	m := readLastJSONLine(t, buf)
	if m["level"] != "ERROR" {
		t.Errorf("level = %v, want ERROR", m["level"])
	}
	if status, _ := m["status"].(float64); int(status) != http.StatusInternalServerError {
		t.Errorf("status = %v, want 500", m["status"])
	}
}

// T-43 (b): 後段が WriteHeader を呼ばずに Write のみで暗黙 200 を返すケースでも
// access_log は status=200 を観測する (status 暗黙化への耐性)。
func TestAccessLog_ImplicitOK_Status200(t *testing.T) {
	l, buf := makeLoggerBuf(t)
	h := middleware.RequestID(middleware.AccessLog(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("body"))
	})))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	m := readLastJSONLine(t, buf)
	if status, _ := m["status"].(float64); int(status) != http.StatusOK {
		t.Errorf("status = %v, want 200 (implicit)", m["status"])
	}
}

// T-44: duration_ms は float64 で記録 (ms 単位)。
func TestAccessLog_DurationMS_Float(t *testing.T) {
	l, buf := makeLoggerBuf(t)
	h := middleware.RequestID(middleware.AccessLog(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	m := readLastJSONLine(t, buf)
	d, ok := m["duration_ms"].(float64)
	if !ok {
		t.Fatalf("duration_ms not float64: %T %v", m["duration_ms"], m["duration_ms"])
	}
	if d < 0 {
		t.Errorf("duration_ms should be >= 0, got %v", d)
	}
}

// T-45: クエリ文字列に redact 対象キーが含まれる場合、ログ上の path は値が [REDACTED] 化される。
func TestAccessLog_Path_RedactQueryParams(t *testing.T) {
	l, buf := makeLoggerBuf(t)
	h := middleware.RequestID(middleware.AccessLog(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/cb?code=secret&state=abc", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	m := readLastJSONLine(t, buf)
	path, ok := m["path"].(string)
	if !ok {
		t.Fatalf("path missing or non-string: %v", m["path"])
	}
	if strings.Contains(path, "secret") {
		t.Errorf("path should not contain raw secret value, got: %q", path)
	}
	// URL エンコード後 ([REDACTED] は %5BREDACTED%5D になる) も識別できるよう、
	// 中核の REDACTED 文字列の存在で判定する (両形式を許容)。
	if !strings.Contains(path, "REDACTED") {
		t.Errorf("path should contain REDACTED marker for code, got: %q", path)
	}
	// state は redact 対象外なのでそのまま残る。
	if !strings.Contains(path, "state=abc") {
		t.Errorf("non-secret param should be preserved, got: %q", path)
	}
}

// T-46: 開始時にはログを出さない (D3: 終了時のみ 1 行)。
func TestAccessLog_NoLogBeforeFinish(t *testing.T) {
	l, buf := makeLoggerBuf(t)
	h := middleware.RequestID(middleware.AccessLog(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// handler 実行中の buf は空であるべき (access_log は defer で出力するため)。
		if buf.Len() != 0 {
			t.Errorf("access_log should not emit before handler completes, got: %q", buf.String())
		}
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// 終了後は msg=access のレコードが正確に 1 行出ているはず。他 middleware が
	// 別 msg のログを混在させても誤検知しないように msg=access で絞って数える。
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	access := 0
	for _, line := range lines {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("Unmarshal: %v line=%q", err, line)
		}
		if rec["msg"] == "access" {
			access++
		}
	}
	if access != 1 {
		t.Errorf("expected exactly 1 access log line, got %d (buf=%q)", access, buf.String())
	}
}
