// Package discovery は OIDC Discovery 1.0 / RFC 8414 の OP メタデータを構築・配信する
// (M1.1 で導入、設計 #32)。
//
// `GET /.well-known/openid-configuration` のレスポンス body 構築 + handler を提供する。
// route 登録は P4 (`server.go`) で行う。
//
// メタデータ構築は config.OIDCConfig からの単純なフィールドコピー + 既定値だが、
// `id_token_signing_alg_values_supported` / `response_types_supported` 等の M1.1 広告内容は
// 設計 #32 で確定済みの「最小セット」を採用する (F-4)。後続マイルストーンで実装が進む際に
// 配列要素を追加する。
package discovery

import (
	"github.com/mktkhr/id-core/core/internal/config"
)

// 各 supported 配列の M1.1 広告内容 (F-4 確定値)。
// パッケージ変数として定義し、Build() の各呼び出しで slice を共有する (Metadata は読み取り専用扱い)。
//
// セキュリティ: 受信側 (RP) は配列を読み取るのみで mutate しない前提。
// もし mutate されると後続呼び出しに伝搬するが、Metadata 構造体を public に晒さず
// `Marshal` でバイト列化する用途のみのため実害はない。
var (
	supportedResponseTypes      = []string{"code"}
	supportedGrantTypes         = []string{"authorization_code"}
	supportedSubjectTypes       = []string{"public"}
	supportedIDTokenSigningAlgs = []string{"RS256"}
	supportedScopes             = []string{"openid"}
	supportedTokenAuthMethods   = []string{"client_secret_basic"}
)

// Metadata は OIDC Discovery 1.0 / RFC 8414 のレスポンス JSON 構造を表す。
//
// JSON キー順序は struct field 順 (encoding/json は宣言順を保つ)。OIDC Discovery 1.0 慣習順
// (issuer → endpoints → supported features) に並べることで人間可読性を担保する。
//
// M1.1 範囲では 11 フィールドを広告。後続マイルストーンで追加する想定 (例: `claims_supported`,
// `code_challenge_methods_supported`, `request_object_signing_alg_values_supported` 等)。
type Metadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	UserinfoEndpoint                  string   `json:"userinfo_endpoint"`
	JWKSURI                           string   `json:"jwks_uri"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	SubjectTypesSupported             []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported  []string `json:"id_token_signing_alg_values_supported"`
	ScopesSupported                   []string `json:"scopes_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
}

// Build は config.OIDCConfig から Metadata を構築する (F-2 / F-4)。
//
// 各 endpoint URL は config.Load 段階で env override or issuer ベースの URL.JoinPath 構築が
// 完了している前提 (本関数は単純にフィールドコピー)。supported 配列は M1.1 確定値を埋め込む。
//
// config 構造体を直接受けることで余計なアダプタを増やさない (config パッケージは既に
// internal/* から依存される基盤層であり、依存方向 oidc/discovery → config は安全)。
func Build(cfg config.OIDCConfig) Metadata {
	return Metadata{
		Issuer:                            cfg.Issuer,
		AuthorizationEndpoint:             cfg.AuthorizationEndpoint,
		TokenEndpoint:                     cfg.TokenEndpoint,
		UserinfoEndpoint:                  cfg.UserInfoEndpoint,
		JWKSURI:                           cfg.JWKSURI,
		ResponseTypesSupported:            supportedResponseTypes,
		GrantTypesSupported:               supportedGrantTypes,
		SubjectTypesSupported:             supportedSubjectTypes,
		IDTokenSigningAlgValuesSupported:  supportedIDTokenSigningAlgs,
		ScopesSupported:                   supportedScopes,
		TokenEndpointAuthMethodsSupported: supportedTokenAuthMethods,
	}
}
