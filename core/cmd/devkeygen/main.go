// Command devkeygen は dev / staging 環境向けの RSA 2048 bit 署名鍵ペアを生成する CLI。
//
// 設計 #32 P1_01 ステップ 3。`make -C core dev-keygen` から呼ばれ、
// 出力先 `./dev-keys/` (既定) に PKCS#8 PEM (秘密鍵) + PKIX PEM (公開鍵) を書き出す。
//
// 鍵フォーマット (F-13):
//   - 秘密鍵: PEM タイプ "PRIVATE KEY" (PKCS#8)、permission 0600
//   - 公開鍵: PEM タイプ "PUBLIC KEY" (PKIX)、permission 0644
//
// 鍵長は 2048 bit 固定 (M1.1 範囲、`-bits` フラグなし。論点 #5)。
// 出力ディレクトリは事前に存在しなくても自動作成 (0700)。
//
// 既存ファイル保護 (F-14): 出力先に同名ファイルが既に存在する場合は `-force` 指定がない限りエラー。
// dev 鍵の上書き事故 + リポジトリへの誤コミット (`.gitignore` 必須) 抑止。
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	defaultOutDir = "./dev-keys"

	keyBits = 2048

	privateKeyFileName = "signing.pem"
	publicKeyFileName  = "signing.pub.pem"

	privateKeyMode os.FileMode = 0o600
	publicKeyMode  os.FileMode = 0o644
	outDirMode     os.FileMode = 0o700

	pemTypePrivate = "PRIVATE KEY"
	pemTypePublic  = "PUBLIC KEY"
)

const (
	exitOK    = 0
	exitError = 1
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run は main 本体を testable に切り出した関数。
//
// 引数:
//   - args     : os.Args[1:] 相当のフラグ配列
//   - stdout   : 進捗メッセージ出力先 (テストでは bytes.Buffer)
//   - stderr   : エラーメッセージ出力先
//
// 戻り値: 終了コード (0 = 成功 / 1 = 失敗)。
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("devkeygen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	outDir := fs.String("out", defaultOutDir, "出力ディレクトリ (signing.pem + signing.pub.pem を配置)")
	force := fs.Bool("force", false, "既存ファイルを上書きする (デフォルトは保護のためエラー)")
	if err := fs.Parse(args); err != nil {
		// flag.ContinueOnError では Parse がメッセージ出力済み。終了コードのみ返す。
		return exitError
	}

	if err := generate(*outDir, *force, stdout); err != nil {
		fmt.Fprintf(stderr, "devkeygen: %v\n", err)
		return exitError
	}
	return exitOK
}

// generate は実際の鍵生成 + ファイル書き出しを行う。
//
// 失敗パターン:
//   - 出力ディレクトリ作成不能 (権限なし等)
//   - 既存ファイルあり + force=false
//   - rsa.GenerateKey 失敗 (rand.Reader が壊れた等の異常事態)
//   - PKCS#8 / PKIX エンコード失敗
//   - WriteFile 失敗
func generate(outDir string, force bool, stdout io.Writer) error {
	if outDir == "" {
		return errors.New("-out が空です (出力ディレクトリを指定してください)")
	}
	if err := os.MkdirAll(outDir, outDirMode); err != nil {
		return fmt.Errorf("出力ディレクトリの作成に失敗しました: %s: %w", outDir, err)
	}

	privPath := filepath.Join(outDir, privateKeyFileName)
	pubPath := filepath.Join(outDir, publicKeyFileName)

	if !force {
		for _, p := range []string{privPath, pubPath} {
			if _, err := os.Stat(p); err == nil {
				return fmt.Errorf("既存ファイルが存在します: %s (上書きする場合は -force を指定してください)", p)
			} else if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("既存ファイル確認に失敗しました: %s: %w", p, err)
			}
		}
	}

	key, err := rsa.GenerateKey(rand.Reader, keyBits)
	if err != nil {
		return fmt.Errorf("RSA %d bit 鍵の生成に失敗しました: %w", keyBits, err)
	}

	privDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return fmt.Errorf("秘密鍵の PKCS#8 エンコードに失敗しました: %w", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return fmt.Errorf("公開鍵の PKIX エンコードに失敗しました: %w", err)
	}

	privPEM := pem.EncodeToMemory(&pem.Block{Type: pemTypePrivate, Bytes: privDER})
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: pemTypePublic, Bytes: pubDER})

	if err := writeFileMode(privPath, privPEM, privateKeyMode); err != nil {
		return fmt.Errorf("秘密鍵ファイルの書き出しに失敗しました: %s: %w", privPath, err)
	}
	if err := writeFileMode(pubPath, pubPEM, publicKeyMode); err != nil {
		return fmt.Errorf("公開鍵ファイルの書き出しに失敗しました: %s: %w", pubPath, err)
	}

	fmt.Fprintf(stdout, "dev 鍵を生成しました: %s (0600) / %s (0644)\n", privPath, pubPath)
	return nil
}

// writeFileMode は os.WriteFile のラッパー。既存ファイルがあれば mode を再設定する
// (umask の影響で WriteFile 単体では mode が落ちることがあるため Chmod も呼ぶ)。
func writeFileMode(path string, data []byte, mode os.FileMode) error {
	if err := os.WriteFile(path, data, mode); err != nil {
		return err
	}
	// umask 経由で意図より緩い mode が適用される可能性に備え、明示的に Chmod する。
	return os.Chmod(path, mode)
}
