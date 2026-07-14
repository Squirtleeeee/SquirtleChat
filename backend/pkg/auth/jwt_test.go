package auth

import (
	"testing"
	"time"
)

func TestRefreshRoundTrip(t *testing.T) {
	secret := "test-secret"
	refresh, err := SignRefresh(secret, 42, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	uid, err := ParseRefresh(secret, refresh)
	if err != nil {
		t.Fatal(err)
	}
	if uid != 42 {
		t.Fatalf("expected 42, got %d", uid)
	}
}

func TestAccessRoundTrip(t *testing.T) {
	secret := "test-secret"
	access, err := SignAccess(secret, 7, "alice", "dev1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseAccess(secret, access)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != 7 || claims.Username != "alice" || claims.DeviceID != "dev1" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}
