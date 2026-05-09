package keystore_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mktkhr/id-core/core/internal/keystore"
	"github.com/mktkhr/id-core/core/internal/logger"
)

// newTestLogger は keystore.Init に渡すための「黙る」ロガー。
// keystore は M1.1 では logger を使わない (P4 で main.go 側からログ出力) が
// I/F 上必要なため、buffer 出力で副作用を切り離した状態を作る。
func newTestLogger(t *testing.T) *logger.Logger {
	t.Helper()
	return logger.New(logger.FormatJSON, &bytes.Buffer{})
}

// writePKCS8PEM は RSA 秘密鍵を PKCS#8 PEM ファイルとして書き出す。
func writePKCS8PEM(t *testing.T, key *rsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	block := &pem.Block{Type: "PRIVATE KEY", Bytes: der}
	path := filepath.Join(t.TempDir(), "signing.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// writeRawPEM は任意の PEM タイプ + body をファイル化する (異常系テスト用)。
func writeRawPEM(t *testing.T, blockType string, body []byte) string {
	t.Helper()
	block := &pem.Block{Type: blockType, Bytes: body}
	path := filepath.Join(t.TempDir(), "broken.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// ----------------------------------------------------------------------------
// 正常系
// ----------------------------------------------------------------------------

// ファイルモード正常系: PKCS#8 PEM ロード → kid 計算 → Active / Verifying が機能。
func TestInit_FileMode_Success(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	path := writePKCS8PEM(t, rsaKey)

	ks, src, err := keystore.Init(context.Background(),
		keystore.OIDCKeyConfig{KeyFile: path}, newTestLogger(t))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if src != keystore.SourceFile {
		t.Errorf("Source = %v, want SourceFile", src)
	}

	pair, err := ks.Active(context.Background())
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if pair.Alg != keystore.AlgRS256 {
		t.Errorf("Alg = %q, want %q", pair.Alg, keystore.AlgRS256)
	}
	if got := len(pair.Kid); got != keystore.KidHexLen {
		t.Errorf("Kid len = %d, want %d", got, keystore.KidHexLen)
	}
	if pair.PrivateKey == nil || pair.PublicKey == nil {
		t.Errorf("PrivateKey or PublicKey is nil")
	}

	verifying, err := ks.Verifying(context.Background())
	if err != nil {
		t.Fatalf("Verifying: %v", err)
	}
	if len(verifying) != 1 {
		t.Fatalf("Verifying() len = %d, want 1", len(verifying))
	}
	if verifying[0].Kid != pair.Kid {
		t.Errorf("Verifying() kid = %q, Active() kid = %q (single key expected to match)", verifying[0].Kid, pair.Kid)
	}
}

// 起動時生成モード正常系: メモリ生成 → 2048 bit / RS256 / kid 自動算出。
func TestInit_GenerateMode_Success(t *testing.T) {
	ks, src, err := keystore.Init(context.Background(),
		keystore.OIDCKeyConfig{DevGenerateKey: true}, newTestLogger(t))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if src != keystore.SourceGenerated {
		t.Errorf("Source = %v, want SourceGenerated", src)
	}

	pair, err := ks.Active(context.Background())
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if pair.BitLen() != 2048 {
		t.Errorf("BitLen = %d, want 2048", pair.BitLen())
	}
	if pair.Alg != keystore.AlgRS256 {
		t.Errorf("Alg = %q, want %q", pair.Alg, keystore.AlgRS256)
	}
	if pair.PrivateKey == nil {
		t.Errorf("PrivateKey nil")
	}
}

// kid 決定論: 同じ公開鍵から常に同じ kid (100 回呼び出し全一致)。
func TestDeriveKid_Deterministic_100x(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	first, err := keystore.DeriveKid(&rsaKey.PublicKey)
	if err != nil {
		t.Fatalf("DeriveKid first: %v", err)
	}
	if got := len(first); got != keystore.KidHexLen {
		t.Errorf("kid len = %d, want %d", got, keystore.KidHexLen)
	}
	for i := 0; i < 99; i++ {
		kid, err := keystore.DeriveKid(&rsaKey.PublicKey)
		if err != nil {
			t.Fatalf("DeriveKid #%d: %v", i, err)
		}
		if kid != first {
			t.Fatalf("kid 不一致 (call %d): got %q, first %q", i, kid, first)
		}
	}
}

// kid override: CORE_OIDC_KEY_ID 設定値が自動算出より優先。
func TestInit_KeyIDOverride(t *testing.T) {
	const customKid = "custom-kid-2026-05"
	ks, _, err := keystore.Init(context.Background(),
		keystore.OIDCKeyConfig{DevGenerateKey: true, KeyID: customKid},
		newTestLogger(t))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	pair, err := ks.Active(context.Background())
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if pair.Kid != customKid {
		t.Errorf("Kid = %q, want %q (override should take precedence)", pair.Kid, customKid)
	}
}

// 鍵長透過: RSA 1024 / 2048 / 3072 を全てロード成功 (4096 は -short skip 別関数)。
func TestInit_FileMode_KeyLengthTransparent(t *testing.T) {
	bits := []int{1024, 2048, 3072}
	for _, b := range bits {
		b := b
		t.Run(fmt.Sprintf("RSA-%d", b), func(t *testing.T) {
			t.Parallel()
			rsaKey, err := rsa.GenerateKey(rand.Reader, b)
			if err != nil {
				t.Fatalf("GenerateKey(%d): %v", b, err)
			}
			path := writePKCS8PEM(t, rsaKey)
			ks, _, err := keystore.Init(context.Background(),
				keystore.OIDCKeyConfig{KeyFile: path}, newTestLogger(t))
			if err != nil {
				t.Fatalf("Init: %v", err)
			}
			pair, err := ks.Active(context.Background())
			if err != nil {
				t.Fatalf("Active: %v", err)
			}
			if got := pair.BitLen(); got != b {
				t.Errorf("BitLen = %d, want %d", got, b)
			}
		})
	}
}

// 4096 bit は生成に時間がかかるので -short でスキップ可能な独立関数に分離。
func TestInit_FileMode_RSA4096(t *testing.T) {
	if testing.Short() {
		t.Skip("RSA 4096 generation is slow; skipping in -short mode")
	}
	rsaKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Fatalf("GenerateKey(4096): %v", err)
	}
	path := writePKCS8PEM(t, rsaKey)
	ks, _, err := keystore.Init(context.Background(),
		keystore.OIDCKeyConfig{KeyFile: path}, newTestLogger(t))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	pair, err := ks.Active(context.Background())
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if pair.BitLen() != 4096 {
		t.Errorf("BitLen = %d, want 4096", pair.BitLen())
	}
}

// ----------------------------------------------------------------------------
// 異常系
// ----------------------------------------------------------------------------

// 異常系: PKCS#1 PEM (`-----BEGIN RSA PRIVATE KEY-----`) を渡したら明確エラー。
func TestInit_FileMode_PKCS1Rejected(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	der := x509.MarshalPKCS1PrivateKey(rsaKey)
	path := writeRawPEM(t, "RSA PRIVATE KEY", der)

	_, _, err = keystore.Init(context.Background(),
		keystore.OIDCKeyConfig{KeyFile: path}, newTestLogger(t))
	if err == nil {
		t.Fatal("PKCS#1 PEM should be rejected")
	}
	if !strings.Contains(err.Error(), "PKCS#8") {
		t.Errorf("error msg lacks 'PKCS#8' hint: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "openssl pkcs8") {
		t.Errorf("error msg lacks recovery command 'openssl pkcs8': %q", err.Error())
	}
}

// 異常系: encrypted PEM (`-----BEGIN ENCRYPTED PRIVATE KEY-----`) は非対応。
func TestInit_FileMode_EncryptedRejected(t *testing.T) {
	// body は dummy で OK。block.Type だけで判定し、ParsePKCS8 まで到達しない。
	path := writeRawPEM(t, "ENCRYPTED PRIVATE KEY", []byte("dummy"))

	_, _, err := keystore.Init(context.Background(),
		keystore.OIDCKeyConfig{KeyFile: path}, newTestLogger(t))
	if err == nil {
		t.Fatal("encrypted PEM should be rejected")
	}
	if !strings.Contains(err.Error(), "encrypted PEM は非対応") {
		t.Errorf("error msg lacks 'encrypted PEM は非対応': %q", err.Error())
	}
	if !strings.Contains(err.Error(), "復号済み PEM") {
		t.Errorf("error msg lacks recovery hint '復号済み PEM': %q", err.Error())
	}
}

// 異常系: 不正 PEM (Base64 デコード不能 / PEM ヘッダなし)。
func TestInit_FileMode_BrokenPEM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.pem")
	if err := os.WriteFile(path, []byte("this is not a pem"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, _, err := keystore.Init(context.Background(),
		keystore.OIDCKeyConfig{KeyFile: path}, newTestLogger(t))
	if err == nil {
		t.Fatal("broken PEM should be rejected")
	}
	if !strings.Contains(err.Error(), "PEM の解析に失敗") {
		t.Errorf("error msg lacks 'PEM の解析に失敗': %q", err.Error())
	}
}

// 異常系: 空ファイル。
func TestInit_FileMode_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.pem")
	if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, _, err := keystore.Init(context.Background(),
		keystore.OIDCKeyConfig{KeyFile: path}, newTestLogger(t))
	if err == nil {
		t.Fatal("empty file should be rejected")
	}
}

// 異常系: ファイルなし。
func TestInit_FileMode_FileNotFound(t *testing.T) {
	_, _, err := keystore.Init(context.Background(),
		keystore.OIDCKeyConfig{KeyFile: "/nonexistent/__id-core_test_no_such_path__.pem"},
		newTestLogger(t))
	if err == nil {
		t.Fatal("missing file should be rejected")
	}
	if !strings.Contains(err.Error(), "鍵ファイルの読み込みに失敗") {
		t.Errorf("error msg lacks '鍵ファイルの読み込みに失敗': %q", err.Error())
	}
}

// 異常系: PKCS#8 内に EC 鍵 (RSA でない) → RS256 必須エラー。
func TestInit_FileMode_NonRSARejected(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(ecKey)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey(EC): %v", err)
	}
	path := writeRawPEM(t, "PRIVATE KEY", der)

	_, _, err = keystore.Init(context.Background(),
		keystore.OIDCKeyConfig{KeyFile: path}, newTestLogger(t))
	if err == nil {
		t.Fatal("non-RSA key should be rejected")
	}
	if !strings.Contains(err.Error(), "RS256 では RSA 鍵") {
		t.Errorf("error msg lacks 'RS256 では RSA 鍵': %q", err.Error())
	}
}

// 異常系: KeyFile も DevGenerateKey も未指定 (config.Load で防がれるはずだが防御的検証)。
func TestInit_NoKeySource(t *testing.T) {
	_, _, err := keystore.Init(context.Background(),
		keystore.OIDCKeyConfig{}, newTestLogger(t))
	if err == nil {
		t.Fatal("no key source should be rejected")
	}
	if !strings.Contains(err.Error(), "KeyFile") {
		t.Errorf("error msg lacks 'KeyFile': %q", err.Error())
	}
}

// 異常系: PKCS#8 タイプ以外の PEM (例: CERTIFICATE)。
func TestInit_FileMode_UnknownPEMType(t *testing.T) {
	path := writeRawPEM(t, "CERTIFICATE", []byte("dummy"))
	_, _, err := keystore.Init(context.Background(),
		keystore.OIDCKeyConfig{KeyFile: path}, newTestLogger(t))
	if err == nil {
		t.Fatal("unknown PEM type should be rejected")
	}
	if !strings.Contains(err.Error(), "PKCS#8 ではありません") {
		t.Errorf("error msg lacks 'PKCS#8 ではありません': %q", err.Error())
	}
}

// ----------------------------------------------------------------------------
// BitLen / Source / nil 防御
// ----------------------------------------------------------------------------

// BitLen() が期待通りの値を返す (1024 / 2048 / 3072)。
func TestKeyPair_BitLen(t *testing.T) {
	for _, b := range []int{1024, 2048, 3072} {
		b := b
		t.Run(fmt.Sprintf("RSA-%d", b), func(t *testing.T) {
			t.Parallel()
			rsaKey, err := rsa.GenerateKey(rand.Reader, b)
			if err != nil {
				t.Fatalf("GenerateKey: %v", err)
			}
			pair := &keystore.KeyPair{PublicKey: &rsaKey.PublicKey}
			if got := pair.BitLen(); got != b {
				t.Errorf("BitLen = %d, want %d", got, b)
			}
		})
	}
}

// nil-safe: BitLen() は KeyPair / PublicKey / N が nil でも 0 を返す。
func TestKeyPair_BitLen_NilSafe(t *testing.T) {
	cases := []struct {
		name string
		pair *keystore.KeyPair
	}{
		{name: "nil pair", pair: nil},
		{name: "nil PublicKey", pair: &keystore.KeyPair{}},
		// PublicKey 内の N=nil ケースは型構造上 *rsa.PublicKey の N が nil の場合
		{name: "nil N", pair: &keystore.KeyPair{PublicKey: &rsa.PublicKey{}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.pair.BitLen(); got != 0 {
				t.Errorf("BitLen = %d, want 0", got)
			}
		})
	}
}

// Source.String() の出力。
func TestSource_String(t *testing.T) {
	cases := []struct {
		s    keystore.Source
		want string
	}{
		{s: keystore.SourceFile, want: "file"},
		{s: keystore.SourceGenerated, want: "generated"},
		{s: keystore.Source(99), want: "unknown"},
	}
	for _, tc := range cases {
		if got := tc.s.String(); got != tc.want {
			t.Errorf("Source(%d).String() = %q, want %q", tc.s, got, tc.want)
		}
	}
}
