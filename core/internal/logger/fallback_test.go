package logger

// FallbackWriter は内部 writer の挙動をテストする必要があるため、
// 内部テストパッケージ (package logger) に置き、stderr 注入経路を直接検証する。

import (
	"bytes"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

// failingWriter は任意のエラーを返すモック writer。calls は呼び出し回数 (atomic)。
type failingWriter struct {
	err   error
	calls atomic.Int64
}

func (f *failingWriter) Write(p []byte) (int, error) {
	f.calls.Add(1)
	return 0, f.err
}

// T-19: primary が失敗した場合、stderr (fallback) に書き込み、Write 呼び出し元には
// エラーを伝播しない (リクエスト処理は継続)。
func TestFallbackWriter_PrimaryFails_FallsBackToStderr(t *testing.T) {
	primary := &failingWriter{err: errors.New("disk full")}
	var stderrBuf bytes.Buffer

	fw := &FallbackWriter{primary: primary, fallback: &stderrBuf}

	n, err := fw.Write([]byte(`{"msg":"hello"}` + "\n"))
	if err != nil {
		t.Errorf("Write should not propagate error to caller, got: %v", err)
	}
	if n == 0 {
		t.Errorf("Write should report bytes written (>0), got %d", n)
	}
	if stderrBuf.Len() == 0 {
		t.Errorf("fallback (stderr) should have received the bytes, got empty")
	}
	if got := fw.DropCount(); got != 0 {
		t.Errorf("DropCount = %d, want 0 (fallback succeeded)", got)
	}
}

// T-20: primary / fallback 両方失敗時に DropCount が増分し、Write はエラーを返さない。
func TestFallbackWriter_BothFail_IncrementDropCount(t *testing.T) {
	primary := &failingWriter{err: errors.New("p down")}
	fallback := &failingWriter{err: errors.New("f down")}

	fw := &FallbackWriter{primary: primary, fallback: fallback}

	for i := 0; i < 3; i++ {
		if _, err := fw.Write([]byte("x")); err != nil {
			t.Errorf("Write should not propagate error even when both fail, got: %v", err)
		}
	}

	if got := fw.DropCount(); got != 3 {
		t.Errorf("DropCount after 3 failures = %d, want 3", got)
	}
}

// T-21: primary が正常な場合、fallback は呼ばれず DropCount は 0。
func TestFallbackWriter_PrimaryOK_NoDropAndNoFallback(t *testing.T) {
	var primary bytes.Buffer
	fallback := &failingWriter{err: errors.New("should-not-be-called")}

	fw := &FallbackWriter{primary: &primary, fallback: fallback}

	for i := 0; i < 5; i++ {
		if _, err := fw.Write([]byte("ok")); err != nil {
			t.Errorf("Write error: %v", err)
		}
	}

	if got := fw.DropCount(); got != 0 {
		t.Errorf("DropCount = %d, want 0 (primary succeeded)", got)
	}
	if c := fallback.calls.Load(); c != 0 {
		t.Errorf("fallback should not be called when primary succeeds, got %d calls", c)
	}
}

// 補助テスト: NewFallbackWriter は os.Stderr を fallback として設定する。
func TestNewFallbackWriter_DefaultsToStderr(t *testing.T) {
	var primary bytes.Buffer
	fw := NewFallbackWriter(&primary)

	if fw.primary != &primary {
		t.Errorf("primary not wired correctly")
	}
	if fw.fallback != os.Stderr {
		t.Errorf("fallback should be os.Stderr, got %T %v", fw.fallback, fw.fallback)
	}
}

// partialWriter は最初に halfBytes だけ書いて error を返すモック writer (io.Writer 契約)。
type partialWriter struct {
	halfBytes int
	err       error
}

func (p *partialWriter) Write(buf []byte) (int, error) {
	n := p.halfBytes
	if n > len(buf) {
		n = len(buf)
	}
	return n, p.err
}

// 補助テスト: primary が部分書き込み + error を返した場合、fallback は残りバイトのみ
// 書き込み、Write の戻り値は primary + fallback の合計バイト数になる。
func TestFallbackWriter_PartialPrimaryWrite_NoDuplication(t *testing.T) {
	primary := &partialWriter{halfBytes: 4, err: errors.New("disk-cut-off")}
	var fallback bytes.Buffer

	fw := &FallbackWriter{primary: primary, fallback: &fallback}

	payload := []byte("0123456789") // 10 bytes
	n, err := fw.Write(payload)
	if err != nil {
		t.Fatalf("Write should not propagate error, got: %v", err)
	}
	if n != len(payload) {
		t.Errorf("Write n = %d, want %d (primary 4 + fallback 6)", n, len(payload))
	}
	if got := fallback.String(); got != "456789" {
		t.Errorf("fallback should receive only the remaining bytes, got: %q", got)
	}
	if got := fw.DropCount(); got != 0 {
		t.Errorf("DropCount = %d, want 0 (fallback completed remainder)", got)
	}
}

// 補助テスト: 並行書き込み下でも DropCount が atomic に増分される。
func TestFallbackWriter_ConcurrentDrops(t *testing.T) {
	primary := &failingWriter{err: errors.New("p")}
	fallback := &failingWriter{err: errors.New("f")}
	fw := &FallbackWriter{primary: primary, fallback: fallback}

	const writers = 10
	const iterations = 100
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _ = fw.Write([]byte("x"))
			}
		}()
	}
	wg.Wait()

	if got, want := fw.DropCount(), int64(writers*iterations); got != want {
		t.Errorf("DropCount = %d, want %d (atomic miscount?)", got, want)
	}
}
