// Command core は id-core OIDC OP の HTTP サーバーを起動する。
//
// M0.2: 構造化ログ + request_id middleware を組み込んだ最小骨格。
// M0.3: PostgreSQL pgxpool 接続 + dbmigrate.AssertClean (start gate) を起動シーケンスに統合。
// M1.1: keystore.Init で OIDC 署名鍵を読み込み (or 起動時生成)、起動 INFO/WARN ログを出して
//
//	server.New に keystore.KeySet を渡す。Discovery (`/.well-known/openid-configuration`)、
//	JWKS (`/jwks`)、未実装 endpoint stub (`/authorize`, `/token`, `/userinfo` の 503) が登録される。
//
// 後続マイルストーンで OIDC エンドポイント本実装 (authorize / token / userinfo) を追加する。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/mktkhr/id-core/core/internal/config"
	"github.com/mktkhr/id-core/core/internal/db"
	"github.com/mktkhr/id-core/core/internal/dbmigrate"
	"github.com/mktkhr/id-core/core/internal/keystore"
	"github.com/mktkhr/id-core/core/internal/logger"
	"github.com/mktkhr/id-core/core/internal/server"
)

// recommendedMinKeyBitLen は keystore 起動時に WARN ログを出す閾値 (M1.1 設計 #32 論点 #16)。
// RS256 として 2048 bit が下限推奨で、これを下回る鍵は受け入れるが運用警告を発する。
const recommendedMinKeyBitLen = 2048

// exitCode は run() の終了コード。標準 Unix 慣例に従い 0=成功 / 1=失敗。
const (
	exitOK    = 0
	exitError = 1
)

// defaultMigrationsSource は AssertClean が参照する migrations のソース URL の既定値。
// 実行 cwd を `core/` (`make run` 経路と同じ) 前提にしている。
// 別 cwd から起動する運用 (CI / コンテナ) では CORE_MIGRATIONS_DIR env で絶対パスを与える。
const defaultMigrationsSource = "file://db/migrations"

// resolveMigrationsSource は CORE_MIGRATIONS_DIR env を優先採用する。
// 値は file:// を前置していてもいなくても許容する (絶対パス文字列 / file:// URL の両方対応)。
func resolveMigrationsSource() string {
	v := os.Getenv("CORE_MIGRATIONS_DIR")
	if v == "" {
		return defaultMigrationsSource
	}
	if len(v) >= 7 && v[:7] == "file://" {
		return v
	}
	return "file://" + v
}

func main() {
	os.Exit(run(os.Stderr))
}

// run は main 本体を testable に切り出した関数。
//
// 起動シーケンス (M0.3 設計書 Q9 / F-6 / F-13):
//  1. bootstrap() で config + logger + event_id を準備
//  2. ctx に event_id を載せる
//  3. db.Open で pgxpool 接続 (失敗 → exit 1)
//  4. dbmigrate.AssertClean で schema_migrations の整合性確認 (dirty → exit 1)
//  5. server.New で HTTP サーバを構築
//  6. ListenAndServe (M0.2 既存パターン踏襲)
//
// 終了コードを int で返すことで、テスト側から異常系の直接実行と検証が可能。
func run(stderr *os.File) int {
	cfg, l, eventID, err := bootstrap()
	if err != nil {
		// bootstrap 失敗時は構造化ロガーが使えない可能性があるため、
		// 引数 stderr に最終フォールバック行を出す。
		fmt.Fprintf(stderr, "起動準備に失敗しました: %v\n", err)
		return exitError
	}

	ctx := logger.WithEventID(context.Background(), eventID)

	// db.Open は内部で pgxpool.NewWithConfig → pool.Ping(ctx) まで実施する (P1 設計)。
	// このため Q9 の起動順序 (logger → pgxpool → Ping → AssertClean → server) は
	// db.Open 一段で「pgxpool 構築 + 初回 Ping」をカバーしている。
	pool, err := db.Open(ctx, &cfg.Database, l)
	if err != nil {
		l.Error(ctx, "DB 接続に失敗しました", err)
		return exitError
	}
	defer pool.Close()

	migrationsSource := resolveMigrationsSource()
	if err := dbmigrate.AssertClean(ctx, db.BuildDSN(ctx, &cfg.Database), migrationsSource, l); err != nil {
		if errors.Is(err, dbmigrate.ErrDirty) {
			l.Error(ctx,
				"schema_migrations is dirty: 'make migrate-force VERSION=<n>' で復旧してください",
				err)
		} else {
			l.Error(ctx, "schema_migrations の整合性確認に失敗しました", err)
		}
		return exitError
	}

	// M1.1 (#32): keystore 初期化 + 起動 INFO/WARN ログ。
	// db.Open / AssertClean の後、server.New の前に挿入 (F-15 起動シーケンス)。
	ks, src, err := initKeystore(ctx, &cfg.OIDC, l)
	if err != nil {
		l.Error(ctx, "鍵管理層の初期化に失敗しました", err)
		return exitError
	}
	if err := emitKeystoreStartupLogs(ctx, l, ks, src, cfg.Env); err != nil {
		l.Error(ctx, "鍵情報の取得に失敗しました", err)
		return exitError
	}

	srv, err := server.New(cfg, l, pool, ks)
	if err != nil {
		l.Error(ctx, "HTTP サーバーの構築に失敗しました", err)
		return exitError
	}

	emitStartupLog(l, ctx, srv.Addr)

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		l.Error(ctx, "サーバーの実行に失敗しました", err)
		return exitError
	}
	return exitOK
}

// initKeystore は config.OIDCConfig から keystore.OIDCKeyConfig を組み立てて keystore.Init を呼ぶ。
//
// config.Load() 段階で env 別の必須/代替ルール (両方未指定の禁止 / prod での生成モード禁止) は
// 検証済のため、本関数は keystore に正しい入力を渡すアダプタとして動く。
func initKeystore(ctx context.Context, oidc *config.OIDCConfig, l *logger.Logger) (keystore.KeySet, keystore.Source, error) {
	return keystore.Init(ctx, keystore.OIDCKeyConfig{
		KeyFile:        oidc.KeyFile,
		DevGenerateKey: oidc.DevGenerateKey,
		KeyID:          oidc.KeyID,
	}, l)
}

// emitKeystoreStartupLogs は起動鍵情報 INFO ログ + 条件付き WARN ログを出力する (F-18、観測性)。
//
// 出力ログ:
//  1. INFO 起動鍵情報 (kid / alg / source / env、常時)
//  2. WARN dev 鍵生成モードの単一 Pod 制約 (source=generated 時のみ、F-8 / Q5)
//  3. WARN 鍵長 < 2048 bit の運用警告 (該当時のみ、論点 #16)
//
// **F-18 redact**: 秘密鍵 / PEM フルダンプ / RSA modulus / exponent はログに出さない。
// kid (= 公開鍵 DER SHA-256 先頭 24 hex) は公開可能な fingerprint として扱う。
func emitKeystoreStartupLogs(ctx context.Context, l *logger.Logger, ks keystore.KeySet, src keystore.Source, env config.EnvName) error {
	pair, err := ks.Active(ctx)
	if err != nil {
		return fmt.Errorf("keystore.Active: %w", err)
	}

	l.Info(ctx, "起動鍵情報",
		slog.String("kid", pair.Kid),
		slog.String("alg", pair.Alg),
		slog.String("source", src.String()),
		slog.String("env", string(env)),
	)

	if src == keystore.SourceGenerated {
		// F-8 / Q5: dev 鍵生成モード = 単一 Pod 専用。Helm/manifest 側で replicas: 1 を強制する責務分界。
		l.Warn(ctx, "dev 鍵生成モード = 単一 Pod 専用、複数 Pod 環境では CORE_OIDC_KEY_FILE で共有 Secret を使え")
	}

	if bitLen := pair.BitLen(); bitLen > 0 && bitLen < recommendedMinKeyBitLen {
		// 論点 #16: 鍵長は透過 (拒否しない) が、運用上 RS256 として弱い鍵長には WARN を出す。
		l.Warn(ctx, fmt.Sprintf("鍵長 %d bit は RS256 として弱い、%d bit 以上を推奨", bitLen, recommendedMinKeyBitLen),
			slog.Int("bit_len", bitLen),
		)
	}

	return nil
}

// emitStartupLog は起動 INFO ログを構造化形式で出力する (F-4 / F-14 / Q3)。
//
// 必須フィールド:
//   - msg: "core サーバーを起動します" (日本語)
//   - addr: 待ち受けアドレス (例: ":8080")
//   - event_id: ctx に WithEventID 済みの UUID v7 が logger により自動付与される
//
// event_id は ctx 単一ソースに統一する (引数二重指定で値が食い違うバグを避ける)。
// run() から分離することで、テストで任意の logger を注入して JSON スキーマを検証できる。
func emitStartupLog(l *logger.Logger, ctx context.Context, addr string) {
	l.Info(ctx, "core サーバーを起動します",
		slog.String("addr", addr),
	)
}

// bootstrap は起動前の設定読み込み・ロガー初期化・event_id 発番をまとめて行う。
// 各失敗を error として返し、run() で終了コード判定する責務分離。
//
// 注意: M0.3 から DB 接続は run() 側に置く (pool の lifecycle を defer で管理するため)。
// bootstrap は I/O を伴わない準備のみを担当する。
func bootstrap() (*config.Config, *logger.Logger, string, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, "", fmt.Errorf("設定の読み込みに失敗しました: %w", err)
	}

	l, err := logger.Default()
	if err != nil {
		return nil, nil, "", fmt.Errorf("ロガーの初期化に失敗しました: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, nil, "", fmt.Errorf("event_id (UUID v7) の生成に失敗しました: %w", err)
	}

	return cfg, l, id.String(), nil
}
