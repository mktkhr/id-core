package jwks

import (
	"context"
	"fmt"
	"net/http"

	"github.com/mktkhr/id-core/core/internal/keystore"
)

// HTTP レスポンスヘッダ + Cache-Control 値の定数。
const (
	contentTypeJSON = "application/json"

	cacheControlNoCache = "no-cache, must-revalidate"
)

// Handler は `GET /jwks` の HTTP ハンドラ (F-5)。
//
// 起動時に 1 回だけ keystore.Verifying() → BuildSet → Marshal → ETag を実行し、
// リクエスト毎の処理はヘッダ書き込み + キャッシュ済み body 書き出しのみ (F-21 + パフォーマンス)。
//
// 全フィールド読み取り専用で goroutine 安全。複数 Pod 環境でも同一鍵セットから
// 構築されるなら全 Pod で body / ETag が一致する (F-21、staticKeySet 前提)。
//
// M2.x の鍵 rotation 対応では Handler を起動時固定キャッシュから「鍵セット変更時に再構築」型に
// 拡張する必要があるが、M1.1 範囲では single-shot 構築で十分 (鍵更新 = Pod 再起動)。
type Handler struct {
	body  []byte // Marshal 結果 (起動時計算)
	etag  string // ETag(body) (起動時計算)
	cache string // Cache-Control ヘッダ値 (起動時計算)
}

// New は Handler を構築する。
//
// 失敗パターン:
//   - keystore.Verifying() の失敗 (KeySet が未初期化等)
//   - BuildSet の失敗 (公開鍵 nil、jwx の AddKey 失敗)
//   - Marshal の失敗 (jwx の内部エラー)
//
// New 失敗時は呼び出し側 (P4 main.go) で起動失敗扱いとする。
func New(ks keystore.KeySet, maxAge int) (*Handler, error) {
	if ks == nil {
		return nil, fmt.Errorf("jwks.New: KeySet が nil です")
	}
	keys, err := ks.Verifying(context.Background())
	if err != nil {
		return nil, fmt.Errorf("jwks.New: keystore.Verifying に失敗しました: %w", err)
	}
	set, err := BuildSet(keys)
	if err != nil {
		return nil, fmt.Errorf("jwks.New: %w", err)
	}
	body, err := Marshal(set)
	if err != nil {
		return nil, fmt.Errorf("jwks.New: %w", err)
	}
	return &Handler{
		body:  body,
		etag:  ETag(body),
		cache: cacheControlValue(maxAge),
	}, nil
}

// ServeHTTP は JWKS レスポンスを返す。
//
// 挙動:
//   - If-None-Match ヘッダ値が ETag と完全一致 → 304 Not Modified (body 空、ETag のみ応答)
//   - それ以外 → 200 OK + Content-Type / Cache-Control / ETag ヘッダ + JSON body
//
// メソッド制限 (GET 以外で 405) は P4 の chi router 側で行う。
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("If-None-Match") == h.etag {
		// 304 では body / Content-Type を出さない (RFC 7232 §4.1)。
		// ETag は応答に含めるのが推奨 (RP がキャッシュ更新可能)。
		w.Header().Set("ETag", h.etag)
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", contentTypeJSON)
	w.Header().Set("Cache-Control", h.cache)
	w.Header().Set("ETag", h.etag)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(h.body)
}

// cacheControlValue は CORE_OIDC_JWKS_MAX_AGE に応じて Cache-Control 値を返す (F-6)。
//
//	maxAge <= 0 → "no-cache, must-revalidate"  (rotation 中の即時更新等で 0 を選択する想定)
//	maxAge >  0 → "public, max-age=<N>, must-revalidate" (既定 300 秒)
//
// `immutable` は使わない (M2.x で鍵 rotation 予定のため、F-6)。
func cacheControlValue(maxAge int) string {
	if maxAge <= 0 {
		return cacheControlNoCache
	}
	return fmt.Sprintf("public, max-age=%d, must-revalidate", maxAge)
}
