package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/mktkhr/id-core/core/internal/config"
	"github.com/mktkhr/id-core/core/internal/logger"
)

// T-63: core/ 配下の production コードから log.Fatal* が完全排除されている。
//
// 単体テストで lint 相当のガードを組み込む。検出条件は `log.Fatal` の呼び出し
// (= 開き括弧付き、関数名末尾は任意の英字列) のみで、コメントや「使わない」と
// 書いてある docstring は誤検知しない。テストコード (*_test.go) は対象外。
//
// 外部コマンド (grep) には依存せず、Go 標準ライブラリで walk する。
func TestNoLogFatal_InCorePackage(t *testing.T) {
	root := repoRoot(t)
	target := filepath.Join(root, "core")

	pattern := regexp.MustCompile(`log\.Fatal[A-Za-z]*\(`)
	var hits []string

	err := filepath.WalkDir(target, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(body), "\n") {
			if pattern.MatchString(line) {
				hits = append(hits, fmt.Sprintf("%s:%d: %s", path, i+1, line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(hits) > 0 {
		t.Fatalf("log.Fatal の呼び出しが core/ 配下に残存:\n%s", strings.Join(hits, "\n"))
	}
}

// T-63 補助: 標準 log パッケージの import が cmd/core/main.go から消えている。
func TestNoStdLogImport_InMain(t *testing.T) {
	root := repoRoot(t)
	mainPath := filepath.Join(root, "core", "cmd", "core", "main.go")

	body, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	for _, forbidden := range []string{
		`"log"` + "\n",
		`	"log"` + "\n",
	} {
		if strings.Contains(string(body), forbidden) {
			t.Errorf("main.go に標準 log の import が残存: %q", forbidden)
		}
	}
}

// T-64: 設定読み込み失敗時の挙動 — log.Fatal を使わず exitError (=1) を返す。
//
// 不正な CORE_PORT を環境変数に設定して config.Load を失敗させ、run() が
// 1 を返すことを直接検証する。stderr フォールバック出力もキャプチャ。
func TestRun_ConfigLoadError_ReturnsExitError(t *testing.T) {
	t.Setenv("CORE_PORT", "not-a-number")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	exitCode := run(w)
	_ = w.Close()

	if exitCode != exitError {
		t.Errorf("run() = %d, want %d (exitError)", exitCode, exitError)
	}

	var stderrBuf bytes.Buffer
	_, _ = stderrBuf.ReadFrom(r)
	if !strings.Contains(stderrBuf.String(), "設定の読み込みに失敗") {
		t.Errorf("stderr should contain bootstrap error, got: %q", stderrBuf.String())
	}
}

// T-100: DB 接続失敗 → run() が exitError を返す (F-6 起動時失敗の logger.Error + 非ゼロ exit)。
//
// 接続不能なホスト/ポートを設定し、db.Open が失敗することを引き金に exit code を assert する。
// 構造化ログは stdout に出る (run 内 logger.Default()) ので stderr には影響しない。
func TestRun_DBOpenFailure_ReturnsExitError(t *testing.T) {
	t.Setenv("CORE_PORT", "")
	t.Setenv("CORE_LOG_FORMAT", "json")
	t.Setenv("CORE_DB_HOST", "127.0.0.1")
	t.Setenv("CORE_DB_PORT", "1") // unreachable
	t.Setenv("CORE_DB_USER", "u")
	t.Setenv("CORE_DB_PASSWORD", "p")
	t.Setenv("CORE_DB_NAME", "d")
	t.Setenv("CORE_DB_SSLMODE", "disable")
	// M1.1 (#32) で必須化された CORE_ENV / CORE_OIDC_* を設定し、config.Load を通す。
	// 本テストは db.Open 失敗を観測対象にしているため OIDC 検証は通過する必要がある。
	t.Setenv("CORE_ENV", "dev")
	t.Setenv("CORE_OIDC_ISSUER", "http://localhost:8080")
	t.Setenv("CORE_OIDC_DEV_GENERATE_KEY", "1")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	exitCode := run(w)
	_ = w.Close()

	if exitCode != exitError {
		t.Errorf("run() = %d, want %d (DB unreachable のため exitError)", exitCode, exitError)
	}
}

// T-65: bootstrap が成功する場合に event_id (UUID v7) 形式を返す。
//
// run() を呼ぶと ListenAndServe で長時間ブロックするため、bootstrap だけ直接呼んで検証する。
func TestBootstrap_Success_EventIDIsUUIDv7(t *testing.T) {
	t.Setenv("CORE_PORT", "")           // デフォルト値で起動
	t.Setenv("CORE_LOG_FORMAT", "json") // 明示的に json
	// M0.3 で CORE_DB_* が必須化されたため、bootstrap では config.Load 段階で値が必要。
	// 接続自体は本テスト範囲外 (P3 で起動シーケンスに統合)。
	t.Setenv("CORE_DB_HOST", "localhost")
	t.Setenv("CORE_DB_PORT", "5432")
	t.Setenv("CORE_DB_USER", "idcore")
	t.Setenv("CORE_DB_PASSWORD", "idcore")
	t.Setenv("CORE_DB_NAME", "idcore")
	// M1.1 (#32): CORE_ENV / CORE_OIDC_* も必須。dev + 起動時鍵生成モードで bootstrap を通す。
	t.Setenv("CORE_ENV", "dev")
	t.Setenv("CORE_OIDC_ISSUER", "http://localhost:8080")
	t.Setenv("CORE_OIDC_DEV_GENERATE_KEY", "1")

	cfg, l, eventID, err := bootstrap()
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if cfg == nil {
		t.Errorf("cfg should not be nil")
	}
	if l == nil {
		t.Errorf("logger should not be nil")
	}
	parsed, perr := uuid.Parse(eventID)
	if perr != nil {
		t.Fatalf("event_id is not a valid UUID: %q (%v)", eventID, perr)
	}
	if parsed.Version() != 7 {
		t.Errorf("event_id UUID version = %d, want 7", parsed.Version())
	}
}

// T-65 補助: 起動 INFO ログの実出力 JSON を検証する。
//
// emitStartupLog に buffer-backed logger を注入し、出力された JSON Lines に
// 日本語 msg / addr / event_id が含まれることを直接 assert する。
func TestEmitStartupLog_HasEventIDAndAddrAndJapaneseMsg(t *testing.T) {
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)

	const eventID = "01911f4e-7234-7b2a-8000-000000000001"
	const addr = ":8765"

	ctx := logger.WithEventID(context.Background(), eventID)
	emitStartupLog(l, ctx, addr)

	out := strings.TrimSpace(buf.String())
	if out == "" {
		t.Fatalf("emitStartupLog emitted nothing")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("Unmarshal: %v (out=%q)", err, out)
	}

	if m["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", m["level"])
	}
	if m["msg"] != "core サーバーを起動します" {
		t.Errorf("msg = %v, want '日本語起動メッセージ'", m["msg"])
	}
	if m["addr"] != addr {
		t.Errorf("addr = %v, want %v", m["addr"], addr)
	}
	if m["event_id"] != eventID {
		t.Errorf("event_id = %v, want %v", m["event_id"], eventID)
	}
}

// repoRoot はテスト実行ファイルの位置からリポジトリルートを推定する。
//
// runtime.Caller(0) で現テストファイルのパスを取り、core/cmd/core/ から 3 階層上を返す。
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	// file = .../core/cmd/core/main_test.go → 3 階層上が repo root
	return filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(file))))
}


// ===== M1.1 (#32) keystore 統合テスト =====

// CORE_ENV=prod + 鍵 env 未設定 → run() が exit 1 (config.Load 段階で起動失敗、F-9 / F-20-c)。
//
// 注: 統合テストではなくユニットテスト相当 (config.Load 失敗時点で run が抜けるため DB 不要)。
func TestRun_ProdWithoutKeyEnv_ReturnsExitError(t *testing.T) {
	t.Setenv("CORE_PORT", "")
	t.Setenv("CORE_LOG_FORMAT", "json")
	t.Setenv("CORE_ENV", "prod")
	t.Setenv("CORE_OIDC_ISSUER", "https://id.example.com")
	t.Setenv("CORE_OIDC_KEY_FILE", "")
	t.Setenv("CORE_OIDC_DEV_GENERATE_KEY", "")
	t.Setenv("CORE_DB_HOST", "localhost")
	t.Setenv("CORE_DB_PORT", "5432")
	t.Setenv("CORE_DB_USER", "u")
	t.Setenv("CORE_DB_PASSWORD", "p")
	t.Setenv("CORE_DB_NAME", "d")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	exitCode := run(w)
	_ = w.Close()

	if exitCode != exitError {
		t.Errorf("run() = %d, want %d (prod + 鍵未設定で起動失敗)", exitCode, exitError)
	}
	var stderrBuf bytes.Buffer
	_, _ = stderrBuf.ReadFrom(r)
	if !strings.Contains(stderrBuf.String(), "設定の読み込みに失敗") {
		t.Errorf("stderr should contain config load failure, got: %q", stderrBuf.String())
	}
}

// emitKeystoreStartupLogs の出力スキーマを直接検証 (DB 不要、ロガー単離)。
func TestEmitKeystoreStartupLogs_SchemaAndConditions(t *testing.T) {
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)
	ctx := logger.WithEventID(context.Background(), "test-event-id")

	ks, src, err := initKeystore(ctx, &config.OIDCConfig{DevGenerateKey: true}, l)
	if err != nil {
		t.Fatalf("initKeystore: %v", err)
	}
	if err := emitKeystoreStartupLogs(ctx, l, ks, src, config.EnvDev); err != nil {
		t.Fatalf("emitKeystoreStartupLogs: %v", err)
	}

	// 起動鍵情報 INFO ログを探す
	var found map[string]any
	for _, line := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if jerr := json.Unmarshal([]byte(line), &m); jerr != nil {
			continue
		}
		if m["msg"] == "起動鍵情報" {
			found = m
			break
		}
	}
	if found == nil {
		t.Fatalf("起動鍵情報 INFO ログが出力されていない: %q", buf.String())
	}

	// 必須フィールドの型と値
	for _, key := range []string{"kid", "alg", "source", "env", "event_id"} {
		v, ok := found[key].(string)
		if !ok {
			t.Errorf("起動鍵情報 missing/wrong type field %q: got %v", key, found[key])
			continue
		}
		if v == "" {
			t.Errorf("起動鍵情報 field %q is empty", key)
		}
	}
	if found["alg"] != "RS256" {
		t.Errorf("alg = %v, want RS256", found["alg"])
	}
	if found["source"] != "generated" {
		t.Errorf("source = %v, want generated", found["source"])
	}
	if found["env"] != "dev" {
		t.Errorf("env = %v, want dev", found["env"])
	}

	// kid は 24 hex 文字 (F-11)
	if kid, _ := found["kid"].(string); len(kid) != 24 {
		t.Errorf("kid length = %d, want 24 (F-11)", len(kid))
	}

	// dev 鍵生成モードの WARN
	if !strings.Contains(buf.String(), "dev 鍵生成モード") {
		t.Errorf("WARN dev 鍵生成モード が出力されていない: %q", buf.String())
	}

	// F-18 redact 確認: 秘密鍵 / PEM / RSA modulus(n) / exponent(e) 値が出ていない
	for _, leak := range []string{"BEGIN PRIVATE", "BEGIN RSA", `"n":`, `"e":`, `"d":`} {
		if strings.Contains(buf.String(), leak) {
			t.Errorf("F-18 redact 違反: ログに %q が含まれている: %q", leak, buf.String())
		}
	}
}

// 短い鍵 (1024 bit) で WARN ログが出ること (論点 #16)。
func TestEmitKeystoreStartupLogs_ShortKey_EmitsWarn(t *testing.T) {
	// keystore.Init は SourceGenerated だと 2048 bit 固定なので、
	// 直接 buildKeyPair 相当の状態を作るには... → keystore がそういう内部 helper を export していないため、
	// ファイルモードで 1024 bit PEM を渡してテストする。
	rsaKey1024PEM := generateTestPEM1024(t)
	tempPath := writeTempPEM(t, rsaKey1024PEM)

	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)
	ctx := logger.WithEventID(context.Background(), "test-event-id")

	ks, src, err := initKeystore(ctx, &config.OIDCConfig{KeyFile: tempPath}, l)
	if err != nil {
		t.Fatalf("initKeystore: %v", err)
	}
	if err := emitKeystoreStartupLogs(ctx, l, ks, src, config.EnvDev); err != nil {
		t.Fatalf("emitKeystoreStartupLogs: %v", err)
	}

	if !strings.Contains(buf.String(), "1024 bit") {
		t.Errorf("WARN 鍵長 1024 bit が出力されていない: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "2048 bit 以上を推奨") {
		t.Errorf("WARN 推奨メッセージが含まれていない: %q", buf.String())
	}
}

// generateTestPEM1024 は 1024 bit RSA を生成して PKCS#8 PEM バイト列を返す。
func generateTestPEM1024(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

func writeTempPEM(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.pem")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}
