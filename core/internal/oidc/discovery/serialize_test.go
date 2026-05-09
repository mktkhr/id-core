package discovery_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mktkhr/id-core/core/internal/config"
	"github.com/mktkhr/id-core/core/internal/oidc/discovery"
)

// 100 回 marshal して全て同一バイト列 (F-21 決定的シリアライズ)。
func TestMarshal_Deterministic_100x(t *testing.T) {
	cfg := config.OIDCConfig{
		Issuer:                "https://id.example.com",
		AuthorizationEndpoint: "https://id.example.com/authorize",
		TokenEndpoint:         "https://id.example.com/token",
		UserInfoEndpoint:      "https://id.example.com/userinfo",
		JWKSURI:               "https://id.example.com/jwks",
	}
	m := discovery.Build(cfg)

	first, err := discovery.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal first: %v", err)
	}

	for i := 0; i < 99; i++ {
		got, err := discovery.Marshal(m)
		if err != nil {
			t.Fatalf("Marshal #%d: %v", i, err)
		}
		if !bytes.Equal(got, first) {
			t.Fatalf("Marshal は決定的でなければならない (call %d で差分発生)", i)
		}
	}
}

// JSON のキー順序が struct field 宣言順 (= OIDC Discovery 1.0 慣習順) であること。
//
// encoding/json の挙動仕様: 構造体は宣言順でキーを出力する。これに依存してテストする。
func TestMarshal_FieldOrder(t *testing.T) {
	cfg := config.OIDCConfig{
		Issuer:                "https://id.example.com",
		AuthorizationEndpoint: "https://id.example.com/authorize",
		TokenEndpoint:         "https://id.example.com/token",
		UserInfoEndpoint:      "https://id.example.com/userinfo",
		JWKSURI:               "https://id.example.com/jwks",
	}
	body, err := discovery.Marshal(discovery.Build(cfg))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	expectedOrder := []string{
		`"issuer"`,
		`"authorization_endpoint"`,
		`"token_endpoint"`,
		`"userinfo_endpoint"`,
		`"jwks_uri"`,
		`"response_types_supported"`,
		`"grant_types_supported"`,
		`"subject_types_supported"`,
		`"id_token_signing_alg_values_supported"`,
		`"scopes_supported"`,
		`"token_endpoint_auth_methods_supported"`,
	}
	s := string(body)
	prevIdx := -1
	for _, key := range expectedOrder {
		idx := strings.Index(s, key)
		if idx < 0 {
			t.Errorf("key not found in output: %s\noutput=%s", key, s)
			continue
		}
		if idx <= prevIdx {
			t.Errorf("key %s appears at index %d, but previous was at %d (順序違反)\noutput=%s", key, idx, prevIdx, s)
		}
		prevIdx = idx
	}
}

// 出力が valid JSON であり、Marshal → Unmarshal で構造体に戻せる。
func TestMarshal_RoundTripUnmarshal(t *testing.T) {
	cfg := config.OIDCConfig{
		Issuer:                "https://id.example.com",
		AuthorizationEndpoint: "https://id.example.com/authorize",
		TokenEndpoint:         "https://id.example.com/token",
		UserInfoEndpoint:      "https://id.example.com/userinfo",
		JWKSURI:               "https://id.example.com/jwks",
	}
	body, err := discovery.Marshal(discovery.Build(cfg))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got discovery.Metadata
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("Unmarshal: %v\nbody=%s", err, body)
	}
	if got.Issuer != cfg.Issuer {
		t.Errorf("Round-trip Issuer = %q, want %q", got.Issuer, cfg.Issuer)
	}
}

// ETag: 同一 body から常に同じ値 (100 回呼び出し全一致)。
func TestETag_Deterministic_100x(t *testing.T) {
	body := []byte(`{"issuer":"https://id.example.com"}`)
	first := discovery.ETag(body)
	for i := 0; i < 99; i++ {
		got := discovery.ETag(body)
		if got != first {
			t.Fatalf("ETag は決定的でなければならない (call %d): got %q, first %q", i, got, first)
		}
	}
}

// ETag: body が 1 バイト変わると値も変わる (sha256 の avalanche を確認)。
func TestETag_BodyChangeDetection(t *testing.T) {
	a := []byte(`{"issuer":"https://id.example.com"}`)
	b := []byte(`{"issuer":"https://id.example.cox"}`) // 末尾 1 バイト違い
	if discovery.ETag(a) == discovery.ETag(b) {
		t.Error("ETag が異なる body で同一になった (sha256 衝突 = 異常)")
	}
}

// ETag フォーマット: strong ETag (引用符込み)、22 文字 base64url + 2 文字引用符 = 24 文字。
func TestETag_Format(t *testing.T) {
	body := []byte(`{}`)
	got := discovery.ETag(body)
	if len(got) != 24 {
		t.Errorf("ETag length = %d, want 24 (`\"` + 22 chars base64url + `\"`)", len(got))
	}
	if !strings.HasPrefix(got, `"`) {
		t.Errorf("ETag は `\"` で始まるべき: %q", got)
	}
	if !strings.HasSuffix(got, `"`) {
		t.Errorf("ETag は `\"` で終わるべき: %q", got)
	}
	// W/ prefix は strong ETag では付かない
	if strings.HasPrefix(got, "W/") {
		t.Errorf("ETag に W/ プレフィックスが付いた (strong ETag 違反): %q", got)
	}
	// base64.RawURLEncoding は = padding を含まない
	if strings.Contains(got, "=") {
		t.Errorf("ETag に = padding が含まれた (RawURLEncoding 違反): %q", got)
	}
}
