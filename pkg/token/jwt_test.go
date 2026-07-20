package token

import (
	"testing"
	"time"
)

func TestIssueAndVerify(t *testing.T) {
	m := NewManager("test-secret", time.Hour)
	tok, exp, err := m.Issue(42, "admin")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if exp.Before(time.Now()) {
		t.Fatal("expiry should be in the future")
	}

	claims, err := m.Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.AdminID != 42 || claims.Username != "admin" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestVerifyRejectsTampered(t *testing.T) {
	m := NewManager("secret-a", time.Hour)
	tok, _, _ := m.Issue(1, "admin")

	other := NewManager("secret-b", time.Hour)
	if _, err := other.Verify(tok); err == nil {
		t.Fatal("expected verification failure for wrong secret")
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	m := NewManager("secret", -time.Minute) // already expired
	tok, _, _ := m.Issue(1, "admin")
	if _, err := m.Verify(tok); err == nil {
		t.Fatal("expected expired token to fail verification")
	}
}
