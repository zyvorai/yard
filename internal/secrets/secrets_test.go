package secrets

import "testing"

func TestEncryptRoundTrip(t *testing.T) {
	key, err := RandomKey()
	if err != nil {
		t.Fatal(err)
	}
	blob, err := Encrypt(key, "hello-secret")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decrypt(key, blob)
	if err != nil || got != "hello-secret" {
		t.Fatalf("got %q err %v", got, err)
	}
}
