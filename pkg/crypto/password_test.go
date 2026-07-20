package crypto

import "testing"

func TestHashAndCheck(t *testing.T) {
	hash, err := HashPassword("s3cr3t-pass")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "s3cr3t-pass" {
		t.Fatal("hash must not equal plaintext")
	}
	if !CheckPassword(hash, "s3cr3t-pass") {
		t.Fatal("correct password should verify")
	}
	if CheckPassword(hash, "wrong") {
		t.Fatal("wrong password must not verify")
	}
}
