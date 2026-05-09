package discovery_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mktkhr/id-core/core/internal/config"
	"github.com/mktkhr/id-core/core/internal/oidc/discovery"
)

// テスト用の最小 OIDCConfig を組み立てる (config.Load を経由しない)。
// オプションで DiscoveryMaxAge を上書き可能。
func newTestConfig(maxAge int) config.OIDCConfig {
	return config.OIDCConfig{
		Issuer:                "https://id.example.com",
		AuthorizationEndpoint: "https://id.example.com/authorize",
		TokenEndpoint:         "https://id.example.com/token",
		UserInfoEndpoint:      "https://id.example.com/userinfo",
		JWKSURI:               "https://id.example.com/jwks",
		DiscoveryMaxAge:       maxAge,
	}
}

// 200 + 必須ヘッダ + body 構造の検証。
func TestHandler_Success_200(t *testing.T) {
	h, err := discovery.New(newTestConfig(0))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", nil)
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
	if got := res.Header.Get("ETag"); got == "" {
		t.Errorf("ETag header missing")
	}
	if got := res.Header.Get("Cache-Control"); got == "" {
		t.Errorf("Cache-Control header missing")
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	var m discovery.Metadata
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("Unmarshal body: %v\nbody=%s", err, body)
	}
	if m.Issuer != "https://id.example.com" {
		t.Errorf("Issuer = %q", m.Issuer)
	}
}

// Cache-Control 切替: max-age=0 → no-cache, must-revalidate / >0 → public, max-age=N, must-revalidate。
func TestHandler_CacheControlSwitch(t *testing.T) {
	cases := []struct {
		name   string
		maxAge int
		want   string
	}{
		{name: "max-age=0 (既定)", maxAge: 0, want: "no-cache, must-revalidate"},
		{name: "max-age=600", maxAge: 600, want: "public, max-age=600, must-revalidate"},
		{name: "max-age=86400 (上限)", maxAge: 86400, want: "public, max-age=86400, must-revalidate"},
		{name: "max-age=-1 → 0 同等", maxAge: -1, want: "no-cache, must-revalidate"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			h, err := discovery.New(newTestConfig(tc.maxAge))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if got := rec.Header().Get("Cache-Control"); got != tc.want {
				t.Errorf("Cache-Control = %q, want %q", got, tc.want)
			}
		})
	}
}

// If-None-Match 一致 → 304 Not Modified、body 空。
func TestHandler_IfNoneMatch_Matches_304(t *testing.T) {
	h, err := discovery.New(newTestConfig(0))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// 1 回目: ETag を取得
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req1)
	etag := rec1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag header missing on first request")
	}

	// 2 回目: If-None-Match で同じ ETag を渡す
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
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
		t.Errorf("304 body should be empty, got %d bytes: %q", len(body), body)
	}
	// 304 でも ETag は応答に含めるのが推奨
	if got := res.Header.Get("ETag"); got != etag {
		t.Errorf("304 ETag = %q, want %q", got, etag)
	}
}

// If-None-Match 不一致 → 200 + body 返却。
func TestHandler_IfNoneMatch_NoMatch_200(t *testing.T) {
	h, err := discovery.New(newTestConfig(0))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
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

// ContractTest 5 ケース: handler 経由で各 issuer 形式の endpoint URL が JSON 中に
// 期待通りに現れる。
func TestHandler_ContractTest_5Cases(t *testing.T) {
	cases := []struct {
		name              string
		cfg               config.OIDCConfig
		wantAuthorization string
	}{
		{
			name: "1. 標準",
			cfg: config.OIDCConfig{
				Issuer:                "https://id.example.com",
				AuthorizationEndpoint: "https://id.example.com/authorize",
				TokenEndpoint:         "https://id.example.com/token",
				UserInfoEndpoint:      "https://id.example.com/userinfo",
				JWKSURI:               "https://id.example.com/jwks",
			},
			wantAuthorization: "https://id.example.com/authorize",
		},
		{
			name: "2. subpath",
			cfg: config.OIDCConfig{
				Issuer:                "https://example.com/id-core",
				AuthorizationEndpoint: "https://example.com/id-core/authorize",
				TokenEndpoint:         "https://example.com/id-core/token",
				UserInfoEndpoint:      "https://example.com/id-core/userinfo",
				JWKSURI:               "https://example.com/id-core/jwks",
			},
			wantAuthorization: "https://example.com/id-core/authorize",
		},
		{
			name: "3. 末尾スラッシュ (config 側で strip 済前提)",
			cfg: config.OIDCConfig{
				Issuer:                "https://example.com/id-core",
				AuthorizationEndpoint: "https://example.com/id-core/authorize",
				TokenEndpoint:         "https://example.com/id-core/token",
				UserInfoEndpoint:      "https://example.com/id-core/userinfo",
				JWKSURI:               "https://example.com/id-core/jwks",
			},
			wantAuthorization: "https://example.com/id-core/authorize",
		},
		{
			name: "4. dev 非 https",
			cfg: config.OIDCConfig{
				Issuer:                "http://localhost:8080",
				AuthorizationEndpoint: "http://localhost:8080/authorize",
				TokenEndpoint:         "http://localhost:8080/token",
				UserInfoEndpoint:      "http://localhost:8080/userinfo",
				JWKSURI:               "http://localhost:8080/jwks",
			},
			wantAuthorization: "http://localhost:8080/authorize",
		},
		{
			name: "5. 非標準ポート",
			cfg: config.OIDCConfig{
				Issuer:                "https://id.example.com:9443",
				AuthorizationEndpoint: "https://id.example.com:9443/authorize",
				TokenEndpoint:         "https://id.example.com:9443/token",
				UserInfoEndpoint:      "https://id.example.com:9443/userinfo",
				JWKSURI:               "https://id.example.com:9443/jwks",
			},
			wantAuthorization: "https://id.example.com:9443/authorize",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			h, err := discovery.New(tc.cfg)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d", rec.Code)
			}
			body := rec.Body.String()
			if !strings.Contains(body, tc.wantAuthorization) {
				t.Errorf("response body lacks expected authorization endpoint %q\nbody=%s", tc.wantAuthorization, body)
			}
		})
	}
}

// 同一インスタンスで 100 回 GET → ETag 安定 (起動時キャッシュの確認)。
func TestHandler_ETagStableAcrossRequests(t *testing.T) {
	h, err := discovery.New(newTestConfig(300))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	var first string
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		got := rec.Header().Get("ETag")
		if got == "" {
			t.Fatalf("ETag missing at iter %d", i)
		}
		if i == 0 {
			first = got
		} else if got != first {
			t.Fatalf("ETag drifted at iter %d: got %q, first %q", i, got, first)
		}
	}
}
