package middleware_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mktkhr/id-core/core/internal/middleware"
)

// T-33: handler が panic("test") した場合、HTTP 500 + Content-Type + F-7 基本形 JSON を返す。
func TestRecover_PanicString_500WithJSON(t *testing.T) {
	l, _ := makeLoggerBuf(t)
	h := middleware.RequestID(middleware.Recover(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("oops")
	})))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json; charset=utf-8", ct)
	}

	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if body["code"] != "INTERNAL_ERROR" {
		t.Errorf("code = %v, want INTERNAL_ERROR", body["code"])
	}
	if msg, _ := body["message"].(string); msg == "" {
		t.Errorf("message should be non-empty")
	}
	if rid, _ := body["request_id"].(string); rid == "" {
		t.Errorf("request_id should be non-empty in panic response")
	}
}

// T-34: panic レスポンスの body にスタックトレース・内部パスが含まれない (F-10)。
func TestRecover_PanicResponse_NoStackTrace(t *testing.T) {
	l, _ := makeLoggerBuf(t)
	h := middleware.RequestID(middleware.Recover(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("leaky-panic-message")
	})))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, forbidden := range []string{
		"goroutine ",       // runtime stack header
		"runtime/panic",    // package name
		".go:",             // file:line markers
		"leaky-panic",      // panic value should not leak
		"github.com/",      // module path
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("response body should not contain %q, got: %q", forbidden, body)
		}
	}
}

// T-35: panic レスポンスの request_id が context の値と一致する (X-Request-Id ヘッダ経由で確認)。
func TestRecover_PanicResponse_RequestIDMatchesHeader(t *testing.T) {
	l, _ := makeLoggerBuf(t)
	h := middleware.RequestID(middleware.Recover(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("x")
	})))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	hdr := rec.Header().Get("X-Request-Id")
	if hdr == "" {
		t.Fatalf("X-Request-Id header missing")
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["request_id"] != hdr {
		t.Errorf("body.request_id = %v, want %v (== X-Request-Id header)", body["request_id"], hdr)
	}
}

// T-36: panic 時の内部ログに level=ERROR / msg / request_id / error / stack_trace が含まれる。
func TestRecover_InternalLog_HasStackTrace(t *testing.T) {
	l, buf := makeLoggerBuf(t)
	h := middleware.RequestID(middleware.Recover(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("internal-only-stack")
	})))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	m := readLastJSONLine(t, buf)
	if m["level"] != "ERROR" {
		t.Errorf("level = %v, want ERROR", m["level"])
	}
	if rid, _ := m["request_id"].(string); rid == "" {
		t.Errorf("request_id missing in panic log")
	}
	if errStr, _ := m["error"].(string); errStr == "" {
		t.Errorf("error field missing in panic log")
	}
	if stack, _ := m["stack_trace"].(string); !strings.Contains(stack, "goroutine") {
		t.Errorf("stack_trace should contain runtime stack info, got: %q", stack)
	}
}

// T-37: handler が panic しない場合、recover は何もしない (パススルー)。
func TestRecover_NoPanic_PassThrough(t *testing.T) {
	l, buf := makeLoggerBuf(t)
	h := middleware.RequestID(middleware.Recover(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Result().StatusCode)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("body = %q, want ok", rec.Body.String())
	}
	// recover の defer がパススルーされる場合、内部 ERROR ログは出ないはず。
	if buf.Len() != 0 {
		// 通常 access_log を組み合わせない単体テストなので何も出ない想定。
		// 出ているとしても ERROR レベルではないことを確認する。
		// ここでは「recover からの ERROR ログがない」ことだけを最低限確認。
		if strings.Contains(buf.String(), `"level":"ERROR"`) {
			t.Errorf("no panic occurred, but ERROR level log was emitted: %q", buf.String())
		}
	}
}

// T-38: panic value が error 型の場合、error chain として "error" フィールドに記録。
func TestRecover_PanicError_ErrorFieldHasMessage(t *testing.T) {
	l, buf := makeLoggerBuf(t)
	sentinel := errors.New("sentinel-error-msg")
	h := middleware.RequestID(middleware.Recover(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(sentinel)
	})))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	m := readLastJSONLine(t, buf)
	if errStr, _ := m["error"].(string); !strings.Contains(errStr, "sentinel-error-msg") {
		t.Errorf("error field should contain sentinel error message, got: %v", m["error"])
	}
}

// T-39: panic value が任意の型 (int / struct 等) でも fmt.Sprintf("%v", v) で文字列化記録。
func TestRecover_PanicAnyValue_StringifiedInLog(t *testing.T) {
	type customPanicPayload struct {
		Op   string
		Code int
	}
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{"int", 42, "42"},
		{"struct", customPanicPayload{Op: "x", Code: 999}, "999"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l, buf := makeLoggerBuf(t)
			value := tc.value
			h := middleware.RequestID(middleware.Recover(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic(value)
			})))

			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			m := readLastJSONLine(t, buf)
			if errStr, _ := m["error"].(string); !strings.Contains(errStr, tc.want) {
				t.Errorf("error field should contain %q, got: %v", tc.want, m["error"])
			}
		})
	}
}

// 補助テスト: ハンドラが既に WriteHeader 済みの状態で panic した場合、recover は
// 500 化を試みず (二重 WriteHeader 防止)、内部 ERROR ログに status_already_written を記録する。
func TestRecover_PanicAfterWriteHeader_NoSecondWrite(t *testing.T) {
	l, buf := makeLoggerBuf(t)
	h := middleware.RequestID(middleware.Recover(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted) // 202
		_, _ = w.Write([]byte("partial"))
		panic("after-write-header")
	})))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// レスポンスの最終 status は handler が書いた 202 のまま (recover が上書きしない)。
	if rec.Result().StatusCode != http.StatusAccepted {
		t.Errorf("status = %d, want 202 (handler が書いた値が残る)", rec.Result().StatusCode)
	}
	m := readLastJSONLine(t, buf)
	if status, _ := m["status_already_written"].(float64); int(status) != http.StatusAccepted {
		t.Errorf("log should record status_already_written=202, got: %v", m["status_already_written"])
	}
}

// 補助テスト: D1 順序の回帰検知 — recover の外側に access_log が居ない (=逆順 wrap した) 場合、
// access_log の status 観測が壊れることを示すことで、本来の D1 順序の必然性を文書化する。
//
// この test は「D1 を逆にすると status=0 が観測される (= access_log が panic unwind 中に走る)」
// ことを直接確認する。
func TestRecover_DeprecatedReverseOrder_ObservesStatusZero(t *testing.T) {
	l, buf := makeLoggerBuf(t)
	// 意図的に D1 を逆順にした wrap (access_log が recover の内側): handler.panic は
	// recover が捕捉する前に access_log の defer に到達し、access_log の statusRecorder は
	// 何も WriteHeader を観測していないので status=0 で出力する。
	wrong := middleware.RequestID(
		middleware.Recover(l,
			middleware.AccessLog(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic("oops")
			})),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	wrong.ServeHTTP(rec, req)

	// この逆順では access_log の status はデフォルト (200) のままになる
	// (statusRecorder は WriteHeader を観測しないため)。本来は 500 を観測すべき。
	// ここは「逆順だと 500 を観測できない」ことを直接示すことで D1 順序を保証する。
	access := findFirstAccessLog(t, buf)
	if access == nil {
		t.Fatalf("access log not found in buf=%q", buf.String())
	}
	statusFloat, ok := access["status"].(float64)
	if !ok {
		t.Fatalf("access status missing or non-numeric: %v", access["status"])
	}
	status := int(statusFloat)
	// 逆順では access_log は status=200 (statusRecorder のデフォルト初期値) を観測する。
	// recover が上位で 500 を書いていても、access_log の statusRecorder には伝わらない。
	if status != http.StatusOK {
		t.Errorf("逆順 wrap での access status = %d, want 200 (デフォルト初期値が観測されるはず)", status)
	}
	if status == http.StatusInternalServerError {
		t.Errorf("逆順 wrap で 500 を観測してしまった (= D1 を入れ替えても回帰しないことになり順序保証が無意味化)")
	}
}

func findFirstAccessLog(t *testing.T, buf interface {
	String() string
}) map[string]any {
	t.Helper()
	for _, line := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}
		if m["msg"] == "access" {
			return m
		}
	}
	return nil
}
