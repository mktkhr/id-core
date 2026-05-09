package discovery_test

import (
	"reflect"
	"testing"

	"github.com/mktkhr/id-core/core/internal/config"
	"github.com/mktkhr/id-core/core/internal/oidc/discovery"
)

// 基本ケース: 全 11 フィールドが期待値で埋まる。
func TestBuild_BasicFields(t *testing.T) {
	cfg := config.OIDCConfig{
		Issuer:                "https://id.example.com",
		AuthorizationEndpoint: "https://id.example.com/authorize",
		TokenEndpoint:         "https://id.example.com/token",
		UserInfoEndpoint:      "https://id.example.com/userinfo",
		JWKSURI:               "https://id.example.com/jwks",
	}
	m := discovery.Build(cfg)

	if m.Issuer != "https://id.example.com" {
		t.Errorf("Issuer = %q", m.Issuer)
	}
	if m.AuthorizationEndpoint != "https://id.example.com/authorize" {
		t.Errorf("AuthorizationEndpoint = %q", m.AuthorizationEndpoint)
	}
	if m.TokenEndpoint != "https://id.example.com/token" {
		t.Errorf("TokenEndpoint = %q", m.TokenEndpoint)
	}
	if m.UserinfoEndpoint != "https://id.example.com/userinfo" {
		t.Errorf("UserinfoEndpoint = %q", m.UserinfoEndpoint)
	}
	if m.JWKSURI != "https://id.example.com/jwks" {
		t.Errorf("JWKSURI = %q", m.JWKSURI)
	}

	// supported 配列の M1.1 確定値 (F-4)
	cases := []struct {
		name string
		got  []string
		want []string
	}{
		{name: "ResponseTypesSupported", got: m.ResponseTypesSupported, want: []string{"code"}},
		{name: "GrantTypesSupported", got: m.GrantTypesSupported, want: []string{"authorization_code"}},
		{name: "SubjectTypesSupported", got: m.SubjectTypesSupported, want: []string{"public"}},
		{name: "IDTokenSigningAlgValuesSupported", got: m.IDTokenSigningAlgValuesSupported, want: []string{"RS256"}},
		{name: "ScopesSupported", got: m.ScopesSupported, want: []string{"openid"}},
		{name: "TokenEndpointAuthMethodsSupported", got: m.TokenEndpointAuthMethodsSupported, want: []string{"client_secret_basic"}},
	}
	for _, tc := range cases {
		if !reflect.DeepEqual(tc.got, tc.want) {
			t.Errorf("%s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

// ContractTest 5 ケース (Q8 / F-17): issuer 形式の違いで endpoint URL が正しく組み立つ。
//
// 本テストでは config.OIDCConfig を直接組み立てる (config.Load を経由せず、Build の挙動を
// 単離テスト)。issuer 由来の endpoint 構築は config.Load 側で実施されるため、
// 本テストは Build がそれをそのまま反映することを検証する。
func TestBuild_ContractTest_5Cases(t *testing.T) {
	cases := []struct {
		name              string
		issuer            string
		authorizationEnd  string
		tokenEnd          string
		userInfoEnd       string
		jwksURI           string
		wantAuthorization string
	}{
		{
			name:              "1. 標準",
			issuer:            "https://id.example.com",
			authorizationEnd:  "https://id.example.com/authorize",
			tokenEnd:          "https://id.example.com/token",
			userInfoEnd:       "https://id.example.com/userinfo",
			jwksURI:           "https://id.example.com/jwks",
			wantAuthorization: "https://id.example.com/authorize",
		},
		{
			name:              "2. subpath",
			issuer:            "https://example.com/id-core",
			authorizationEnd:  "https://example.com/id-core/authorize",
			tokenEnd:          "https://example.com/id-core/token",
			userInfoEnd:       "https://example.com/id-core/userinfo",
			jwksURI:           "https://example.com/id-core/jwks",
			wantAuthorization: "https://example.com/id-core/authorize",
		},
		{
			name:              "3. 末尾スラッシュ (config 側で strip 済前提)",
			issuer:            "https://example.com/id-core",
			authorizationEnd:  "https://example.com/id-core/authorize",
			tokenEnd:          "https://example.com/id-core/token",
			userInfoEnd:       "https://example.com/id-core/userinfo",
			jwksURI:           "https://example.com/id-core/jwks",
			wantAuthorization: "https://example.com/id-core/authorize",
		},
		{
			name:              "4. dev 非 https",
			issuer:            "http://localhost:8080",
			authorizationEnd:  "http://localhost:8080/authorize",
			tokenEnd:          "http://localhost:8080/token",
			userInfoEnd:       "http://localhost:8080/userinfo",
			jwksURI:           "http://localhost:8080/jwks",
			wantAuthorization: "http://localhost:8080/authorize",
		},
		{
			name:              "5. 非標準ポート",
			issuer:            "https://id.example.com:9443",
			authorizationEnd:  "https://id.example.com:9443/authorize",
			tokenEnd:          "https://id.example.com:9443/token",
			userInfoEnd:       "https://id.example.com:9443/userinfo",
			jwksURI:           "https://id.example.com:9443/jwks",
			wantAuthorization: "https://id.example.com:9443/authorize",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.OIDCConfig{
				Issuer:                tc.issuer,
				AuthorizationEndpoint: tc.authorizationEnd,
				TokenEndpoint:         tc.tokenEnd,
				UserInfoEndpoint:      tc.userInfoEnd,
				JWKSURI:               tc.jwksURI,
			}
			m := discovery.Build(cfg)
			if m.Issuer != tc.issuer {
				t.Errorf("Issuer = %q, want %q", m.Issuer, tc.issuer)
			}
			if m.AuthorizationEndpoint != tc.wantAuthorization {
				t.Errorf("AuthorizationEndpoint = %q, want %q", m.AuthorizationEndpoint, tc.wantAuthorization)
			}
			// 他の endpoint も入力通り反映されることを確認
			if m.TokenEndpoint != tc.tokenEnd {
				t.Errorf("TokenEndpoint mismatch")
			}
			if m.UserinfoEndpoint != tc.userInfoEnd {
				t.Errorf("UserinfoEndpoint mismatch")
			}
			if m.JWKSURI != tc.jwksURI {
				t.Errorf("JWKSURI mismatch")
			}
		})
	}
}

// endpoint 個別 override が独立に反映される (論点 #12)。
func TestBuild_EndpointOverridesIndependently(t *testing.T) {
	cfg := config.OIDCConfig{
		Issuer:                "https://id.example.com",
		AuthorizationEndpoint: "https://auth.other.example.com/authorize",
		TokenEndpoint:         "https://id.example.com/token", // override せず issuer 由来
		UserInfoEndpoint:      "https://userinfo.other.example.com/me",
		JWKSURI:               "https://cdn.example.com/jwks.json",
	}
	m := discovery.Build(cfg)
	if m.AuthorizationEndpoint != "https://auth.other.example.com/authorize" {
		t.Errorf("AuthorizationEndpoint override 不適用: %q", m.AuthorizationEndpoint)
	}
	if m.TokenEndpoint != "https://id.example.com/token" {
		t.Errorf("TokenEndpoint = %q (issuer 由来でなければならない)", m.TokenEndpoint)
	}
	if m.UserinfoEndpoint != "https://userinfo.other.example.com/me" {
		t.Errorf("UserinfoEndpoint override 不適用: %q", m.UserinfoEndpoint)
	}
	if m.JWKSURI != "https://cdn.example.com/jwks.json" {
		t.Errorf("JWKSURI override 不適用: %q", m.JWKSURI)
	}
}
