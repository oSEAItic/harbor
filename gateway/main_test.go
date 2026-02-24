package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthorizeAPIKey(t *testing.T) {
	cfg := gatewayConfig{
		APIKeys: map[string]struct{}{"secret": {}},
	}
	got, err := authorize(dummyRequest("secret"), cfg)
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if got.Type != "api_key" {
		t.Fatalf("type=%q want api_key", got.Type)
	}
}

func TestAuthorizeJWT(t *testing.T) {
	claims := map[string]interface{}{
		"sub": "alice",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}
	signed, err := signHS256(claims, "test-secret")
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}

	cfg := gatewayConfig{JWTSecret: "test-secret"}
	got, err := authorize(dummyBearerRequest(signed), cfg)
	if err != nil {
		t.Fatalf("authorize jwt: %v", err)
	}
	if got.Actor != "jwt:alice" {
		t.Fatalf("actor=%q want jwt:alice", got.Actor)
	}
}

func TestRateLimiter(t *testing.T) {
	rl := newRateLimiter(2)
	if !rl.Allow("u1:c1") {
		t.Fatal("first request should pass")
	}
	if !rl.Allow("u1:c1") {
		t.Fatal("second request should pass")
	}
	if rl.Allow("u1:c1") {
		t.Fatal("third request should be rate limited")
	}
}

func dummyRequest(apiKey string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/run", nil)
	r.Header.Set("X-API-Key", apiKey)
	return r
}

func dummyBearerRequest(token string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/run", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	return r
}

func signHS256(claims map[string]interface{}, secret string) (string, error) {
	header := map[string]interface{}{
		"alg": "HS256",
		"typ": "JWT",
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	h := base64.RawURLEncoding.EncodeToString(headerJSON)
	p := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signing := h + "." + p

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signing))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signing + "." + sig, nil
}
