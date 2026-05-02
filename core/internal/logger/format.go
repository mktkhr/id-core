package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"
)

// Format はログ出力フォーマット種別。
type Format int

const (
	// FormatJSON は JSON Lines 形式 (本番想定)。
	FormatJSON Format = iota
	// FormatText は key=value 形式 (開発時用)。
	FormatText
)

const envLogFormat = "CORE_LOG_FORMAT"

// FormatFromEnv は CORE_LOG_FORMAT 環境変数からフォーマットを決定する。
//
// 値が未設定または "json" のとき FormatJSON、"text" のとき FormatText を返す。
// それ以外はエラー。
func FormatFromEnv() (Format, error) {
	v := os.Getenv(envLogFormat)
	switch v {
	case "", "json":
		return FormatJSON, nil
	case "text":
		return FormatText, nil
	default:
		return 0, fmt.Errorf("logger: 不正な %s の値: %q (json|text のみ受け付ける)", envLogFormat, v)
	}
}

// utcTimeReplaceAttr は slog.HandlerOptions.ReplaceAttr フックで time フィールドを
// UTC + RFC3339Nano に変換する。プロセス全体の time.Local を変更する副作用を避ける目的。
func utcTimeReplaceAttr(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey && a.Value.Kind() == slog.KindTime {
		a.Value = slog.StringValue(a.Value.Time().UTC().Format(time.RFC3339Nano))
	}
	return a
}

// newHandler は format に応じた slog.Handler を生成する。
// Level は LevelDebug (使い分けは Q7 に従い、本番では DEBUG を出さない運用とする)。
func newHandler(format Format, w io.Writer) slog.Handler {
	opts := &slog.HandlerOptions{
		Level:       slog.LevelDebug,
		ReplaceAttr: utcTimeReplaceAttr,
	}
	switch format {
	case FormatText:
		return slog.NewTextHandler(w, opts)
	default:
		return slog.NewJSONHandler(w, opts)
	}
}
