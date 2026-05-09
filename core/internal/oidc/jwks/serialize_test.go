package jwks_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mktkhr/id-core/core/internal/keystore"
	"github.com/mktkhr/id-core/core/internal/logger"
	"github.com/mktkhr/id-core/core/internal/oidc/jwks"
)

// -update フラグ: golden ファイルを現在の出力で更新する (CI では使わない)。
//
//	go test ./internal/oidc/jwks -run Golden -update
var updateGolden = flag.Bool("update", false, "update jwks_golden.json from current output (ローカル更新時のみ)")

const (
	goldenPEMPath  = "testdata/test_rsa_2048.pem"
	goldenJSONPath = "testdata/jwks_golden.json"
)

// loadGoldenKeySet は固定 PEM (testdata/test_rsa_2048.pem) を keystore.Init 経由で読み込み、
// 単一鍵の jwk.Set に組み立てて Marshal バイト列を返す。
func loadGoldenKeySetBody(t *testing.T) []byte {
	t.Helper()
	l := logger.New(logger.FormatJSON, &bytes.Buffer{})
	ks, _, err := keystore.Init(context.Background(),
		keystore.OIDCKeyConfig{KeyFile: goldenPEMPath}, l)
	if err != nil {
		t.Fatalf("keystore.Init: %v", err)
	}
	keys, err := ks.Verifying(context.Background())
	if err != nil {
		t.Fatalf("Verifying: %v", err)
	}
	set, err := jwks.BuildSet(keys)
	if err != nil {
		t.Fatalf("BuildSet: %v", err)
	}
	body, err := jwks.Marshal(set)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return body
}

// 100 回 marshal で全て同一バイト列 (F-21、論点 #10 Codex HIGH 1)。
func TestMarshal_Deterministic_100x(t *testing.T) {
	first := loadGoldenKeySetBody(t)
	for i := 0; i < 99; i++ {
		got := loadGoldenKeySetBody(t)
		if !bytes.Equal(got, first) {
			t.Fatalf("Marshal は決定的でなければならない (call %d):\nfirst=%s\ngot  =%s", i, first, got)
		}
	}
}

// golden ファイル契約テスト: 固定 PEM から Build した JWKS が testdata/jwks_golden.json と完全一致。
//
// jwx の minor バージョンアップ等で出力フォーマットが変わると本テストが失敗する。
// 意図的な変更時は -update フラグで golden を再生成し、PR でレビュー差分を検査する (論点 #10)。
func TestMarshal_Golden(t *testing.T) {
	body := loadGoldenKeySetBody(t)

	if *updateGolden {
		// pretty 印字で人間可読性を上げる (golden は人間がレビューするファイル)。
		var buf bytes.Buffer
		if err := json.Indent(&buf, body, "", "  "); err != nil {
			t.Fatalf("Indent: %v", err)
		}
		// 末尾改行を追加して POSIX text file 慣習に合わせる
		buf.WriteByte('\n')
		if err := os.WriteFile(goldenJSONPath, buf.Bytes(), 0o644); err != nil {
			t.Fatalf("WriteFile golden: %v", err)
		}
		t.Logf("golden updated: %s", goldenJSONPath)
		return
	}

	want, err := os.ReadFile(goldenJSONPath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("golden ファイル %s が存在しません。-update フラグで生成してください", goldenJSONPath)
		}
		t.Fatalf("ReadFile golden: %v", err)
	}

	// golden は indent 済み + 末尾改行付きで保存しているため、比較側も同じ形に整える。
	var gotBuf bytes.Buffer
	if err := json.Indent(&gotBuf, body, "", "  "); err != nil {
		t.Fatalf("Indent: %v", err)
	}
	gotBuf.WriteByte('\n')

	if !bytes.Equal(gotBuf.Bytes(), want) {
		t.Errorf("JWKS が golden と一致しません (jwx の出力フォーマット変更の可能性)。\n--- want (golden) ---\n%s\n--- got ---\n%s",
			want, gotBuf.Bytes())
		// debug 補助: testdata 配下に got を保存
		debugPath := filepath.Join(filepath.Dir(goldenJSONPath), "jwks_golden.got.json")
		_ = os.WriteFile(debugPath, gotBuf.Bytes(), 0o644)
		t.Logf("got を %s に保存しました (差分確認用)", debugPath)
	}
}

// private 成分 (d / p / q / dp / dq / qi) が JWKS 全体に含まれない (Codex LOW 2、F-18)。
func TestMarshal_NoPrivateComponents(t *testing.T) {
	body := loadGoldenKeySetBody(t)

	var got struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(got.Keys) == 0 {
		t.Fatal("got.Keys is empty")
	}

	for i, k := range got.Keys {
		for _, forbidden := range []string{"d", "p", "q", "dp", "dq", "qi"} {
			if _, exists := k[forbidden]; exists {
				t.Errorf("keys[%d] に private 成分 %q が含まれている: %+v", i, forbidden, k)
			}
		}
	}
}

// ETag: 同一 body から常に同じ値 (100 回呼び出し全一致)。
func TestETag_Deterministic_100x(t *testing.T) {
	body := []byte(`{"keys":[]}`)
	first := jwks.ETag(body)
	for i := 0; i < 99; i++ {
		got := jwks.ETag(body)
		if got != first {
			t.Fatalf("ETag は決定的でなければならない (call %d): got %q, first %q", i, got, first)
		}
	}
}

// ETag: body が 1 バイト変わると値も変わる。
func TestETag_BodyChangeDetection(t *testing.T) {
	a := []byte(`{"keys":[{"kty":"RSA"}]}`)
	b := []byte(`{"keys":[{"kty":"EC"}]}`)
	if jwks.ETag(a) == jwks.ETag(b) {
		t.Error("ETag が異なる body で同一になった (sha256 衝突 = 異常)")
	}
}

// ETag フォーマット: strong ETag (24 文字、引用符込み)、padding なし、W/ なし。
func TestETag_Format(t *testing.T) {
	body := []byte(`{}`)
	got := jwks.ETag(body)
	if len(got) != 24 {
		t.Errorf("ETag length = %d, want 24", len(got))
	}
	if !strings.HasPrefix(got, `"`) || !strings.HasSuffix(got, `"`) {
		t.Errorf("ETag は引用符で囲まれているべき: %q", got)
	}
	if strings.HasPrefix(got, "W/") {
		t.Errorf("ETag に W/ プレフィックスが付いた (strong ETag 違反): %q", got)
	}
	if strings.Contains(got, "=") {
		t.Errorf("ETag に = padding が含まれた: %q", got)
	}
}

// 2 鍵の Set でも順序が安定 (M2.x rotation 対応の forward-compat)。
func TestBuildSet_TwoKeys_OrderStable(t *testing.T) {
	kp1 := newTestKeyPair(t, 2048)
	kp2 := newTestKeyPair(t, 2048)

	set1, err := jwks.BuildSet([]*keystore.KeyPair{kp1, kp2})
	if err != nil {
		t.Fatalf("BuildSet: %v", err)
	}
	body1, err := jwks.Marshal(set1)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// 同じ順序で再構築 → 同じバイト列
	set2, _ := jwks.BuildSet([]*keystore.KeyPair{kp1, kp2})
	body2, _ := jwks.Marshal(set2)
	if !bytes.Equal(body1, body2) {
		t.Errorf("同じ順序の BuildSet が異なる結果: \nbody1=%s\nbody2=%s", body1, body2)
	}

	// 順序を入れ替えると結果が変わる (= 順序が反映されている)
	setReverse, _ := jwks.BuildSet([]*keystore.KeyPair{kp2, kp1})
	bodyReverse, _ := jwks.Marshal(setReverse)
	if bytes.Equal(body1, bodyReverse) {
		t.Error("順序を入れ替えても結果が同じ = 順序が反映されていない")
	}
}

// BuildSet で nil 鍵が渡されたら明示エラー。
func TestBuildSet_NilKeyRejected(t *testing.T) {
	_, err := jwks.BuildSet([]*keystore.KeyPair{nil})
	if err == nil {
		t.Error("nil key should be rejected")
	}
}

// Marshal で nil set はエラー。
func TestMarshal_NilSetRejected(t *testing.T) {
	_, err := jwks.Marshal(nil)
	if err == nil {
		t.Error("nil set should be rejected")
	}
}
