package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/mktkhr/id-core/core/internal/apperror"
	"github.com/mktkhr/id-core/core/internal/logger"
)

// Recover middleware は handler の panic を捕捉し、HTTP 500 + F-7 基本形 JSON で
// クライアントに応答する (F-9 / F-10)。
//
// セキュリティ要件 (F-10):
//   - クライアントには固定メッセージ + request_id のみを返す。
//   - スタックトレース・内部ファイルパス・実装詳細は HTTP レスポンスに絶対に含めない。
//   - スタックトレースは内部の構造化ログ (level=ERROR) にのみ記録する。
//
// 設計順序 (D1):
//
//	request_id → access_log → recover → handler
//
// recover が access_log の内側に置かれているため、handler の panic は recover が
// 500 応答に変換した後、access_log の defer に戻る → access_log は status=500 / ERROR を
// 観測できる。逆順では access_log.defer が panic unwind 中に走り status=0 / 誤判定になる。
//
// l == nil の場合は契約違反として panic する (シングルポイント設計の前提を統一)。
func Recover(l *logger.Logger, next http.Handler) http.Handler {
	if l == nil {
		panic("middleware.Recover: logger must not be nil")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// access_log と同じ statusRecorder で wroteHeader を追跡する。
		// ハンドラが panic 前に WriteHeader / Write 済みの場合、http.ResponseWriter は
		// WriteHeader を二度受け付けないため、500 化を諦めてログにのみ記録する
		// (D1 順序により外側の access_log は handler が書いた status を観測する)。
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		defer func() {
			v := recover()
			if v == nil {
				return
			}

			ctx := r.Context()
			requestID := logger.RequestIDFrom(ctx)

			// panic 値を error / 文字列で正規化。
			var panicErr error
			switch t := v.(type) {
			case error:
				panicErr = t
			default:
				panicErr = fmt.Errorf("%v", t)
			}

			// 内部 ERROR ログ: スタックトレース付き (F-10)。
			stack := string(debug.Stack())
			attrs := []any{slog.String("stack_trace", stack)}
			if rw.wroteHeader {
				attrs = append(attrs, slog.Int("status_already_written", rw.status))
			}
			l.Error(ctx, "ハンドラで panic を捕捉しました", panicErr, attrs...)

			if rw.wroteHeader {
				// 既にレスポンスが書き込まれているため 500 化不能。クライアントは
				// 中途半端なレスポンスを受け取る可能性があるが、middleware では
				// 救済不能のためログのみ残して終わる (handler 側のバグとして扱う)。
				return
			}

			// クライアント応答: 固定メッセージ + request_id (スタック非含)。
			// apperror.WriteJSON が Content-Type "application/json; charset=utf-8" を設定する。
			coded := apperror.New(apperror.CodeInternalError, apperror.MessageInternalError)
			if err := apperror.WriteJSON(rw, http.StatusInternalServerError, coded, requestID); err != nil {
				// 書き込み失敗 (クライアント切断等)。本筋のリクエスト処理は終わっているので
				// ログにのみ残し、再 panic はしない。
				l.Error(ctx, "panic 応答の書き込みに失敗しました", err)
			}
		}()

		next.ServeHTTP(rw, r)
	})
}
