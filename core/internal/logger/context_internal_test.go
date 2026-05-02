package logger

// 内部キー requestIDKey / eventIDKey に直接 string 以外を入れて、
// RequestIDFrom / EventIDFrom の型アサーション失敗パスが空文字を返すことを直接検証する。
// (外部テストパッケージからは internal キーへ書き込めないため、本テストのみ package logger 側に置く)

import (
	"context"
	"testing"
)

func TestRequestIDFrom_TypeAssertFailure_ReturnsEmpty(t *testing.T) {
	ctx := context.WithValue(context.Background(), requestIDKey, 12345) // 非 string

	if got := RequestIDFrom(ctx); got != "" {
		t.Errorf("RequestIDFrom (non-string value) = %q, want empty", got)
	}
}

func TestEventIDFrom_TypeAssertFailure_ReturnsEmpty(t *testing.T) {
	ctx := context.WithValue(context.Background(), eventIDKey, struct{}{}) // 非 string

	if got := EventIDFrom(ctx); got != "" {
		t.Errorf("EventIDFrom (non-string value) = %q, want empty", got)
	}
}
