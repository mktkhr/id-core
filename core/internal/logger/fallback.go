package logger

import (
	"io"
	"os"
	"sync/atomic"
)

// FallbackWriter は primary writer の書き込み失敗時に fallback writer (デフォルト stderr) へ
// フォールバックする io.Writer。fallback も失敗した場合は drop counter を増分する (Q9)。
//
// Write は呼び出し元にエラーを返さない: ログ出力失敗でリクエスト処理を止めない方針。
// drop counter は内部保持し、外部公開は M1.x のメトリクス連携で行う (DropCount で取得可)。
type FallbackWriter struct {
	primary  io.Writer
	fallback io.Writer
	drops    atomic.Int64
}

// NewFallbackWriter は primary を wrap し、fallback として os.Stderr を設定した
// FallbackWriter を返す。
func NewFallbackWriter(primary io.Writer) *FallbackWriter {
	return &FallbackWriter{
		primary:  primary,
		fallback: os.Stderr,
	}
}

// Write は primary -> fallback の順に書き込みを試みる。
//
// 戻り値は常に nil error: ログ出力の失敗を呼び出し元 (Logger.log -> handler.Handle) に
// 伝播させない。
//
// io.Writer 契約への配慮:
//   - primary が部分書き込み + error (n > 0 && err != nil) を返した場合は、
//     fallback には残り p[n:] のみを書き込み、二重出力を防ぐ。
//   - fallback も同様に部分書き込みを許容する (n が len(残り) と異なってもエラー扱い)。
//
// 戻り値:
//   - primary が完了したら primary の n を返す。
//   - 部分書き込みを fallback で補完できたら (primary n) + (fallback n) を返す。
//   - 両方失敗した場合は len(p) を返して drop counter を 1 増分する
//     (バイト数は呼び出し側のループ条件を壊さないため正の値で返す)。
func (w *FallbackWriter) Write(p []byte) (int, error) {
	pn, perr := w.primary.Write(p)
	if perr == nil {
		return pn, nil
	}

	// 部分書き込み: primary が pn バイト書いた後にエラーになった場合は、
	// 残り p[pn:] のみを fallback に渡す (重複出力防止)。
	// pn の範囲は不正実装に備えて [0, len(p)] にクランプする (slice panic 回避)。
	if pn < 0 {
		pn = 0
	}
	if pn > len(p) {
		pn = len(p)
	}
	remaining := p[pn:]
	fn, ferr := w.fallback.Write(remaining)
	if ferr == nil && fn == len(remaining) {
		return pn + fn, nil
	}

	w.drops.Add(1)
	return len(p), nil
}

// DropCount は primary / fallback 両方失敗してドロップしたレコード数を返す (atomic 取得)。
// M1.x のメトリクスエンドポイントから参照する想定。
func (w *FallbackWriter) DropCount() int64 {
	return w.drops.Load()
}
