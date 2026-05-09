package discovery

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
)

// etagBodyByteLen は ETag 計算で sha256 出力 32 バイトの先頭から採用するバイト数。
// 16 バイト = 128 bit の衝突耐性で実用上十分 (論点 #4 確定)。
const etagBodyByteLen = 16

// Marshal は Metadata を OIDC Discovery レスポンスとして決定的にシリアライズする (F-21)。
//
// 決定論性の根拠:
//   - encoding/json は struct フィールドの宣言順を維持して出力する
//   - json.Marshal は indent / 余分な空白を入れず、改行も追加しない
//   - 配列要素 (supportedResponseTypes 等) はパッケージ変数で固定順序
//
// これにより同一鍵セット (実際には鍵に依存しないが Metadata 全体) で 100 回呼び出しても
// 全て同一バイト列となり、ETag が安定する。
func Marshal(m Metadata) ([]byte, error) {
	return json.Marshal(m)
}

// ETag はレスポンス body から strong ETag を計算する (論点 #4 確定)。
//
// 計算式: SHA-256(body) → 先頭 16 バイト → base64.RawURLEncoding (no-padding) → ダブルクォート囲み
//
// 出力例: `"abcd_efghIJKLmnopQRSt"` (24 文字 = 22 文字 base64url + 2 文字引用符)
//
// strong ETag (RFC 7232 §2.3) として扱うため、`W/` プレフィックスは付けない。
// HTTP クライアント (RP の OIDC ライブラリ) は If-None-Match ヘッダで本値を完全一致比較する。
//
// 16 バイト = 128 bit を採用する根拠 (論点 #4):
//   - メタデータ本数は実用上 1 本/Pod で衝突確率は事実上 0
//   - 本値が短いほうがログ・ヘッダの可読性が高い (32 バイト = 43 文字は冗長)
func ETag(body []byte) string {
	sum := sha256.Sum256(body)
	return `"` + base64.RawURLEncoding.EncodeToString(sum[:etagBodyByteLen]) + `"`
}
