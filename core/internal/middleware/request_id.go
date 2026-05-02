// Package middleware は HTTP middleware を提供する。
//
// 構成 (D1 順序、外側から):
//
//	request_id  → access_log → recover → handler
//
// この順序により、panic 含む全てのログレコードに request_id が付き、access_log は
// recover が変換した最終 status を観測できる。
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/mktkhr/id-core/core/internal/logger"
)

// 仕様 F-6 の妥当性基準値。
const (
	headerXRequestID = "X-Request-Id"

	// maxRequestIDLength はクライアント提供 X-Request-Id の最大オクテット長 (F-6 (a))。
	maxRequestIDLength = 128
)

// ctxKey は context.Context のキーとして使う非公開型。
type ctxKey struct{ name string }

var clientRequestIDKey = ctxKey{"client_request_id"}

// RequestID middleware は X-Request-Id を生成・検証・context 注入する。
//
// 動作:
//  1. クライアント X-Request-Id が F-6 妥当性基準を満たせばそのまま採用。
//  2. 不正なら破棄して UUID v7 を新規生成し、サニタイズ済み元値を context に
//     client_request_id として残す (access_log middleware が拾ってログに記録)。
//  3. レスポンスヘッダ X-Request-Id は **next.ServeHTTP を呼ぶ前**に設定する
//     (handler が WriteHeader 後に追加するヘッダは HTTP レスポンスに反映されないため)。
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client := r.Header.Get(headerXRequestID)

		var id string
		var clientForLog string
		if isValidRequestID(client) {
			id = client
		} else {
			id = newRequestID()
			if client != "" {
				clientForLog = sanitizeClientRequestID(client)
			}
		}

		ctx := logger.WithRequestID(r.Context(), id)
		if clientForLog != "" {
			ctx = withClientRequestID(ctx, clientForLog)
		}
		w.Header().Set(headerXRequestID, id)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ClientRequestIDFrom は不正だったクライアント X-Request-Id (サニタイズ済み) を ctx から取得する。
// 妥当だった場合や未送信の場合は空文字。
func ClientRequestIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(clientRequestIDKey).(string); ok {
		return v
	}
	return ""
}

func withClientRequestID(ctx context.Context, v string) context.Context {
	return context.WithValue(ctx, clientRequestIDKey, v)
}

// isValidRequestID は F-6 妥当性基準を満たすか判定する。
//   - 空でない
//   - 長さ ≤ maxRequestIDLength オクテット
//   - 全文字が ASCII 印字可能 (0x21-0x7E)、制御文字・空白・タブなし
func isValidRequestID(s string) bool {
	if s == "" {
		return false
	}
	if len(s) > maxRequestIDLength {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x21 || c > 0x7E {
			return false
		}
	}
	return true
}

// sanitizeClientRequestID は印字可能 ASCII 範囲外の文字を Unicode escape (\uXXXX) に置換し、
// 結果を maxRequestIDLength オクテット以内に切り詰める (DoS 防止)。
//
// 入力は rune 単位で走査し、複数バイトの UTF-8 文字も 1 つの Unicode escape に変換する。
// 切り詰めは escape sequence の境界 (1 rune / 1 escape) で行い、`\u00` のような不完全な
// 表現を残さない。
func sanitizeClientRequestID(s string) string {
	var b strings.Builder
	for _, r := range s {
		var part string
		if r >= 0x21 && r <= 0x7E {
			part = string(r)
		} else {
			// rune が U+FFFF を超える場合は %x で 5-6 桁に伸びる (ὠ0 等)。
			part = fmt.Sprintf("\\u%04x", r)
		}
		if b.Len()+len(part) > maxRequestIDLength {
			break
		}
		b.WriteString(part)
	}
	return b.String()
}

// newRequestID は UUID v7 を生成して文字列化する。
//
// uuid.NewV7() は内部で crypto/rand を使うため、現実的にエラーは起きないが、
// 失敗時に Nil UUID (00000000-...) を返すと全リクエストが同一 ID 化して F-5 の
// トレーサビリティが破綻する。そのため失敗時はホスト + PID + 時刻 + atomic counter
// ベースの代替 ID を生成して一意性を維持する (UUID 形式ではないが request 同一化を防ぐ)。
//
// 一意性の根拠:
//   - hostname + PID: 同一マシン内では PID が一意、複数マシンでは hostname が一意。
//   - UnixNano + counter: 同一プロセス内では counter で 1 ナノ秒以内の衝突を回避。
func newRequestID() string {
	if id, err := uuid.NewV7(); err == nil {
		return id.String()
	}
	n := atomic.AddInt64(&fallbackSeq, 1)
	return fmt.Sprintf("fallback-%s-%d-%d-%d", fallbackHost, fallbackPID, time.Now().UnixNano(), n)
}

var (
	fallbackSeq int64
	// fallbackHost / fallbackPID は package init 時にキャッシュ。
	// hostname 取得失敗時は "unknown" に置換 (起動継続を優先)。
	fallbackHost = func() string {
		h, err := os.Hostname()
		if err != nil || h == "" {
			return "unknown"
		}
		return h
	}()
	fallbackPID = os.Getpid()
)
