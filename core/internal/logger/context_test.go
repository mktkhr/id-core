package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mktkhr/id-core/core/internal/logger"
)

// T-6: WithRequestID -> RequestIDFrom で値が往復する。
func TestContext_RequestID_RoundTrip(t *testing.T) {
	ctx := logger.WithRequestID(context.Background(), "01911f4e-7234-7b2a-8000-000000000001")
	if got := logger.RequestIDFrom(ctx); got != "01911f4e-7234-7b2a-8000-000000000001" {
		t.Errorf("RequestIDFrom = %q, want UUIDv7", got)
	}
}

// T-7: WithEventID -> EventIDFrom で値が往復する。
func TestContext_EventID_RoundTrip(t *testing.T) {
	ctx := logger.WithEventID(context.Background(), "01911f4e-7234-7b2a-8000-000000000002")
	if got := logger.EventIDFrom(ctx); got != "01911f4e-7234-7b2a-8000-000000000002" {
		t.Errorf("EventIDFrom = %q, want UUIDv7", got)
	}
}

// T-8: 未設定の context から取得しても panic せず空文字を返す。
func TestContext_MissingID_ReturnsEmpty(t *testing.T) {
	ctx := context.Background()

	if got := logger.RequestIDFrom(ctx); got != "" {
		t.Errorf("RequestIDFrom (missing) = %q, want empty", got)
	}
	if got := logger.EventIDFrom(ctx); got != "" {
		t.Errorf("EventIDFrom (missing) = %q, want empty", got)
	}
}

// 補助テスト: 非 string 型の値が ctx に入っていても panic せず空文字を返す。
// 通常は WithRequestID 経由でしか書けないが、防御的型アサーションの retreat path を担保する。
func TestContext_NonStringValue_ReturnsEmpty(t *testing.T) {
	// 同じキー型で別の型の値を入れるには内部キーへのアクセスが必要だが、
	// ここでは同名の独自キーで context を汚染しても影響しないことを確認するに留める。
	type unrelatedKey struct{ name string }
	ctx := context.WithValue(context.Background(), unrelatedKey{"request_id"}, 12345)

	if got := logger.RequestIDFrom(ctx); got != "" {
		t.Errorf("RequestIDFrom should be insulated from unrelated keys, got: %q", got)
	}
	if got := logger.EventIDFrom(ctx); got != "" {
		t.Errorf("EventIDFrom should be insulated from unrelated keys, got: %q", got)
	}
}

// 補助テスト: nil context でも panic しない (防御)。
func TestContext_NilContext_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RequestIDFrom panicked on nil ctx: %v", r)
		}
	}()
	//nolint:staticcheck // nil context の防御挙動を直接検証する目的。
	if got := logger.RequestIDFrom(nil); got != "" {
		t.Errorf("RequestIDFrom(nil) = %q, want empty", got)
	}
	if got := logger.EventIDFrom(nil); got != "" {
		t.Errorf("EventIDFrom(nil) = %q, want empty", got)
	}
}

// T-9 (a): logger.Info 経由で context の request_id が自動付与される (HTTP 経路想定)。
func TestLogger_AutoAttachesRequestID(t *testing.T) {
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)
	ctx := logger.WithRequestID(context.Background(), "req-abc")

	l.Info(ctx, "handled")

	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &m); err != nil {
		t.Fatalf("Unmarshal: %v (out=%q)", err, buf.String())
	}
	if got := m["request_id"]; got != "req-abc" {
		t.Errorf("request_id = %v, want req-abc", got)
	}
	if _, hasEvent := m["event_id"]; hasEvent {
		t.Errorf("event_id should not be present when only request_id is set, got: %v", m["event_id"])
	}
}

// T-9 (b): logger.Info 経由で context の event_id が自動付与される (非 HTTP 経路想定)。
func TestLogger_AutoAttachesEventID(t *testing.T) {
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)
	ctx := logger.WithEventID(context.Background(), "evt-xyz")

	l.Info(ctx, "background-job")

	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got := m["event_id"]; got != "evt-xyz" {
		t.Errorf("event_id = %v, want evt-xyz", got)
	}
	if _, hasReq := m["request_id"]; hasReq {
		t.Errorf("request_id should not be present when only event_id is set, got: %v", m["request_id"])
	}
}

// 補助テスト: id 未設定の context から出力すると、request_id / event_id どちらも attr に
// 含まれない (空文字で誤って付与しない)。
func TestLogger_NoAutoAttachWhenContextEmpty(t *testing.T) {
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)

	l.Info(context.Background(), "no-ctx-ids")

	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, ok := m["request_id"]; ok {
		t.Errorf("request_id should be omitted when ctx has none, got: %v", m["request_id"])
	}
	if _, ok := m["event_id"]; ok {
		t.Errorf("event_id should be omitted when ctx has none, got: %v", m["event_id"])
	}
}
