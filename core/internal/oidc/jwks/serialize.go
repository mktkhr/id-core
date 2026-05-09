package jwks

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/lestrrat-go/jwx/v3/jwk"

	"github.com/mktkhr/id-core/core/internal/keystore"
)

// etagBodyByteLen は ETag 計算で sha256 出力 32 バイトの先頭から採用するバイト数。
// 16 バイト = 128 bit の衝突耐性で実用上十分 (論点 #4 確定、Discovery と同一仕様)。
const etagBodyByteLen = 16

// BuildSet は keystore.KeyPair の集合から jwk.Set を構築する (論点 #10 b)。
//
// 1 鍵 (M1.1) でも複数鍵 (M2.x rotation 後) でも同じ I/F で扱える。
// Set 内の順序は引数 keys の順序を保つ (jwk.Set の AddKey は append 動作)。
//
// 失敗パターン: ToJWK の失敗 (PublicKey が nil 等)、AddKey の失敗 (jwx 内部の重複 kid 等)。
func BuildSet(keys []*keystore.KeyPair) (jwk.Set, error) {
	set := jwk.NewSet()
	for i, kp := range keys {
		k, err := ToJWK(kp)
		if err != nil {
			return nil, fmt.Errorf("jwks.BuildSet: keys[%d] の JWK 変換に失敗しました: %w", i, err)
		}
		if err := set.AddKey(k); err != nil {
			return nil, fmt.Errorf("jwks.BuildSet: keys[%d] の Set 追加に失敗しました: %w", i, err)
		}
	}
	return set, nil
}

// Marshal は jwk.Set を OIDC JWKS レスポンスとして決定的にシリアライズする (F-21、論点 #10 c)。
//
// 決定論性の根拠:
//   - jwx/v3 は jwk.Set 用の MarshalJSON を実装しており、キー順序が固定 (kty / use / alg / kid / n / e ...)
//   - encoding/json は indent / 余分な空白を入れず、改行も追加しない
//   - 配列要素 (Set 内の鍵) は AddKey の挿入順を保つ
//
// 同一鍵セットで 100 回呼び出しても全て同一バイト列となり、ETag が安定する。
// jwx の minor バージョンアップで内部キー順序が変わる可能性に備えて golden test も併用する。
func Marshal(set jwk.Set) ([]byte, error) {
	if set == nil {
		return nil, fmt.Errorf("jwks.Marshal: set が nil です")
	}
	return json.Marshal(set)
}

// ETag は JWKS レスポンス body から strong ETag を計算する (論点 #4)。
//
// 計算式: SHA-256(body) → 先頭 16 バイト → base64.RawURLEncoding (no-padding) → ダブルクォート囲み。
// Discovery と同一仕様 (再利用関数の検討は P4 でリファクタ余地ありだが M1.1 では各パッケージ個別実装)。
func ETag(body []byte) string {
	sum := sha256.Sum256(body)
	return `"` + base64.RawURLEncoding.EncodeToString(sum[:etagBodyByteLen]) + `"`
}
