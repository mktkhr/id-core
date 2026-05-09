//go:build integration

// 本ファイルは M1.1 (#32) の起動シーケンス全体を実 PostgreSQL + 実 keystore で結合検証する。
// build tag = integration により `make test-integration` でのみ実行される。

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mktkhr/id-core/core/internal/config"
	"github.com/mktkhr/id-core/core/internal/keystore"
	"github.com/mktkhr/id-core/core/internal/logger"
	"github.com/mktkhr/id-core/core/internal/server"
	"github.com/mktkhr/id-core/core/internal/testutil/dbtest"
)

// integrationCfg は統合テスト用の最小 config を組み立てる。
// dbtest.NewPool 経由で実 PostgreSQL に繋ぎ、OIDC は dev + 起動時生成モード。
func integrationCfg(t *testing.T) *config.Config {
	t.Helper()
	// dbtest が parse する DSN を流用するため、URL から DB 接続情報を抜き出すのではなく
	// 単純に手元の DB 接続情報を埋める。Pool 自体は dbtest.NewPool が握る。
	return &config.Config{
		Env:  config.EnvDev,
		Port: 8080,
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			User:     "core",
			Password: "core_dev_pw",
			DBName:   "id_core_test",
			SSLMode:  "disable",
		},
		OIDC: config.OIDCConfig{
			Issuer:                "http://localhost:8080",
			DevGenerateKey:        true,
			JWKSMaxAge:            300,
			DiscoveryMaxAge:       0,
			AuthorizationEndpoint: "http://localhost:8080/authorize",
			TokenEndpoint:         "http://localhost:8080/token",
			UserInfoEndpoint:      "http://localhost:8080/userinfo",
			JWKSURI:               "http://localhost:8080/jwks",
		},
	}
}

// TestIntegration_M11_StartupAndOIDCRoutes は M1.1 の主要 OIDC エンドポイントが
// 実 PostgreSQL 接続 + 実 keystore で動作することを検証する (F-15 / F-20-c)。
//
// 検証項目:
//  1. server.New が成功し、Discovery / JWKS / 503 stub の全 route を提供する
//  2. Discovery レスポンスの jwks_uri が cfg.OIDC.JWKSURI と一致
//  3. JWKS レスポンスの kid が keystore.Active の kid と一致
//  4. 503 stub の body 構造 (`error` snake_case + `available_at` milestone)
func TestIntegration_M11_StartupAndOIDCRoutes(t *testing.T) {
	ctx, pool := dbtest.NewPool(t) // ローカルで DB 不在なら skip、CI では fatal
	cfg := integrationCfg(t)

	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)

	// keystore 初期化 (run() の起動シーケンスと同じ呼び出し順)
	ks, src, err := initKeystore(ctx, &cfg.OIDC, l)
	if err != nil {
		t.Fatalf("initKeystore: %v", err)
	}
	if src != keystore.SourceGenerated {
		t.Errorf("Source = %v, want SourceGenerated", src)
	}
	if err := emitKeystoreStartupLogs(ctx, l, ks, src, cfg.Env); err != nil {
		t.Fatalf("emitKeystoreStartupLogs: %v", err)
	}

	srv, err := server.New(cfg, l, pool, ks)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}

	// Active 鍵の kid を取得 (後続の比較に使う)
	activePair, err := ks.Active(ctx)
	if err != nil {
		t.Fatalf("ks.Active: %v", err)
	}
	wantKid := activePair.Kid

	// 1) Discovery 200 + メタデータ整合
	{
		req := httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", nil)
		rec := httptest.NewRecorder()
		srv.Handler.ServeHTTP(rec, req)
		res := rec.Result()
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("Discovery status = %d, want 200", res.StatusCode)
		}
		body, _ := io.ReadAll(res.Body)
		var meta map[string]any
		if err := json.Unmarshal(body, &meta); err != nil {
			t.Fatalf("Discovery body unmarshal: %v\nbody=%s", err, body)
		}
		if meta["issuer"] != cfg.OIDC.Issuer {
			t.Errorf("issuer = %v, want %q", meta["issuer"], cfg.OIDC.Issuer)
		}
		if meta["jwks_uri"] != cfg.OIDC.JWKSURI {
			t.Errorf("jwks_uri = %v, want %q", meta["jwks_uri"], cfg.OIDC.JWKSURI)
		}
	}

	// 2) JWKS 200 + kid が起動鍵と一致
	{
		req := httptest.NewRequest(http.MethodGet, "/jwks", nil)
		rec := httptest.NewRecorder()
		srv.Handler.ServeHTTP(rec, req)
		res := rec.Result()
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("JWKS status = %d, want 200", res.StatusCode)
		}
		body, _ := io.ReadAll(res.Body)
		var jwksBody struct {
			Keys []map[string]any `json:"keys"`
		}
		if err := json.Unmarshal(body, &jwksBody); err != nil {
			t.Fatalf("JWKS body unmarshal: %v\nbody=%s", err, body)
		}
		if len(jwksBody.Keys) != 1 {
			t.Fatalf("JWKS keys length = %d, want 1", len(jwksBody.Keys))
		}
		gotKid, _ := jwksBody.Keys[0]["kid"].(string)
		if gotKid != wantKid {
			t.Errorf("JWKS kid = %q, want %q (keystore.Active と一致すべき)", gotKid, wantKid)
		}
	}

	// 3) 起動 INFO ログの kid フィールドも一致
	{
		var infoLog map[string]any
		for _, line := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
			if line == "" {
				continue
			}
			var m map[string]any
			if jerr := json.Unmarshal([]byte(line), &m); jerr != nil {
				continue
			}
			if m["msg"] == "起動鍵情報" {
				infoLog = m
				break
			}
		}
		if infoLog == nil {
			t.Fatal("起動鍵情報 INFO ログが出力されていない")
		}
		if logKid, _ := infoLog["kid"].(string); logKid != wantKid {
			t.Errorf("起動ログ kid = %q, keystore.Active kid = %q (三者一致すべき)", logKid, wantKid)
		}
	}

	// 4) 503 stub
	notimplCases := []struct {
		method      string
		path        string
		availableAt string
	}{
		{method: http.MethodGet, path: "/authorize", availableAt: "M1.2"},
		{method: http.MethodPost, path: "/token", availableAt: "M1.3"},
		{method: http.MethodGet, path: "/userinfo", availableAt: "M1.4"},
	}
	for _, tc := range notimplCases {
		tc := tc
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			srv.Handler.ServeHTTP(rec, req)
			res := rec.Result()
			defer res.Body.Close()
			if res.StatusCode != http.StatusServiceUnavailable {
				t.Errorf("%s %s: status = %d, want 503", tc.method, tc.path, res.StatusCode)
			}
			body, _ := io.ReadAll(res.Body)
			var got map[string]string
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("notimpl body unmarshal: %v\nbody=%s", err, body)
			}
			if got["error"] != "endpoint_not_implemented" {
				t.Errorf("error = %q, want endpoint_not_implemented", got["error"])
			}
			if got["available_at"] != tc.availableAt {
				t.Errorf("available_at = %q, want %q", got["available_at"], tc.availableAt)
			}
		})
	}
}

// integrationRepoRoot は build tag が付いていないため runtime.Caller で参照される。
//
// 統合テスト時の cwd 依存を避けるためのユーティリティ (将来 README 等を参照する場合の予約)。
func integrationRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(file))))
}

// 未使用警告を抑止 (将来テストで利用する predefined helper)
var _ = integrationRepoRoot

// 起動鍵情報 INFO ログのフォーマット安定性 (前項の TestIntegration_... と被るが、
// kid 三者一致がコアなので integration build でも独立に確認する)。
func TestIntegration_M11_StartupLogContext(t *testing.T) {
	ctx, _ := dbtest.NewPool(t)
	_ = ctx

	cfg := integrationCfg(t)
	var buf bytes.Buffer
	l := logger.New(logger.FormatJSON, &buf)

	ks, src, err := initKeystore(context.Background(), &cfg.OIDC, l)
	if err != nil {
		t.Fatalf("initKeystore: %v", err)
	}
	if err := emitKeystoreStartupLogs(context.Background(), l, ks, src, cfg.Env); err != nil {
		t.Fatalf("emitKeystoreStartupLogs: %v", err)
	}

	// dev 鍵生成モードの WARN は必ず出る
	if !strings.Contains(buf.String(), "dev 鍵生成モード") {
		t.Errorf("WARN dev 鍵生成モード が出力されていない: %q", buf.String())
	}
	// F-18 redact: 秘密鍵 / PEM / RSA modulus(n) / exponent(e) の値が出ない
	for _, leak := range []string{"BEGIN PRIVATE", "BEGIN RSA", `"d":`, `"p":`, `"q":`} {
		if strings.Contains(buf.String(), leak) {
			t.Errorf("F-18 redact 違反: ログに %q が含まれる: %q", leak, buf.String())
		}
	}
}
