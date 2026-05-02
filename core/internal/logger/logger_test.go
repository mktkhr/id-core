package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mktkhr/id-core/core/internal/logger"
)

// T-1: CORE_LOG_FORMAT=json (デフォルト) で 1 行 JSON が time/level/msg を含む。
func TestLogger_JSONFormat_Default(t *testing.T) {
	t.Setenv("CORE_LOG_FORMAT", "")

	f, err := logger.FormatFromEnv()
	if err != nil {
		t.Fatalf("FormatFromEnv: %v", err)
	}
	if f != logger.FormatJSON {
		t.Fatalf("default format = %v, want JSON", f)
	}

	var buf bytes.Buffer
	l := logger.New(f, &buf)
	l.Info(context.Background(), "test-msg")

	out := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(out, "{") || !strings.HasSuffix(out, "}") {
		t.Fatalf("output should be 1-line JSON, got: %q", out)
	}
	if strings.Count(out, "\n") != 0 {
		t.Errorf("output should be single line, got newlines: %q", out)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("Unmarshal: %v (out=%q)", err, out)
	}
	for _, k := range []string{"time", "level", "msg"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing key %q in output: %v", k, m)
		}
	}
	if m["msg"] != "test-msg" {
		t.Errorf("msg = %v, want test-msg", m["msg"])
	}
}

// T-2: CORE_LOG_FORMAT=text で key=value 形式が出る。
func TestLogger_TextFormat(t *testing.T) {
	t.Setenv("CORE_LOG_FORMAT", "text")

	f, err := logger.FormatFromEnv()
	if err != nil {
		t.Fatalf("FormatFromEnv: %v", err)
	}
	if f != logger.FormatText {
		t.Fatalf("format = %v, want Text", f)
	}

	var buf bytes.Buffer
	l := logger.New(f, &buf)
	l.Info(context.Background(), "hello")

	out := strings.TrimSpace(buf.String())
	if !strings.Contains(out, "msg=hello") {
		t.Errorf("text format should contain msg=hello, got: %q", out)
	}
	if !strings.Contains(out, "level=INFO") {
		t.Errorf("text format should contain level=INFO, got: %q", out)
	}
	// JSON 形式ではないこと。
	if strings.HasPrefix(out, "{") {
		t.Errorf("text format should not start with '{', got: %q", out)
	}
}

// T-3: CORE_LOG_FORMAT=invalid でエラー。
func TestFormatFromEnv_Invalid(t *testing.T) {
	t.Setenv("CORE_LOG_FORMAT", "yaml")

	_, err := logger.FormatFromEnv()
	if err == nil {
		t.Fatalf("expected error for invalid value, got nil")
	}
	if !strings.Contains(err.Error(), "CORE_LOG_FORMAT") {
		t.Errorf("error should mention CORE_LOG_FORMAT, got: %v", err)
	}
	if !strings.Contains(err.Error(), "yaml") {
		t.Errorf("error should mention the bad value 'yaml', got: %v", err)
	}
}

// T-4: time フィールドが RFC3339Nano UTC (Z suffix)。
func TestLogger_TimeFormat_UTCRFC3339Nano(t *testing.T) {
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)
	l.Info(context.Background(), "x")

	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	timeStr, ok := m["time"].(string)
	if !ok {
		t.Fatalf("time field not string, got %T", m["time"])
	}
	if !strings.HasSuffix(timeStr, "Z") {
		t.Errorf("time should have Z suffix (UTC), got: %q", timeStr)
	}
	parsed, err := time.Parse(time.RFC3339Nano, timeStr)
	if err != nil {
		t.Fatalf("time should parse as RFC3339Nano: %v (got: %q)", err, timeStr)
	}
	if parsed.Location() != time.UTC {
		t.Errorf("parsed location = %v, want UTC", parsed.Location())
	}
}

// 補助テスト: Error() が "error" フィールドを構造化 attr として付与し、
// 改行・制御文字を含むエラー文字列も JSON エンコーダ経由で安全にエスケープされる
// (F-1 ログインジェクション対策の確認)。
func TestLogger_Error_StructuredAndSafe(t *testing.T) {
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)
	injected := errors.New("malicious\n\"injected\"\x00line")
	l.Error(context.Background(), "boom", injected)

	out := strings.TrimSpace(buf.String())
	// 改行が JSON 文字列リテラル内で 1 行に収まっていること (= JSON が破壊されていない)。
	if strings.Count(out, "\n") != 0 {
		t.Errorf("output should be single line, got newlines: %q", out)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("Unmarshal: %v (out=%q)", err, out)
	}
	if got := m["msg"]; got != "boom" {
		t.Errorf("msg = %v, want boom", got)
	}
	if got, ok := m["error"].(string); !ok {
		t.Fatalf("error field should be string, got %T", m["error"])
	} else if got != injected.Error() {
		t.Errorf("error field = %q, want %q (round-trip via JSON encoder)", got, injected.Error())
	}
	if m["level"] != "ERROR" {
		t.Errorf("level = %v, want ERROR", m["level"])
	}
}

// T-5: ロガー初期化前後で time.Local が変化していない (プロセス全体への副作用なし)。
func TestLogger_NoSideEffectOnTimeLocal(t *testing.T) {
	before := time.Local

	var buf bytes.Buffer
	_ = logger.New(logger.FormatJSON, &buf)

	if time.Local != before {
		t.Errorf("time.Local mutated by logger init: before=%v after=%v", before, time.Local)
	}
}
