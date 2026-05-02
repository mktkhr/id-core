package middleware

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mktkhr/id-core/core/internal/logger"
)

// AccessLog middleware は終了時に 1 行のアクセスログを構造化出力する (D3)。
//
// フィールド: time / level / msg=access / request_id / method / path / status / duration_ms
// (F-3 / F-11 / Q7)
//   - 5xx       → ERROR
//   - 4xx       → WARN
//   - それ以外 → INFO
//
// path は URL のパス + (redact 済み) クエリ文字列。redact 対象キー (Q8) の値は
// [REDACTED] に置換される (F-13)。
//
// 不正だったクライアント X-Request-Id (request_id middleware が context に残した
// client_request_id) があれば追加フィールドとして記録する。
//
// l == nil の場合は契約違反として panic する (シングルポイント設計の前提を統一)。
func AccessLog(l *logger.Logger, next http.Handler) http.Handler {
	if l == nil {
		panic("middleware.AccessLog: logger must not be nil")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		// defer で終了時のみ出力 (D3)。
		defer func() {
			elapsed := time.Since(start)
			level := levelFromStatus(rw.status)

			attrs := []any{
				slog.String("msg_kind", "access"),
				slog.String("method", r.Method),
				slog.String("path", redactedPath(r.URL)),
				slog.Int("status", rw.status),
				slog.Float64("duration_ms", float64(elapsed.Microseconds())/1000.0),
			}
			if c := ClientRequestIDFrom(r.Context()); c != "" {
				attrs = append(attrs, slog.String("client_request_id", c))
			}

			switch level {
			case slog.LevelError:
				l.Error(r.Context(), "access", nil, attrs...)
			case slog.LevelWarn:
				l.Warn(r.Context(), "access", attrs...)
			default:
				l.Info(r.Context(), "access", attrs...)
			}
		}()

		next.ServeHTTP(rw, r)
	})
}

// statusRecorder は http.ResponseWriter をラップし WriteHeader の status を捕捉する。
// WriteHeader を呼ばずに Write された場合 (暗黙 200) は status のデフォルト値を維持する。
//
// http.Hijacker / http.Flusher は元 ResponseWriter が実装している場合に透過する
// (WebSocket / Server-Sent Events 等の互換性維持)。Pusher (HTTP/2 push) は
// 利用機会が極めて限定的で本プロジェクトの想定外のため透過しない。
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(p []byte) (int, error) {
	if !r.wroteHeader {
		// 暗黙 200 を観測。Header を 200 で fix する。
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(p)
}

// Flush は元 ResponseWriter が http.Flusher の場合のみ透過する (Server-Sent Events 等)。
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack は元 ResponseWriter が http.Hijacker の場合のみ透過する (WebSocket 等)。
// Hijack 後の status は捕捉できないため warning ログ等の追加対応は呼び出し側で行う。
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.ErrUnsupported
}

func levelFromStatus(status int) slog.Level {
	switch {
	case status >= 500:
		return slog.LevelError
	case status >= 400:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

// redactedPath は URL のパス部分と (redact 済み) クエリ文字列を組み立てて返す。
//
// クエリ文字列内の deny-list キー (例: code / access_token) の値は [REDACTED] に
// 置換する (判定は logger.IsFieldKeyToRedact で一元管理、本パッケージでは再定義しない)。
//
// 受信時の `&` 区切りと出現順序を保持するため、url.Values.Encode() で再エンコード
// するのではなく文字列レベルで分割・置換する。フラグメントは HTTP リクエストには
// 通常含まれないため無視。
func redactedPath(u *url.URL) string {
	if u == nil {
		return ""
	}
	path := u.EscapedPath()
	if u.RawQuery == "" {
		return path
	}
	return path + "?" + redactQueryString(u.RawQuery)
}

func redactQueryString(raw string) string {
	pairs := strings.Split(raw, "&")
	for i, p := range pairs {
		eq := strings.IndexByte(p, '=')
		var key string
		hasEq := eq >= 0
		if hasEq {
			key = p[:eq]
		} else {
			key = p
		}
		// key は URL エンコードされている可能性がある。デコードできなければ生 key で判定。
		decoded, err := url.QueryUnescape(key)
		if err != nil {
			decoded = key
		}
		if logger.IsFieldKeyToRedact(decoded) {
			if hasEq {
				pairs[i] = key + "=" + logger.RedactedValue
			} else {
				// 元が key-only (例: "code") のときは表現を維持して
				// "code" のまま出力する (値そのものが空なので redact 不要)。
				pairs[i] = key
			}
		}
	}
	return strings.Join(pairs, "&")
}
