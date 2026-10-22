package crypto

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ide-digital/nulltrace/internal/nterr"
)

func TestEnvelopeRoundTrip(t *testing.T) {
	env, dek, err := InitEnvelope([]byte("correct horse"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := OpenEnvelope([]byte("correct horse"), env)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, dek) {
		t.Fatal("reopened DEK does not match")
	}
	if _, err := OpenEnvelope([]byte("wrong horse"), env); !errors.Is(err, nterr.ErrInvalidPassphrase) {
		t.Fatalf("wrong passphrase: got %v", err)
	}
}

func TestBlobIsBoundToItsRow(t *testing.T) {
	key, err := NewDEK()
	if err != nil {
		t.Fatal(err)
	}
	blob, err := Encrypt(key, []byte("jane doe"), AAD("identities", "first_name_enc", "a"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := Decrypt(key, blob, AAD("identities", "first_name_enc", "a"))
	if err != nil || string(plain) != "jane doe" {
		t.Fatalf("decrypt: %q %v", plain, err)
	}
	if _, err := Decrypt(key, blob, AAD("identities", "first_name_enc", "b")); err == nil {
		t.Fatal("blob copied to another row should not decrypt")
	}
}
