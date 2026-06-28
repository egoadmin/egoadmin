//go:build e2e

package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"testing"
	"time"
)

const e2eUA = "egoadmin-e2e"

func (e *environment) loginAsAdmin(t *testing.T) loginResponse {
	t.Helper()

	req := e.buildLoginRequest(t, "admin", "123456", "login")
	var out loginResponse
	e.postJSON(t, "/api/user.v1.UserService/Login", req, "", &out)
	return out
}

func (e *environment) buildLoginRequest(t *testing.T, username, password, action string) map[string]any {
	t.Helper()

	if action == "" {
		action = "login"
	}
	var crypto loginCryptoResponse
	e.postJSON(t, "/api/user.v1.UserService/GetLoginCrypto", map[string]any{
		"username": username,
		"ua":       e2eUA,
		"action":   action,
	}, "", &crypto)

	if crypto.Algorithm != "RSA-OAEP-SHA256" {
		t.Fatalf("login crypto algorithm = %q, want RSA-OAEP-SHA256", crypto.Algorithm)
	}
	if crypto.KeyID == "" || crypto.PublicKey == "" || crypto.ChallengeID == "" || crypto.Nonce == "" {
		t.Fatalf("login crypto response missing required fields: %#v", crypto)
	}

	payload := map[string]any{
		"username":    username,
		"password":    password,
		"challengeId": crypto.ChallengeID,
		"nonce":       crypto.Nonce,
		"timestamp":   time.Now().UnixMilli(),
		"ua":          e2eUA,
		"action":      action,
	}
	cipher := encryptLoginPayload(t, crypto.PublicKey, payload)
	return map[string]any{
		"username":       username,
		"ua":             e2eUA,
		"passwordCipher": cipher,
		"keyId":          crypto.KeyID,
		"challengeId":    crypto.ChallengeID,
	}
}

func (e *environment) buildCryptoRequest(t *testing.T, username, action string, payload map[string]any) map[string]any {
	t.Helper()

	if action == "" {
		action = "login"
	}
	var crypto loginCryptoResponse
	e.postJSON(t, "/api/user.v1.UserService/GetLoginCrypto", map[string]any{
		"username": username,
		"ua":       e2eUA,
		"action":   action,
	}, "", &crypto)

	if crypto.Algorithm != "RSA-OAEP-SHA256" {
		t.Fatalf("login crypto algorithm = %q, want RSA-OAEP-SHA256", crypto.Algorithm)
	}
	if crypto.KeyID == "" || crypto.PublicKey == "" || crypto.ChallengeID == "" || crypto.Nonce == "" {
		t.Fatalf("login crypto response missing required fields: %#v", crypto)
	}

	payload["username"] = username
	payload["challengeId"] = crypto.ChallengeID
	payload["nonce"] = crypto.Nonce
	payload["timestamp"] = time.Now().UnixMilli()
	payload["ua"] = e2eUA
	payload["action"] = action

	return map[string]any{
		"passwordCipher": encryptLoginPayload(t, crypto.PublicKey, payload),
		"keyId":          crypto.KeyID,
		"challengeId":    crypto.ChallengeID,
	}
}

func encryptLoginPayload(t *testing.T, publicKeyPEM string, payload any) string {
	t.Helper()

	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		t.Fatal("decode public key PEM: no PEM block")
	}
	pubAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatalf("parse public key: %v", err)
	}
	pub, ok := pubAny.(*rsa.PublicKey)
	if !ok {
		t.Fatalf("public key type = %T, want *rsa.PublicKey", pubAny)
	}

	plain, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal login payload: %v", err)
	}
	cipher, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, plain, nil)
	if err != nil {
		t.Fatalf("encrypt login payload: %v", err)
	}
	return base64.StdEncoding.EncodeToString(cipher)
}
