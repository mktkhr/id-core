package jwks_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"testing"

	"github.com/lestrrat-go/jwx/v3/jwk"

	"github.com/mktkhr/id-core/core/internal/keystore"
	"github.com/mktkhr/id-core/core/internal/oidc/jwks"
)

// テスト用に keystore.KeyPair を構築する。kid は keystore.DeriveKid で算出。
func newTestKeyPair(t *testing.T, bits int) *keystore.KeyPair {
	t.Helper()
	rsaKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	kid, err := keystore.DeriveKid(&rsaKey.PublicKey)
	if err != nil {
		t.Fatalf("DeriveKid: %v", err)
	}
	return &keystore.KeyPair{
		Kid:        kid,
		PublicKey:  &rsaKey.PublicKey,
		PrivateKey: rsaKey,
		Alg:        keystore.AlgRS256,
	}
}

// 公開鍵 → JWK 変換で kty / use / alg / kid が明示セットされる (論点 #10、F-5)。
func TestToJWK_BasicFields(t *testing.T) {
	kp := newTestKeyPair(t, 2048)
	key, err := jwks.ToJWK(kp)
	if err != nil {
		t.Fatalf("ToJWK: %v", err)
	}
	if key == nil {
		t.Fatal("ToJWK returned nil key")
	}

	// JSON エンコードしてフィールドを確認 (jwx の Get メソッドは型 assert が必要なため、
	// JSON 経由で文字列値を統一して assert する)。
	body, err := json.Marshal(key)
	if err != nil {
		t.Fatalf("json.Marshal(key): %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("Unmarshal: %v\nbody=%s", err, body)
	}

	cases := []struct {
		field string
		want  string
	}{
		{field: "kty", want: "RSA"},
		{field: "use", want: "sig"},
		{field: "alg", want: "RS256"},
		{field: "kid", want: kp.Kid},
	}
	for _, tc := range cases {
		v, ok := got[tc.field].(string)
		if !ok {
			t.Errorf("field %q missing or not string in JWK JSON: got=%v\nbody=%s", tc.field, got[tc.field], body)
			continue
		}
		if v != tc.want {
			t.Errorf("field %q = %q, want %q", tc.field, v, tc.want)
		}
	}

	// 公開鍵成分: n と e が出力されている (RSA 鍵として有効)
	if _, ok := got["n"].(string); !ok {
		t.Errorf("RSA modulus 'n' missing in JWK JSON: %s", body)
	}
	if _, ok := got["e"].(string); !ok {
		t.Errorf("RSA exponent 'e' missing in JWK JSON: %s", body)
	}
}

// private 成分 (d, p, q, dp, dq, qi) は出力されない (Codex LOW 2、F-18)。
func TestToJWK_NoPrivateComponents(t *testing.T) {
	kp := newTestKeyPair(t, 2048)
	key, err := jwks.ToJWK(kp)
	if err != nil {
		t.Fatalf("ToJWK: %v", err)
	}

	body, err := json.Marshal(key)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	for _, forbidden := range []string{"d", "p", "q", "dp", "dq", "qi"} {
		if _, exists := got[forbidden]; exists {
			t.Errorf("private component %q must not be present in public JWK: body=%s", forbidden, body)
		}
	}
}

// 同一公開鍵から繰り返し変換しても結果が等価 (kid 決定論性 + Set の冪等性)。
func TestToJWK_StableAcrossCalls(t *testing.T) {
	kp := newTestKeyPair(t, 2048)
	first, err := jwks.ToJWK(kp)
	if err != nil {
		t.Fatalf("ToJWK first: %v", err)
	}
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("Marshal first: %v", err)
	}

	for i := 0; i < 10; i++ {
		got, err := jwks.ToJWK(kp)
		if err != nil {
			t.Fatalf("ToJWK #%d: %v", i, err)
		}
		gotJSON, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("Marshal #%d: %v", i, err)
		}
		if string(gotJSON) != string(firstJSON) {
			t.Fatalf("ToJWK 結果が安定していない (call %d):\nfirst = %s\ngot   = %s", i, firstJSON, gotJSON)
		}
	}
}

// nil-safe: KeyPair / PublicKey が nil の場合は明示エラー。
func TestToJWK_NilSafe(t *testing.T) {
	cases := []struct {
		name string
		kp   *keystore.KeyPair
	}{
		{name: "nil kp", kp: nil},
		{name: "nil PublicKey", kp: &keystore.KeyPair{Kid: "x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := jwks.ToJWK(tc.kp)
			if err == nil {
				t.Errorf("ToJWK should return error when input is nil-ish")
			}
		})
	}
}

// jwk.Key の型安全性: 戻り値は jwk.Key インターフェースを満たす。
func TestToJWK_ReturnsJWKKey(t *testing.T) {
	kp := newTestKeyPair(t, 2048)
	got, err := jwks.ToJWK(kp)
	if err != nil {
		t.Fatalf("ToJWK: %v", err)
	}
	var _ jwk.Key = got // compile-time assertion
}
