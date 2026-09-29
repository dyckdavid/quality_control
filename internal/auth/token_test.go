package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "correct horse battery") {
		t.Fatal("expected match")
	}
	if CheckPassword(hash, "wrong horse battery") {
		t.Fatal("expected mismatch")
	}
}

func TestAccessTokenRoundTrip(t *testing.T) {
	m := NewManager("0123456789abcdef0123456789abcdef", 15*time.Minute, 24*time.Hour)
	id := uuid.New()
	now := time.Now().UTC()
	signed, err := m.SignAccess(id, "ada", now)
	if err != nil {
		t.Fatal(err)
	}
	gotID, name, err := m.ParseAccess(signed, now.Add(time.Minute))
	if err != nil || gotID != id || name != "ada" {
		t.Fatalf("parse: %s %s %v", gotID, name, err)
	}
	if _, _, err := m.ParseAccess(signed, now.Add(time.Hour)); err == nil {
		t.Fatal("expected expired token to fail")
	}
	if _, _, err := m.ParseAccess(signed+"x", now); err == nil {
		t.Fatal("expected tampered token to fail")
	}
}
