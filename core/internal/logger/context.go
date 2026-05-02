// 責務境界 (F-3 / F-4 必須付与の保証):
//
// logger パッケージは "context に id があれば自動付与する" consumer 責務までを持つ。
// "HTTP 経路の全ログレコードに request_id を付与する" 必須性は、HTTP middleware 側で
// 全リクエスト境界に WithRequestID を呼ぶことで保証する (Issue #13 / P2 で実装)。
// 同様に "非 HTTP 経路で event_id 必須付与" は cmd/main やジョブランナー側の責務
// (起動時に WithEventID 済みの ctx を後段に渡す)。
//
// logger 側で未設定検知して fail loud しないのは、"何が HTTP 経路か" を logger が
// 識別する手段がなく (cmd / ジョブ / テスト等は request_id を持たない方が正常)、
// 自動採番すると middleware の取り違え (異なるリクエストに同一 ID) を隠蔽するため。
package logger

import "context"

// ctxKey は context.Context のキーとして使う非公開型。
// string ではなく独立した型を使うことで他パッケージとのキー衝突を避ける (Go 慣例)。
type ctxKey struct{ name string }

var (
	requestIDKey = ctxKey{"request_id"}
	eventIDKey   = ctxKey{"event_id"}
)

// WithRequestID は ctx に HTTP リクエスト ID (UUID v7) を載せた派生 context を返す。
// 仕様 F-3: HTTP 経路の全ログレコードに request_id 必須付与。
func WithRequestID(ctx context.Context, id string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFrom は ctx から request_id を取り出す。未設定または非 string の場合は空文字。
// nil context を防御的に許容する (panic しない)。
func RequestIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// WithEventID は ctx に非 HTTP 経路用イベント ID (UUID v7) を載せた派生 context を返す。
// 仕様 F-4: HTTP 経路外のログレコードには event_id 必須付与 (起動毎・ジョブ毎の一意 ID)。
func WithEventID(ctx context.Context, id string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, eventIDKey, id)
}

// EventIDFrom は ctx から event_id を取り出す。未設定または非 string の場合は空文字。
// nil context を防御的に許容する (panic しない)。
func EventIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(eventIDKey).(string); ok {
		return v
	}
	return ""
}
