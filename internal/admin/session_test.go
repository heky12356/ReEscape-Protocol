package admin

import (
	"testing"
	"time"
)

func TestSessionStoreExpiresAndRevokesTokens(t *testing.T) {
	store := newSessionStore()
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	token, expiresAt, err := store.create(now, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if len(token) < 40 || !expiresAt.After(now) {
		t.Fatalf("unexpected session token or expiry: token=%q expires=%v", token, expiresAt)
	}
	if !store.valid(token, now.Add(time.Hour), "secret") {
		t.Fatal("expected session to be valid before expiry")
	}
	if store.valid(token, now.Add(time.Hour), "rotated-secret") {
		t.Fatal("expected session to be invalid after secret rotation")
	}
	if store.valid(token, expiresAt, "secret") {
		t.Fatal("expected session to expire")
	}

	token, _, err = store.create(now, "secret")
	if err != nil {
		t.Fatal(err)
	}
	store.revoke(token)
	if store.valid(token, now, "secret") {
		t.Fatal("expected revoked session to be invalid")
	}
}
