package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

var (
	privateKey *rsa.PrivateKey
	signer     jose.Signer
	jwkSet     jose.JSONWebKeySet
)

func init() {
	var err error
	// Generate RSA key pair
	privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("Failed to generate private key: %v", err)
	}

	// Create JSON Web Key
	key := jose.JSONWebKey{
		Key:       &privateKey.PublicKey,
		KeyID:     "test-key-id",
		Algorithm: "RS256",
		Use:       "sig",
	}

	jwkSet = jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{key},
	}

	// Create signer
	opts := (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test-key-id")
	signer, err = jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: privateKey}, opts)
	if err != nil {
		log.Fatalf("Failed to create signer: %v", err)
	}
}

func jwksHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jwkSet)
}

func tokenHandler(w http.ResponseWriter, r *http.Request) {
	issuer := "http://mock-jwks.default.svc.cluster.local:8080/token"

	// Check if issuer is overridden via query param (optional, for flexibility)
	if iss := r.URL.Query().Get("issuer"); iss != "" {
		issuer = iss
	}

	claims := jwt.Claims{
		Subject:   "test-user",
		Issuer:    issuer,
		NotBefore: jwt.NewNumericDate(time.Now()),
		Expiry:    jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		Audience:  jwt.Audience{"test-audience"},
	}

	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to sign token: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(raw))
}

func main() {
	http.HandleFunc("/jwks", jwksHandler)
	http.HandleFunc("/token", tokenHandler)

	log.Println("Mock JWKS server listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
