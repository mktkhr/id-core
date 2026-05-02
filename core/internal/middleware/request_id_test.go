package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/mktkhr/id-core/core/internal/logger"
	"github.com/mktkhr/id-core/core/internal/middleware"
)

// captureHandler は h(w, r) の実行時に context から request_id / client_request_id を
// テスト側で観測するためのハンドラ。
func captureHandler(out *capturedRequest) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out.requestID = logger.RequestIDFrom(r.Context())
		out.clientRequestID = middleware.ClientRequestIDFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	}
}

type capturedRequest struct {
	requestID       string
	clientRequestID string
}

// T-22: X-Request-Id ヘッダなし → 新規 UUID v7 生成、context 注入、レスポンスヘッダ設定。
func TestRequestID_NoHeader_Generates(t *testing.T) {
	var captured capturedRequest
	h := middleware.RequestID(captureHandler(&captured))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if captured.requestID == "" {
		t.Fatalf("request_id should be injected into context, got empty")
	}
	if got := rec.Header().Get("X-Request-Id"); got != captured.requestID {
		t.Errorf("response header X-Request-Id = %q, want %q", got, captured.requestID)
	}
	if captured.clientRequestID != "" {
		t.Errorf("client_request_id should not be set when no client header, got %q", captured.clientRequestID)
	}
	// UUID v7 形式の検証は T-31 でまとめて行う。
}

// T-23: 妥当な X-Request-Id (印字可能 ASCII のみ) はそのまま採用。
func TestRequestID_ValidClientHeader_Adopted(t *testing.T) {
	clientID := "abc123-XYZ_._-_~"
	var captured capturedRequest
	h := middleware.RequestID(captureHandler(&captured))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", clientID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if captured.requestID != clientID {
		t.Errorf("request_id = %q, want client value %q", captured.requestID, clientID)
	}
	if got := rec.Header().Get("X-Request-Id"); got != clientID {
		t.Errorf("response header = %q, want %q", got, clientID)
	}
	if captured.clientRequestID != "" {
		t.Errorf("client_request_id should not be set when value is adopted, got %q", captured.clientRequestID)
	}
}

// T-24: 128 オクテットちょうど → 採用される (境界値)。
func TestRequestID_128OctetsExactly_Adopted(t *testing.T) {
	clientID := strings.Repeat("a", 128)
	var captured capturedRequest
	h := middleware.RequestID(captureHandler(&captured))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", clientID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if captured.requestID != clientID {
		t.Errorf("128-octet value should be adopted, got %q", captured.requestID)
	}
	if captured.clientRequestID != "" {
		t.Errorf("no client_request_id expected when adopted")
	}
}

// T-25: 129 オクテット → 破棄、新規生成、サニタイズ済み client_request_id を context に残す。
func TestRequestID_129Octets_RejectedAndSanitized(t *testing.T) {
	clientID := strings.Repeat("a", 129)
	var captured capturedRequest
	h := middleware.RequestID(captureHandler(&captured))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", clientID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if captured.requestID == clientID {
		t.Errorf("129-octet value must NOT be adopted")
	}
	if captured.clientRequestID == "" {
		t.Errorf("client_request_id should be retained for logging when rejected")
	}
	if len(captured.clientRequestID) > 128 {
		t.Errorf("client_request_id should be truncated to <=128 octets, got len=%d", len(captured.clientRequestID))
	}
}

// T-26: 改行を含む値 → 破棄、サニタイズで Unicode escape 化して client_request_id に残す。
func TestRequestID_ControlCharNewline_RejectedAndEscaped(t *testing.T) {
	clientID := "abc\n\r\tdef"
	var captured capturedRequest
	h := middleware.RequestID(captureHandler(&captured))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", clientID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if captured.requestID == clientID {
		t.Errorf("value with newline must NOT be adopted")
	}
	got := captured.clientRequestID
	if got == "" {
		t.Fatalf("client_request_id should be set")
	}
	// 改行・CR・タブが Unicode escape (\u000a, \u000d, \u0009) になっていること。
	for _, want := range []string{`\u000a`, `\u000d`, `\u0009`} {
		if !strings.Contains(got, want) {
			t.Errorf("client_request_id should contain %q escape, got: %q", want, got)
		}
	}
	// 改行が生のまま残っていないこと (F-1 ログインジェクション対策)。
	if strings.ContainsAny(got, "\n\r\t") {
		t.Errorf("client_request_id should not contain raw control chars, got: %q", got)
	}
}

// T-27: 空白 (0x20) を含む値 → 破棄、新規生成。
func TestRequestID_ContainsSpace_Rejected(t *testing.T) {
	assertRejected(t, "abc def")
}

// T-28: タブ (0x09) を含む値 → 破棄。
func TestRequestID_ContainsTab_Rejected(t *testing.T) {
	assertRejected(t, "abc\tdef")
}

// T-29: DEL (0x7F) を含む値 → 破棄 (0x21-0x7E 範囲外)。
func TestRequestID_ContainsDEL_Rejected(t *testing.T) {
	assertRejected(t, "abc\x7Fdef")
}

func assertRejected(t *testing.T, clientID string) {
	t.Helper()
	var captured capturedRequest
	h := middleware.RequestID(captureHandler(&captured))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", clientID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if captured.requestID == clientID {
		t.Errorf("value %q must NOT be adopted", clientID)
	}
	if captured.clientRequestID == "" {
		t.Errorf("client_request_id should be retained for logging when rejected")
	}
}

// T-30: レスポンスヘッダ X-Request-Id が 200 / 4xx / 5xx いずれの status でも常時付与される。
func TestRequestID_HeaderAlwaysSet_AllStatuses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
	}{
		{"200", http.StatusOK},
		{"404", http.StatusNotFound},
		{"500", http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Result().StatusCode != tc.status {
				t.Errorf("status = %d, want %d", rec.Result().StatusCode, tc.status)
			}
			if got := rec.Header().Get("X-Request-Id"); got == "" {
				t.Errorf("X-Request-Id header missing for status %d", tc.status)
			}
		})
	}
}

// T-31: 生成された ID が UUID v7 (バージョンビット 7)。
func TestRequestID_GeneratedIsUUIDv7(t *testing.T) {
	var captured capturedRequest
	h := middleware.RequestID(captureHandler(&captured))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	parsed, err := uuid.Parse(captured.requestID)
	if err != nil {
		t.Fatalf("generated ID is not a valid UUID: %q (%v)", captured.requestID, err)
	}
	if got := parsed.Version(); got != 7 {
		t.Errorf("UUID version = %d, want 7", got)
	}
}

// 補助テスト: F-6 境界の正側 — 0x21 ('!') と 0x7E ('~') を含む値は採用される。
func TestRequestID_BoundaryPrintableASCII_Adopted(t *testing.T) {
	clientID := "!~" + strings.Repeat("a", 10)
	var captured capturedRequest
	h := middleware.RequestID(captureHandler(&captured))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", clientID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if captured.requestID != clientID {
		t.Errorf("0x21..0x7E のみで構成された値は採用されるべき: got %q", captured.requestID)
	}
}

// 補助テスト: 非 ASCII (日本語) は破棄される。
func TestRequestID_NonASCII_Rejected(t *testing.T) {
	assertRejected(t, "リクエストID")
}

// 補助テスト: X-Request-Id レスポンスヘッダは next.ServeHTTP 呼び出し前に
// 設定されている。「先行設定」要件の退行を検知する。
//
// httptest.ResponseRecorder は http.Server と異なり WriteHeader 後の Header().Set を
// ブロックしないため、WriteHeader 直前にヘッダをスナップショットする厳格 mock で検証する。
func TestRequestID_HeaderSetBeforeNext(t *testing.T) {
	var observedByHandler string
	h := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observedByHandler = w.Header().Get("X-Request-Id")
		w.WriteHeader(http.StatusOK)
	}))

	rec := newSnapshotRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rec, req)

	if observedByHandler == "" {
		t.Fatalf("X-Request-Id should be set BEFORE next.ServeHTTP, handler observed empty")
	}
	// WriteHeader 時点でスナップショットされたヘッダに X-Request-Id が含まれていること。
	if got := rec.snapshot.Get("X-Request-Id"); got == "" {
		t.Errorf("X-Request-Id should be set BEFORE WriteHeader (snapshot empty)")
	} else if got != observedByHandler {
		t.Errorf("snapshot X-Request-Id = %q, want %q", got, observedByHandler)
	}
}

// snapshotRecorder は WriteHeader 時点でヘッダをスナップショットする厳格 ResponseWriter。
// http.Server の挙動 (WriteHeader 後の Header 変更は無視) を簡易再現する。
type snapshotRecorder struct {
	h        http.Header
	snapshot http.Header
	written  bool
	code     int
}

func newSnapshotRecorder() *snapshotRecorder { return &snapshotRecorder{h: http.Header{}} }

func (r *snapshotRecorder) Header() http.Header { return r.h }

func (r *snapshotRecorder) WriteHeader(code int) {
	if r.written {
		return
	}
	r.written = true
	r.code = code
	r.snapshot = r.h.Clone()
}

func (r *snapshotRecorder) Write(p []byte) (int, error) {
	if !r.written {
		r.WriteHeader(http.StatusOK)
	}
	return len(p), nil
}

// T-32: サニタイズ後の client_request_id が 128 オクテット以下に切り詰められる (DoS 防止)。
func TestRequestID_SanitizedClientID_TruncatedTo128(t *testing.T) {
	// 制御文字を含み、escape 化すると元の長さの 6 倍 (\uXXXX) に膨らむケース。
	clientID := strings.Repeat("\n", 100) // 100 chars * 6 = 600 octets after escape
	var captured capturedRequest
	h := middleware.RequestID(captureHandler(&captured))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-Id", clientID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	got := captured.clientRequestID
	if got == "" {
		t.Fatalf("client_request_id should be retained")
	}
	if len(got) > 128 {
		t.Errorf("client_request_id should be truncated to <=128 octets, got len=%d", len(got))
	}
}
