package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type saKey struct {
	KeyID  string `json:"keyId"`
	Key    string `json:"key"`
	UserID string `json:"userId"`
}

func parseKey(pemText string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemText))
	if block == nil {
		return nil, fmt.Errorf("invalid pem")
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	k, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not rsa key")
	}
	return k, nil
}

func main() {
	domain := "https://backend-dev-test-udligj.eu1.zitadel.cloud"
	raw, err := os.ReadFile("/home/meghdad/backend-dev-challenge/api-backend.json")
	if err != nil {
		log.Fatal(err)
	}

	var key saKey
	if err := json.Unmarshal(raw, &key); err != nil {
		log.Fatal(err)
	}

	privateKey, err := parseKey(key.Key)
	if err != nil {
		log.Fatal(err)
	}

	now := time.Now().UTC()
	claims := jwt.RegisteredClaims{
		Issuer:    key.UserID,
		Subject:   key.UserID,
		Audience:  jwt.ClaimStrings{domain},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		ID:        uuid.NewString(),
	}

	t := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	t.Header["kid"] = key.KeyID
	assertion, err := t.SignedString(privateKey)
	if err != nil {
		log.Fatal(err)
	}

	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", assertion)
	form.Set("scope", "openid profile urn:zitadel:iam:org:project:id:zitadel:aud")

	resp, err := http.PostForm(strings.TrimRight(domain, "/")+"/oauth/v2/token", form)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		log.Fatal(err)
	}

	pretty, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(pretty))
}
