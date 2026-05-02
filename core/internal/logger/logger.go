// Package logger は id-core 用の構造化ログ出力 API を提供する。
//
// 設計判断 (D2): slog.Logger を直接公開せず、薄い独自インターフェースで包む。
// これにより context から request_id / event_id を自動付与し (Step 3 で実装)、
// Domain 層からロガーを直接呼ばない規約 (F-14) を境界 API レベルで自然化する。
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"
)

// Logger は本プロジェクトのログ出力 API。
//
// 内部実装は slog.Handler。
type Logger struct {
	handler slog.Handler
}

// New は format / writer を指定して Logger を生成する。
//
// w は内部で FallbackWriter にラップされ、書き込み失敗時に stderr へフォールバックする
// (Q9 仕様)。テストで bytes.Buffer を渡す場合は失敗経路に入らないため挙動は同一。
func New(format Format, w io.Writer) *Logger {
	return &Logger{handler: newHandler(format, NewFallbackWriter(w))}
}

// Default は CORE_LOG_FORMAT 環境変数を読み、stdout に出力する production 用 Logger を返す。
//
// CORE_LOG_FORMAT が不正値の場合はエラーを返す (cmd/main の起動失敗として扱う)。
func Default() (*Logger, error) {
	f, err := FormatFromEnv()
	if err != nil {
		return nil, err
	}
	return New(f, os.Stdout), nil
}

// Info は INFO レベルでログを出力する (業務イベント)。
func (l *Logger) Info(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelInfo, msg, nil, args...)
}

// Warn は WARN レベルでログを出力する (4xx 等)。
func (l *Logger) Warn(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelWarn, msg, nil, args...)
}

// Error は ERROR レベルでログを出力する (5xx / panic / 予期しないエラー)。
//
// err は別引数で受け、構造化フィールド "error" として付与する (string 連結で
// 改行・制御文字を混入させないため、F-1 のログインジェクション対策)。
func (l *Logger) Error(ctx context.Context, msg string, err error, args ...any) {
	l.log(ctx, slog.LevelError, msg, err, args...)
}

// Debug は DEBUG レベルでログを出力する (本番無効・開発調査用)。
func (l *Logger) Debug(ctx context.Context, msg string, args ...any) {
	l.log(ctx, slog.LevelDebug, msg, nil, args...)
}

func (l *Logger) log(ctx context.Context, level slog.Level, msg string, errVal error, args ...any) {
	if ctx == nil {
		ctx = context.Background()
	}
	if !l.handler.Enabled(ctx, level) {
		return
	}
	r := slog.NewRecord(time.Now(), level, msg, 0)

	// F-3 / F-4: context から request_id / event_id を取得して付与。
	// HTTP 経路 (middleware が WithRequestID 済み) -> request_id 付与。
	// 非 HTTP 経路 (起動・ジョブ等が WithEventID 済み) -> event_id 付与。
	// どちらも未設定の場合は付与しない (誤って空文字を入れない)。
	if id := RequestIDFrom(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	if id := EventIDFrom(ctx); id != "" {
		r.AddAttrs(slog.String("event_id", id))
	}

	r.Add(args...)
	if errVal != nil {
		r.AddAttrs(slog.String("error", errVal.Error()))
	}
	// Handle のエラーは Step 5 (fallback.go) の FallbackWriter で吸収する。
	// ここで返されるのは writer 側のエラー (stdout 書き込み失敗等) で、
	// FallbackWriter が stderr フォールバック + drop counter で記録するため、
	// Logger.log としてはエラーを呼び出し元に伝播せず継続する (リクエスト処理を止めない)。
	_ = l.handler.Handle(ctx, r)
}
