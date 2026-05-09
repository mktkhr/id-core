// Package keystore は OIDC OP の署名鍵を保管・読み込み・kid 算出する (M1.1 で導入、設計 #32)。
//
// M1.1 範囲は単一鍵保持の staticKeySet のみ。M2.x で複数鍵 + rotation 対応の multiKeySet を
// 同 KeySet インターフェースの実装として追加する想定 (Verifying() に過去鍵を含める拡張のみ)。
//
// 鍵ソースは以下の 2 通り。env 検証 (両方未指定 / 両方指定の禁止) は config.Load() で済んでいる前提。
//   - SourceFile      : K8s Secret 等にマウントされた PEM PKCS#8 鍵ファイルを読み込む (本番)
//   - SourceGenerated : 起動時に rsa.GenerateKey(rand.Reader, 2048) でメモリ生成 (開発専用)
//
// kid アルゴリズム (F-11 確定): 公開鍵 DER (x509.MarshalPKIXPublicKey) の SHA-256 → 先頭 12 バイト
// → 24 hex 文字。RFC 7638 thumbprint **非準拠**。`CORE_OIDC_KEY_ID` env で override 可能。
package keystore

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"

	"github.com/mktkhr/id-core/core/internal/logger"
)

const (
	// AlgRS256 は M1.1 範囲で唯一サポートする署名アルゴリズム識別子 (F-12)。
	AlgRS256 = "RS256"

	// KidByteLen は kid 自動算出時の元バイト長。SHA-256 出力 32 バイトの先頭側を採用。
	KidByteLen = 12
	// KidHexLen は kid の hex 表現の文字数 (= KidByteLen × 2)。
	KidHexLen = KidByteLen * 2

	// generatedKeyBitLen は SourceGenerated モードの鍵長 (RS256 の最小推奨値)。
	generatedKeyBitLen = 2048
)

// PEM ブロックタイプ識別子。Decode 結果の Type 値判定に用いる。
const (
	pemTypePKCS8     = "PRIVATE KEY"
	pemTypePKCS1     = "RSA PRIVATE KEY"
	pemTypeEncrypted = "ENCRYPTED PRIVATE KEY"
)

// KeyPair は単一鍵 (公開鍵 + 秘密鍵 + kid + alg) を保持する。
//
// JWKS では PublicKey と Kid と Alg のみを公開する (PrivateKey は出力しない、F-18)。
// 署名処理 (M1.3 で実装) は PrivateKey を利用する。
type KeyPair struct {
	Kid        string
	PublicKey  *rsa.PublicKey
	PrivateKey *rsa.PrivateKey
	Alg        string
}

// BitLen は公開鍵 modulus N のビット長を返す。
//
// nil-safe: KeyPair / PublicKey / N のいずれかが nil なら 0 を返す。
// P4 で main.go が起動時に呼び、1024 bit 等の弱い鍵に WARN ログを出すために使用する。
func (k *KeyPair) BitLen() int {
	if k == nil || k.PublicKey == nil || k.PublicKey.N == nil {
		return 0
	}
	return k.PublicKey.N.BitLen()
}

// KeySet は署名 / 検証で使う鍵セットの公開インターフェース (論点 #10 確定)。
//
//   - Active     : 署名に使う「現在鍵」。M1.1 では 1 鍵のみ。M2.x rotation 後も「最新鍵」を返す
//   - Verifying  : JWKS で広告する公開鍵集合。M1.1 では Active と同じ 1 鍵のみ。
//     M2.x rotation 中は「現在鍵 + overlap window 内の旧鍵」を返す
type KeySet interface {
	Active(ctx context.Context) (*KeyPair, error)
	Verifying(ctx context.Context) ([]*KeyPair, error)
}

// Source は鍵ソース識別子。P4 の main.go が起動 INFO ログに source=<value> として出す。
type Source int

const (
	SourceFile      Source = iota // CORE_OIDC_KEY_FILE 経由
	SourceGenerated               // CORE_OIDC_DEV_GENERATE_KEY=1 経由 (dev/staging のみ)
)

// String は構造化ログで出力する文字列表現を返す。
func (s Source) String() string {
	switch s {
	case SourceFile:
		return "file"
	case SourceGenerated:
		return "generated"
	default:
		return "unknown"
	}
}

// OIDCKeyConfig は keystore.Init への入力。config.OIDCConfig からの抜粋。
//
// 不変: env 別の必須/代替ルール (両方未指定の禁止 / prod での生成モード禁止 等) は
// config.Load() で検証済み。本構造体に渡される時点で「KeyFile か DevGenerateKey の
// どちらか一方が真」を呼び出し側が保証する。
type OIDCKeyConfig struct {
	KeyFile        string
	DevGenerateKey bool
	KeyID          string // 空文字なら DeriveKid() で自動算出
}

// Init は KeySet を構築する。
//
// 戻り値:
//   - KeySet : staticKeySet (M1.1)
//   - Source : SourceFile / SourceGenerated。P4 main.go が起動ログに含める
//   - error  : 鍵読み込み失敗 / 不正フォーマット 等
//
// logger 引数は M1.1 では未使用 (P4 で main.go 側から WARN/INFO ログを出す方針)。
// 将来 keystore 内部でログを出す可能性に備えて I/F を確保する。
func Init(ctx context.Context, cfg OIDCKeyConfig, l *logger.Logger) (KeySet, Source, error) {
	_ = ctx
	_ = l

	var (
		pair *KeyPair
		src  Source
		err  error
	)
	switch {
	case cfg.KeyFile != "":
		pair, err = loadFromFile(cfg.KeyFile)
		src = SourceFile
	case cfg.DevGenerateKey:
		pair, err = generateInMemory()
		src = SourceGenerated
	default:
		return nil, 0, errors.New("keystore: KeyFile か DevGenerateKey のいずれかを指定してください (config.Load 段階で検証済みのはず)")
	}
	if err != nil {
		return nil, 0, err
	}

	// kid override (空文字なら自動算出値を維持)
	if cfg.KeyID != "" {
		pair.Kid = cfg.KeyID
	}

	return &staticKeySet{pair: pair}, src, nil
}

// DeriveKid は公開鍵 DER (PKIX 形式) の SHA-256 先頭 12 バイトを 24 hex 文字で返す (F-11)。
//
// 同じ公開鍵から常に同じ値を返す決定論的関数。RFC 7638 JWK Thumbprint **非準拠**
// (Thumbprint は JSON canonical 化が必要で実装複雑度が高いため、id-core 独自仕様を採用)。
//
// ログ表記は「kid」または「fingerprint」とし「thumbprint」は使わない (規約書に明記)。
func DeriveKid(pub *rsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", fmt.Errorf("公開鍵 DER 変換に失敗しました: %w", err)
	}
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:KidByteLen]), nil
}

// staticKeySet は M1.1 範囲の単一鍵保持実装。Active / Verifying ともに同じ 1 鍵を返す。
type staticKeySet struct {
	pair *KeyPair
}

func (s *staticKeySet) Active(ctx context.Context) (*KeyPair, error) {
	_ = ctx
	if s == nil || s.pair == nil {
		return nil, errors.New("keystore: アクティブ鍵が未初期化です (Init 失敗)")
	}
	return s.pair, nil
}

func (s *staticKeySet) Verifying(ctx context.Context) ([]*KeyPair, error) {
	_ = ctx
	if s == nil || s.pair == nil {
		return nil, errors.New("keystore: 検証鍵セットが未初期化です (Init 失敗)")
	}
	return []*KeyPair{s.pair}, nil
}

func loadFromFile(path string) (*KeyPair, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- 鍵ファイルパスは config.Load で検証済みの env から渡る
	if err != nil {
		return nil, fmt.Errorf("keystore: 鍵ファイルの読み込みに失敗しました: %s: %w", path, err)
	}
	return parsePKCS8PEM(data)
}

func generateInMemory() (*KeyPair, error) {
	key, err := rsa.GenerateKey(rand.Reader, generatedKeyBitLen)
	if err != nil {
		return nil, fmt.Errorf("keystore: RSA %d 鍵の生成に失敗しました: %w", generatedKeyBitLen, err)
	}
	return buildKeyPair(key)
}

// parsePKCS8PEM は PEM バイト列を PKCS#8 RSA 鍵として解析し、不一致は明確なエラーを返す。
//
// PKCS#1 (`RSA PRIVATE KEY`) / encrypted PEM / 非 RSA 鍵の検出は本マイルストーンの仕様
// (論点 #10 Codex MEDIUM 2 反映)。エラー文言には運用回避策を含める。
func parsePKCS8PEM(data []byte) (*KeyPair, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("keystore: PEM の解析に失敗しました: 有効な PEM ブロックが含まれていません (PKCS#8 PEM を指定してください)")
	}
	switch block.Type {
	case pemTypePKCS8:
		// OK
	case pemTypePKCS1:
		return nil, errors.New("keystore: 鍵フォーマットが PKCS#8 ではありません (PKCS#1 形式)。`openssl pkcs8 -topk8 -nocrypt -in <pkcs1> -out <pkcs8>` で変換してください")
	case pemTypeEncrypted:
		return nil, errors.New("keystore: encrypted PEM は非対応です。復号済み PEM を K8s Secret に配置してください")
	default:
		return nil, fmt.Errorf("keystore: 鍵フォーマットが PKCS#8 ではありません: PEM タイプ %q (期待: %q)", block.Type, pemTypePKCS8)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("keystore: PKCS#8 鍵の解析に失敗しました: %w", err)
	}
	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("keystore: RS256 では RSA 鍵が必要です。受け取った鍵タイプ: %T", parsed)
	}
	return buildKeyPair(rsaKey)
}

func buildKeyPair(key *rsa.PrivateKey) (*KeyPair, error) {
	kid, err := DeriveKid(&key.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("keystore: kid 算出に失敗しました: %w", err)
	}
	return &KeyPair{
		Kid:        kid,
		PublicKey:  &key.PublicKey,
		PrivateKey: key,
		Alg:        AlgRS256,
	}, nil
}
