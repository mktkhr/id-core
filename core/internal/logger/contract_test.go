// F-16 ログスキーマ契約テスト。
//
// 検証対象は 2 系統に分割する:
//
//	(a) HTTP 経路 (msg=access)        — time / level / msg / request_id / method / path / status / duration_ms
//	(b) 非 HTTP 経路 (起動 / job 等)   — time / level / msg / event_id
//
// 方針:
//   - 必須フィールドの存在 + 型 (string / number) を検証する。値の正確性は検証しない
//     (本契約はスキーマ契約で、業務的妥当性は別テスト)。
//   - 追加フィールドは許容する (前方互換)。
//   - 必須フィールドの欠落・型変更はテスト失敗 (破壊的変更検知)。
package logger_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mktkhr/id-core/core/internal/logger"
	"github.com/mktkhr/id-core/core/internal/middleware"
)

type fieldKind int

const (
	kindString fieldKind = iota
	kindNumber
)

func (k fieldKind) String() string {
	switch k {
	case kindString:
		return "string"
	case kindNumber:
		return "number"
	default:
		return "unknown"
	}
}

type fieldSpec struct {
	name string
	kind fieldKind
}

var httpRequiredFields = []fieldSpec{
	{"time", kindString},
	{"level", kindString},
	{"msg", kindString},
	{"request_id", kindString},
	{"method", kindString},
	{"path", kindString},
	{"status", kindNumber},
	{"duration_ms", kindNumber},
}

var nonHTTPRequiredFields = []fieldSpec{
	{"time", kindString},
	{"level", kindString},
	{"msg", kindString},
	{"event_id", kindString},
}

// validateContract は record が specs の必須フィールドを全て持ち、
// 期待型と一致することを検証する。違反内容を文字列スライスで返す
// (空 slice = 適合)。追加フィールドは無視 (許容)。
func validateContract(record map[string]any, specs []fieldSpec) []string {
	var problems []string
	for _, s := range specs {
		v, ok := record[s.name]
		if !ok {
			problems = append(problems, fmt.Sprintf("missing field %q", s.name))
			continue
		}
		switch s.kind {
		case kindString:
			if _, ok := v.(string); !ok {
				problems = append(problems, fmt.Sprintf("field %q: expected %s, got %T", s.name, s.kind, v))
			}
		case kindNumber:
			// encoding/json は数値を float64 にデコードする。
			if _, ok := v.(float64); !ok {
				problems = append(problems, fmt.Sprintf("field %q: expected %s, got %T", s.name, s.kind, v))
			}
		}
	}
	return problems
}

// emitHTTPAccessLog は access_log middleware と等価なフィールド構成で 1 行出力する。
// access_log middleware の実装に依存せず logger 単体で検証可能にするため、契約テスト内で
// 直接フィールドを組み立てる。
func emitHTTPAccessLog(ctx context.Context, l *logger.Logger, extra ...any) {
	args := []any{
		"method", "GET",
		"path", "/healthz",
		"status", 200,
		"duration_ms", 1.234,
	}
	args = append(args, extra...)
	l.Info(ctx, "access", args...)
}

// emitStartupLog は cmd/main の起動ログと等価なフィールド構成で 1 行出力する。
func emitStartupLog(ctx context.Context, l *logger.Logger, extra ...any) {
	l.Info(ctx, "core starting", extra...)
}

func decodeOneLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	out := strings.TrimSpace(buf.String())
	if strings.Count(out, "\n") != 0 {
		t.Fatalf("expected single-line JSON, got newlines: %q", out)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("Unmarshal: %v (out=%q)", err, out)
	}
	return m
}

// T-58: HTTP 経路ログの必須フィールド存在 + 型を検証する。
//
// 2 経路で検証する:
//
//   - "logger 直叩き": logger 単体で組み立てた access ログ
//   - "middleware 実体": middleware.RequestID + middleware.AccessLog 経由の
//     実 HTTP リクエストから出力されるログ (実装ドリフト検知)
func TestContract_HTTPPathSchema(t *testing.T) {
	t.Run("logger 直叩き", func(t *testing.T) {
		var buf bytes.Buffer
		l := logger.New(logger.FormatJSON, &buf)
		ctx := logger.WithRequestID(context.Background(), "01890000-0000-7000-8000-000000000001")

		emitHTTPAccessLog(ctx, l)

		m := decodeOneLine(t, &buf)
		if problems := validateContract(m, httpRequiredFields); len(problems) > 0 {
			t.Fatalf("HTTP path schema violations:\n  %s\nrecord=%v", strings.Join(problems, "\n  "), m)
		}
	})

	t.Run("middleware 実体経由", func(t *testing.T) {
		var buf bytes.Buffer
		l := logger.New(logger.FormatJSON, &buf)

		// middleware.RequestID + middleware.AccessLog の実体を通した出力を検証する。
		// access_log middleware の出力スキーマが将来変わった場合、このテストが失敗する。
		h := middleware.RequestID(middleware.AccessLog(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})))

		req := httptest.NewRequest(http.MethodGet, "/health?x=1", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		// access_log は 1 行だけ出す。末尾改行を取り除いて単一レコードとして decode する。
		m := decodeOneLine(t, &buf)
		if problems := validateContract(m, httpRequiredFields); len(problems) > 0 {
			t.Fatalf("HTTP path (middleware) schema violations:\n  %s\nrecord=%v", strings.Join(problems, "\n  "), m)
		}
	})
}

// T-59: 非 HTTP 経路ログの必須フィールド存在 + 型を検証する。
func TestContract_NonHTTPPathSchema(t *testing.T) {
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)
	ctx := logger.WithEventID(context.Background(), "01890000-0000-7000-8000-000000000002")

	emitStartupLog(ctx, l)

	m := decodeOneLine(t, &buf)
	if problems := validateContract(m, nonHTTPRequiredFields); len(problems) > 0 {
		t.Fatalf("non-HTTP path schema violations:\n  %s\nrecord=%v", strings.Join(problems, "\n  "), m)
	}
}

// T-60: 追加フィールドが含まれてもテストは失敗しない (前方互換)。
func TestContract_FieldAdditionAllowed(t *testing.T) {
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)
	ctx := logger.WithRequestID(context.Background(), "01890000-0000-7000-8000-000000000003")

	emitHTTPAccessLog(ctx, l,
		"client_request_id", "external-trace-xyz",
		"custom_extra", "tolerated",
	)

	m := decodeOneLine(t, &buf)
	if problems := validateContract(m, httpRequiredFields); len(problems) > 0 {
		t.Fatalf("extra fields should be tolerated, got violations:\n  %s\nrecord=%v",
			strings.Join(problems, "\n  "), m)
	}
	if _, ok := m["custom_extra"]; !ok {
		t.Errorf("custom_extra should be present in output, record=%v", m)
	}
}

// T-61: 必須フィールドが欠けると validateContract が違反を返す (破壊的変更検知)。
//
// validateContract の検出能力自体を直接検証することで、T-58 / T-59 が
// "実装が壊れた瞬間に契約テストが赤くなる" ことを保証する。
func TestContract_FieldRemovalFails(t *testing.T) {
	cases := []struct {
		name      string
		record    map[string]any
		specs     []fieldSpec
		wantField string
	}{
		{
			name: "HTTP: request_id 欠落",
			record: map[string]any{
				"time":        "2026-05-02T00:00:00Z",
				"level":       "INFO",
				"msg":         "access",
				"method":      "GET",
				"path":        "/healthz",
				"status":      float64(200),
				"duration_ms": float64(1.0),
			},
			specs:     httpRequiredFields,
			wantField: "request_id",
		},
		{
			name: "HTTP: status 欠落",
			record: map[string]any{
				"time":        "2026-05-02T00:00:00Z",
				"level":       "INFO",
				"msg":         "access",
				"request_id":  "id",
				"method":      "GET",
				"path":        "/healthz",
				"duration_ms": float64(1.0),
			},
			specs:     httpRequiredFields,
			wantField: "status",
		},
		{
			name: "non-HTTP: event_id 欠落",
			record: map[string]any{
				"time":  "2026-05-02T00:00:00Z",
				"level": "INFO",
				"msg":   "core starting",
			},
			specs:     nonHTTPRequiredFields,
			wantField: "event_id",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := validateContract(tc.record, tc.specs)
			if len(problems) == 0 {
				t.Fatalf("expected violation for missing %q, got none", tc.wantField)
			}
			joined := strings.Join(problems, "\n")
			if !strings.Contains(joined, fmt.Sprintf("missing field %q", tc.wantField)) {
				t.Errorf("violation should mention missing %q, got:\n%s", tc.wantField, joined)
			}
		})
	}
}

// T-62: フィールド型が変わると validateContract が違反を返す (破壊的変更検知)。
func TestContract_FieldTypeChangeFails(t *testing.T) {
	cases := []struct {
		name      string
		record    map[string]any
		specs     []fieldSpec
		wantField string
	}{
		{
			name: "status が string にすり替わる",
			record: map[string]any{
				"time":        "2026-05-02T00:00:00Z",
				"level":       "INFO",
				"msg":         "access",
				"request_id":  "id",
				"method":      "GET",
				"path":        "/healthz",
				"status":      "200", // string にすり替え
				"duration_ms": float64(1.0),
			},
			specs:     httpRequiredFields,
			wantField: "status",
		},
		{
			name: "duration_ms が string にすり替わる",
			record: map[string]any{
				"time":        "2026-05-02T00:00:00Z",
				"level":       "INFO",
				"msg":         "access",
				"request_id":  "id",
				"method":      "GET",
				"path":        "/healthz",
				"status":      float64(200),
				"duration_ms": "1.0",
			},
			specs:     httpRequiredFields,
			wantField: "duration_ms",
		},
		{
			name: "request_id が number にすり替わる",
			record: map[string]any{
				"time":        "2026-05-02T00:00:00Z",
				"level":       "INFO",
				"msg":         "access",
				"request_id":  float64(123),
				"method":      "GET",
				"path":        "/healthz",
				"status":      float64(200),
				"duration_ms": float64(1.0),
			},
			specs:     httpRequiredFields,
			wantField: "request_id",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := validateContract(tc.record, tc.specs)
			if len(problems) == 0 {
				t.Fatalf("expected violation for type-changed %q, got none", tc.wantField)
			}
			joined := strings.Join(problems, "\n")
			if !strings.Contains(joined, fmt.Sprintf("field %q: expected", tc.wantField)) {
				t.Errorf("violation should mention type mismatch on %q, got:\n%s", tc.wantField, joined)
			}
		})
	}
}
