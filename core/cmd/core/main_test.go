package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
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

