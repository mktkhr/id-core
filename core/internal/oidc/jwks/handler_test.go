package jwks_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mktkhr/id-core/core/internal/keystore"
	"github.com/mktkhr/id-core/core/internal/logger"
	"github.com/mktkhr/id-core/core/internal/oidc/jwks"
)

// テスト用 KeySet を起動時生成モードで構築。
func newTestKeySet(t *testing.T) keystore.KeySet {
	t.Helper()
	l := logger.New(logger.FormatJSON, &bytes.Buffer{})
	ks, _, err := keystore.Init(context.Background(),
		keystore.OIDCKeyConfig{DevGenerateKey: true}, l)
	if err != nil {
		t.Fatalf("keystore.Init: %v", err)
	}
	return ks
}

// 200 + 必須ヘッダ + body 構造の検証 (F-5 / F-6)。
func TestHandler_Success_200(t *testing.T) {
	ks := newTestKeySet(t)
	h, err := jwks.New(ks, 300)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/jwks", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", res.StatusCode)
	}
	if got := res.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if res.Header.Get("ETag") == "" {
		t.Error("ETag header missing")
	}
	if res.Header.Get("Cache-Control") == "" {
		t.Error("Cache-Control header missing")
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	var got struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("Unmarshal body: %v\nbody=%s", err, body)
	}
	if len(got.Keys) != 1 {
		t.Fatalf("keys length = %d, want 1 (M1.1 single key)", len(got.Keys))
	}
	k := got.Keys[0]
	for _, want := range []string{"kty", "use", "alg", "kid", "n", "e"} {
		if _, ok := k[want]; !ok {
			t.Errorf("keys[0].%s missing in body: %s", want, body)
		}
	}
}

// Cache-Control 切替: max-age=300 既定 / 0 → no-cache / 600。
func TestHandler_CacheControlSwitch(t *testing.T) {
	cases := []struct {
		name   string
		maxAge int
		want   string
	}{
		{name: "既定 300", maxAge: 300, want: "public, max-age=300, must-revalidate"},
		{name: "max-age=0", maxAge: 0, want: "no-cache, must-revalidate"},
		{name: "max-age=600", maxAge: 600, want: "public, max-age=600, must-revalidate"},
		{name: "max-age=86400 (上限)", maxAge: 86400, want: "public, max-age=86400, must-revalidate"},
		{name: "max-age=-1 → 0 同等", maxAge: -1, want: "no-cache, must-revalidate"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ks := newTestKeySet(t)
			h, err := jwks.New(ks, tc.maxAge)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			req := httptest.NewRequest(http.MethodGet, "/jwks", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if got := rec.Header().Get("Cache-Control"); got != tc.want {
				t.Errorf("Cache-Control = %q, want %q", got, tc.want)
			}
		})
	}
}

// If-None-Match 一致 → 304 + body 空。
func TestHandler_IfNoneMatch_Matches_304(t *testing.T) {
	ks := newTestKeySet(t)
	h, err := jwks.New(ks, 300)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// 1 回目: ETag 取得
	req1 := httptest.NewRequest(http.MethodGet, "/jwks", nil)
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req1)
	etag := rec1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag missing")
	}

	// 2 回目: If-None-Match で同じ ETag を渡す
	req2 := httptest.NewRequest(http.MethodGet, "/jwks", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)

	res := rec2.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotModified {
		t.Errorf("status = %d, want 304", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if len(body) != 0 {
		t.Errorf("304 body should be empty, got %d bytes: %s", len(body), body)
	}
	if got := res.Header.Get("ETag"); got != etag {
		t.Errorf("304 ETag = %q, want %q", got, etag)
	}
}

// If-None-Match 不一致 → 200 + body 返却。
func TestHandler_IfNoneMatch_NoMatch_200(t *testing.T) {
	ks := newTestKeySet(t)
	h, err := jwks.New(ks, 300)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/jwks", nil)
	req.Header.Set("If-None-Match", `"different-etag-value"`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("200 body should not be empty")
	}
}

// 100 回 GET で ETag が drift しない (起動時キャッシュ)。
func TestHandler_ETagStableAcrossRequests(t *testing.T) {
	ks := newTestKeySet(t)
	h, err := jwks.New(ks, 300)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	var first string
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodGet, "/jwks", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		got := rec.Header().Get("ETag")
		if i == 0 {
			first = got
		} else if got != first {
			t.Fatalf("ETag drift at iter %d: got %q, first %q", i, got, first)
		}
	}
}

// New 失敗系: nil KeySet → エラー。
func TestNew_NilKeySetRejected(t *testing.T) {
	_, err := jwks.New(nil, 300)
	if err == nil {
		t.Error("nil KeySet should be rejected")
	}
}

// failingKeySet は KeySet の Verifying / Active で任意のエラーを返すモック (New エラー経路テスト用)。
type failingKeySet struct {
	verifyingErr error
	activeErr    error
	keys         []*keystore.KeyPair
}

func (f *failingKeySet) Active(_ context.Context) (*keystore.KeyPair, error) {
	if f.activeErr != nil {
		return nil, f.activeErr
	}
	return nil, nil
}

func (f *failingKeySet) Verifying(_ context.Context) ([]*keystore.KeyPair, error) {
	if f.verifyingErr != nil {
		return nil, f.verifyingErr
	}
	return f.keys, nil
}

// New 失敗系: keystore.Verifying がエラー → New もエラー。
func TestNew_VerifyingError_Propagates(t *testing.T) {
	ks := &failingKeySet{verifyingErr: errSentinel("verifying boom")}
	_, err := jwks.New(ks, 300)
	if err == nil {
		t.Error("Verifying error should propagate")
	}
}

// New 失敗系: BuildSet が失敗する (keys に nil が混入) → New もエラー。
func TestNew_BuildSetError_Propagates(t *testing.T) {
	ks := &failingKeySet{keys: []*keystore.KeyPair{nil}}
	_, err := jwks.New(ks, 300)
	if err == nil {
		t.Error("BuildSet error should propagate")
	}
}

// errSentinel は固定文字列 error を作る簡易 helper (sentinel ではないがインライン error 型代替)。
type errSentinel string

func (e errSentinel) Error() string { return string(e) }
