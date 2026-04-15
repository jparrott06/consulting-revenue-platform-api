package auth

import (
	"testing"
	"time"
)

func TestAccessTokenRoundTrip(t *testing.T) {
	key := []byte("test-secret-key-for-jwt-signing")
	userID := "11111111-1111-1111-1111-111111111111"
	sessionID := "22222222-2222-2222-2222-222222222222"

	tok, err := IssueAccessToken(key, userID, sessionID, time.Minute)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	gotUser, gotSession, err := ParseAccessToken(key, tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if gotUser != userID || gotSession != sessionID {
		t.Fatalf("unexpected claims: user=%s session=%s", gotUser, gotSession)
	}
}
