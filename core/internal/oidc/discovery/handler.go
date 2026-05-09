package discovery

import (
	"fmt"
	"net/http"

	"github.com/mktkhr/id-core/core/internal/config"
)

// HTTP レスポンスヘッダ + Cache-Control 値の定数。
const (
	contentTypeJSON = "application/json"

	cacheControlNoCache = "no-cache, must-revalidate"
)

// Handler は `GET /.well-known/openid-configuration` の HTTP ハンドラ。
//
// 起動時に 1 回だけ Metadata 構築 + Marshal + ETag 計算を行い、リクエスト毎の処理は
// ヘッダ書き込み + キャッシュ済み body 書き出しのみ (F-21 + パフォーマンス)。
//
// すべて読み取り専用フィールドのため goroutine 安全。複数 Pod 環境でも同一鍵セット +
// 同一 config から構築されるなら全 Pod で body / ETag が一致する (F-21)。
type Handler struct {
	body  []byte // Marshal 結果 (起動時計算)
	etag  string // ETag(body) (起動時計算)
	cache string // Cache-Control ヘッダ値 (起動時計算)
}

// New は Handler を構築する。Marshal が失敗した場合に error を返す
// (Metadata は標準ライブラリだけで構築でき通常失敗しないが、防御的に error を返す I/F とする)。
func New(cfg config.OIDCConfig) (*Handler, error) {
	m := Build(cfg)
	body, err := Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("discovery: メタデータの marshal に失敗しました: %w", err)
	}
	return &Handler{
		body:  body,
		etag:  ETag(body),
		cache: cacheControlValue(cfg.DiscoveryMaxAge),
	}, nil
}

// ServeHTTP は OIDC Discovery レスポンスを返す。
//
// 挙動:
//   - If-None-Match ヘッダ値が ETag と完全一致 → 304 Not Modified (body なし)
//   - それ以外 → 200 OK + Content-Type / Cache-Control / ETag ヘッダ + JSON body
//
// メソッド制限 (GET 以外で 405) は P4 の chi router 側で行う。本 handler は GET 前提。
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("If-None-Match") == h.etag {
		// 304 では body / Content-Type を出さない (RFC 7232 §4.1)。
		// ETag は応答に含めるのが推奨 (304 ヘッダで RP がキャッシュ更新できる)。
		w.Header().Set("ETag", h.etag)
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", contentTypeJSON)
	w.Header().Set("Cache-Control", h.cache)
	w.Header().Set("ETag", h.etag)
	w.WriteHeader(http.StatusOK)
	// http.ResponseWriter.Write の error は middleware (access_log) 側でハンドルする
	// (HTTP transport が切れた場合等の表面化処理は本層では行わない)。
	_, _ = w.Write(h.body)
}

// cacheControlValue は CORE_OIDC_DISCOVERY_MAX_AGE に応じて Cache-Control 値を返す (F-1, Q15)。
//
//	maxAge == 0 (既定)  → "no-cache, must-revalidate"
//	maxAge >  0         → "public, max-age=<N>, must-revalidate"
//
// `no-cache` と `max-age` の併記矛盾を避けるため、必ずどちらか一方のみを出力する。
func cacheControlValue(maxAge int) string {
	if maxAge <= 0 {
		return cacheControlNoCache
	}
	return fmt.Sprintf("public, max-age=%d, must-revalidate", maxAge)
}
