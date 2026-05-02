// Package db は PostgreSQL への接続層 (pgxpool) と関連ユーティリティを提供する。
//
// M0.3 で導入。M2.x 以降のドメイン Repository から本パッケージの *pgxpool.Pool が利用される。
package db

import (
	"context"
	"fmt"
	"net/url"

	"github.com/mktkhr/id-core/core/internal/config"
)

// BuildDSN は cfg から PostgreSQL DSN 文字列 (URL 形式) を組み立てる。
//
// 形式: postgres://<user>:<password>@<host>:<port>/<dbname>?sslmode=<sslmode>
// userinfo (user / password) は url.UserPassword で構築する。url.UserPassword は
// 内部で userinfo 部分用のエスケープを行うため、`@` `:` `/` `?` `#` `%` `空白`
// 等の特殊文字を含んでも parse 後に元値が復元される。
//
// 第 1 引数 ctx は F-18 (internal/db の全公開 API は context.Context を第 1 引数に取る)
// 規約準拠のため受け取るが、本関数は純粋に文字列組み立てのみで cancel 観測しない。
//
// 注意: 戻り値には平文パスワードが含まれる。ログ出力や stderr へのダンプに利用してはならない (F-10)。
// ロギング目的では SafeRepr を利用すること。
func BuildDSN(_ context.Context, cfg *config.DatabaseConfig) string {
	q := url.Values{}
	q.Set("sslmode", cfg.SSLMode)
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:     "/" + cfg.DBName,
		RawQuery: q.Encode(),
	}
	return u.String()
}

// SafeRepr は cfg のうちログ出力可能な項目のみを map で返す。
// password は意図的に含めず、user は接続失敗解析の手掛かりとして含める (sensitive 度低)。
//
// 第 1 引数 ctx は F-18 規約準拠のため受け取るが、本関数は純粋に map 構築のみで cancel 観測しない。
//
// F-10: 接続失敗ログには本関数の戻り値のみを利用する。BuildDSN の結果を直接ログに渡さないこと。
func SafeRepr(_ context.Context, cfg *config.DatabaseConfig) map[string]any {
	return map[string]any{
		"host":    cfg.Host,
		"port":    cfg.Port,
		"user":    cfg.User,
		"dbname":  cfg.DBName,
		"sslmode": cfg.SSLMode,
	}
}
