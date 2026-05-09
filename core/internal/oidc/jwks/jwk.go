// Package jwks は OIDC JWKS endpoint (`GET /jwks`) の handler / シリアライザ / 公開鍵 → JWK 変換を提供する
// (M1.1 で導入、設計 #32)。
//
// jwx/v3 (`github.com/lestrrat-go/jwx/v3`) を採用範囲限定で使用する (論点 #10):
//   - a. 公開鍵 → JWK 変換 = jwk.Import
//   - b. JWK Set 構築 = jwk.NewSet() + set.AddKey()
//   - c. JWK Set marshal = json.Marshal(set)  (jwx の Marshaler 実装、決定的キー順)
//   - d. kid 算出 = 自前 (P1 keystore.DeriveKid、F-11 = DER SHA-256 先頭 24 hex)
//
// JWKS の private 成分 (`d`/`p`/`q`/`dp`/`dq`/`qi`) は絶対に出力しない (Codex LOW 2)。
// 本パッケージは `keystore.KeyPair.PublicKey` のみを利用し、PrivateKey は触らない。
package jwks

import (
	"fmt"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"

	"github.com/mktkhr/id-core/core/internal/keystore"
)

// JWK 標準フィールド値 (RFC 7517 / 7518)。コードベース内の文字列リテラル散在を避ける。
const (
	jwkUseSignature = "sig"
)

// ToJWK は keystore.KeyPair の **公開鍵** を jwk.Key に変換し、
// `kty=RSA` / `alg=RS256` / `use=sig` / `kid=<kp.Kid>` を明示セットして返す (論点 #10)。
//
// jwk.Import は kty を入力鍵型から自動推論するが、本実装でも防御的に kty を再セットする
// (jwx のメジャーバージョンが上がった際の挙動変化に備えた契約テストにも該当、Codex LOW 2 反映)。
//
// 入力鍵が *rsa.PublicKey 以外の場合は jwk.Import の戻り error をそのまま返す。
func ToJWK(kp *keystore.KeyPair) (jwk.Key, error) {
	if kp == nil || kp.PublicKey == nil {
		return nil, fmt.Errorf("jwks.ToJWK: KeyPair / PublicKey が nil です")
	}

	key, err := jwk.Import(kp.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("jwks.ToJWK: jwk.Import に失敗しました: %w", err)
	}

	// kty: jwk.Import は自動セット済みだが防御的に再設定 (鍵タイプ整合の二重確認)
	if err := key.Set(jwk.KeyTypeKey, jwa.RSA()); err != nil {
		return nil, fmt.Errorf("jwks.ToJWK: kty 設定に失敗しました: %w", err)
	}
	if err := key.Set(jwk.AlgorithmKey, jwa.RS256()); err != nil {
		return nil, fmt.Errorf("jwks.ToJWK: alg 設定に失敗しました: %w", err)
	}
	if err := key.Set(jwk.KeyUsageKey, jwkUseSignature); err != nil {
		return nil, fmt.Errorf("jwks.ToJWK: use 設定に失敗しました: %w", err)
	}
	if err := key.Set(jwk.KeyIDKey, kp.Kid); err != nil {
		return nil, fmt.Errorf("jwks.ToJWK: kid 設定に失敗しました: %w", err)
	}

	return key, nil
}
