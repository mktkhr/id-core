package main

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mktkhr/id-core/core/internal/keystore"
	"github.com/mktkhr/id-core/core/internal/logger"
)

// 既定出力先 (./dev-keys/) は cwd 依存になるため、テストでは常に -out で
// t.TempDir() を渡す。既定値 (../../dev-keys/) を直接生成すると CI / 並列実行で
// 衝突する。

// 既定挙動: -out <tmpdir> で signing.pem + signing.pub.pem が生成される。
func TestRun_GeneratesPair(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := run([]string{"-out", dir}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run = %d, want %d (stderr=%q)", code, exitOK, stderr.String())
	}

	privPath := filepath.Join(dir, privateKeyFileName)
	pubPath := filepath.Join(dir, publicKeyFileName)

	for _, p := range []string{privPath, pubPath} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file missing: %s: %v", p, err)
		}
	}
}

// permission: signing.pem = 0600 / signing.pub.pem = 0644 (F-13 / F-14)。
func TestRun_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := run([]string{"-out", dir}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run = %d, want %d (stderr=%q)", code, exitOK, stderr.String())
	}

	cases := []struct {
		path string
		want os.FileMode
	}{
		{path: filepath.Join(dir, privateKeyFileName), want: 0o600},
		{path: filepath.Join(dir, publicKeyFileName), want: 0o644},
	}
	for _, tc := range cases {
		info, err := os.Stat(tc.path)
		if err != nil {
			t.Fatalf("Stat(%s): %v", tc.path, err)
		}
		// Perm() で permission ビットのみを比較 (type bits を除外)。
		if got := info.Mode().Perm(); got != tc.want {
			t.Errorf("%s permission = %o, want %o", tc.path, got, tc.want)
		}
	}
}

// 既存ファイル保護: -force なしで再実行するとエラー (既存ファイル不上書き)。
func TestRun_PreventOverwrite_WithoutForce(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer

	// 1 回目: 成功
	if code := run([]string{"-out", dir}, &stdout, &stderr); code != exitOK {
		t.Fatalf("first run failed: %d (stderr=%q)", code, stderr.String())
	}
	stderr.Reset()

	// 2 回目: -force なし → エラー
	code := run([]string{"-out", dir}, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("second run = %d, want %d (overwrite should be prevented)", code, exitError)
	}
	if !strings.Contains(stderr.String(), "既存ファイルが存在します") {
		t.Errorf("stderr lacks '既存ファイルが存在します': %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "-force") {
		t.Errorf("stderr lacks '-force' hint: %q", stderr.String())
	}
}

// -force あり → 上書き成功。
func TestRun_OverwriteWithForce(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer

	if code := run([]string{"-out", dir}, &stdout, &stderr); code != exitOK {
		t.Fatalf("first run failed: %d", code)
	}

	// 1 回目の中身 (秘密鍵) を取得
	priv1, err := os.ReadFile(filepath.Join(dir, privateKeyFileName))
	if err != nil {
		t.Fatalf("ReadFile priv1: %v", err)
	}

	stderr.Reset()
	stdout.Reset()
	if code := run([]string{"-out", dir, "-force"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("second run with -force failed: %d (stderr=%q)", code, stderr.String())
	}

	priv2, err := os.ReadFile(filepath.Join(dir, privateKeyFileName))
	if err != nil {
		t.Fatalf("ReadFile priv2: %v", err)
	}

	// 同じ rand.Reader でも GenerateKey の都度生成のため秘密鍵バイト列は変わるはず。
	if bytes.Equal(priv1, priv2) {
		t.Error("expected priv1 != priv2 after regenerate, but they were equal")
	}
}

// 出力 PEM が PKCS#8 形式 (pemTypePrivate) であること。
func TestRun_PrivateKeyIsPKCS8(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer

	if code := run([]string{"-out", dir}, &stdout, &stderr); code != exitOK {
		t.Fatalf("run failed: %d", code)
	}

	body, err := os.ReadFile(filepath.Join(dir, privateKeyFileName))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	block, _ := pem.Decode(body)
	if block == nil {
		t.Fatalf("PEM decode failed for private key")
	}
	if block.Type != pemTypePrivate {
		t.Errorf("private key PEM type = %q, want %q", block.Type, pemTypePrivate)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("ParsePKCS8PrivateKey: %v", err)
	}
	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		t.Fatalf("private key is not RSA: %T", parsed)
	}
	if rsaKey.N.BitLen() != keyBits {
		t.Errorf("private key bit length = %d, want %d", rsaKey.N.BitLen(), keyBits)
	}
}

// 出力 PEM の公開鍵が PKIX 形式 (pemTypePublic) かつ秘密鍵に対応していること。
func TestRun_PublicKeyIsPKIX_MatchesPrivate(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer

	if code := run([]string{"-out", dir}, &stdout, &stderr); code != exitOK {
		t.Fatalf("run failed: %d", code)
	}

	privBytes, err := os.ReadFile(filepath.Join(dir, privateKeyFileName))
	if err != nil {
		t.Fatalf("ReadFile priv: %v", err)
	}
	pubBytes, err := os.ReadFile(filepath.Join(dir, publicKeyFileName))
	if err != nil {
		t.Fatalf("ReadFile pub: %v", err)
	}

	privBlock, _ := pem.Decode(privBytes)
	if privBlock == nil {
		t.Fatalf("priv PEM decode failed")
	}
	parsedPriv, err := x509.ParsePKCS8PrivateKey(privBlock.Bytes)
	if err != nil {
		t.Fatalf("ParsePKCS8PrivateKey: %v", err)
	}
	rsaKey := parsedPriv.(*rsa.PrivateKey)

	pubBlock, _ := pem.Decode(pubBytes)
	if pubBlock == nil {
		t.Fatalf("pub PEM decode failed")
	}
	if pubBlock.Type != pemTypePublic {
		t.Errorf("public key PEM type = %q, want %q", pubBlock.Type, pemTypePublic)
	}
	parsedPub, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	if err != nil {
		t.Fatalf("ParsePKIXPublicKey: %v", err)
	}
	rsaPub, ok := parsedPub.(*rsa.PublicKey)
	if !ok {
		t.Fatalf("public key is not RSA: %T", parsedPub)
	}
	// 秘密鍵から導出される公開鍵と一致することを確認 (N と E)。
	if rsaPub.N.Cmp(rsaKey.PublicKey.N) != 0 {
		t.Errorf("public key N does not match private key's PublicKey.N")
	}
	if rsaPub.E != rsaKey.PublicKey.E {
		t.Errorf("public key E = %d, want %d", rsaPub.E, rsaKey.PublicKey.E)
	}
}

// devkeygen 出力 → keystore.Init で正常ロード可能 (round-trip / F-7 統合)。
func TestRun_RoundTripWithKeystore(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer

	if code := run([]string{"-out", dir}, &stdout, &stderr); code != exitOK {
		t.Fatalf("devkeygen failed: %d", code)
	}

	privPath := filepath.Join(dir, privateKeyFileName)
	l := logger.New(logger.FormatJSON, &bytes.Buffer{})
	ks, src, err := keystore.Init(context.Background(),
		keystore.OIDCKeyConfig{KeyFile: privPath}, l)
	if err != nil {
		t.Fatalf("keystore.Init: %v", err)
	}
	if src != keystore.SourceFile {
		t.Errorf("Source = %v, want SourceFile", src)
	}

	pair, err := ks.Active(context.Background())
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if pair.BitLen() != keyBits {
		t.Errorf("BitLen = %d, want %d", pair.BitLen(), keyBits)
	}
	if pair.Alg != keystore.AlgRS256 {
		t.Errorf("Alg = %q, want %q", pair.Alg, keystore.AlgRS256)
	}
}

// -out 空文字 → エラー。
func TestRun_EmptyOutDirRejected(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-out", ""}, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("run = %d, want %d (empty -out should fail)", code, exitError)
	}
	if !strings.Contains(stderr.String(), "-out が空です") {
		t.Errorf("stderr lacks '-out が空です': %q", stderr.String())
	}
}

// 不明フラグ → flag.ContinueOnError 経由でエラー。
func TestRun_UnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-no-such-flag"}, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("run = %d, want %d (unknown flag should fail)", code, exitError)
	}
}

// 出力ディレクトリが既存ディレクトリを通り越して階層作成されるケース。
func TestRun_CreatesNestedDir(t *testing.T) {
	base := t.TempDir()
	nested := filepath.Join(base, "level1", "level2", "dev-keys")
	var stdout, stderr bytes.Buffer

	code := run([]string{"-out", nested}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run = %d, want %d (nested mkdir): stderr=%q", code, exitOK, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(nested, privateKeyFileName)); err != nil {
		t.Errorf("expected private key in nested dir: %v", err)
	}
}

// MkdirAll 失敗 (出力先パスが既存ファイルとして存在する) → エラー。
func TestRun_MkdirAllFails(t *testing.T) {
	base := t.TempDir()
	// base/regular_file に通常ファイルを作成し、それを -out で指定する。
	// MkdirAll は同名ファイルが存在する場合 ENOTDIR でエラーになる。
	regular := filepath.Join(base, "regular_file")
	if err := os.WriteFile(regular, []byte("not a dir"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// regular の下にさらに dev-keys を作ろうとすると ENOTDIR
	target := filepath.Join(regular, "dev-keys")

	var stdout, stderr bytes.Buffer
	code := run([]string{"-out", target}, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("run = %d, want %d (mkdir under file should fail)", code, exitError)
	}
	if !strings.Contains(stderr.String(), "出力ディレクトリの作成に失敗") {
		t.Errorf("stderr lacks '出力ディレクトリの作成に失敗': %q", stderr.String())
	}
}
