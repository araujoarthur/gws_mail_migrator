package impersonator

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	RenewalAdvantageWindow = 5 // minutes
)

type TokenManager struct {
	mu          sync.RWMutex
	accessToken string
	expiry      time.Time
	renewing    bool
	targetUser  string
	scopes      string
	renewChan   chan struct{} // channel used to broadcast renewal signals
	credentials SACredentials
}

func NewTokenManager() *TokenManager {
	return &TokenManager{
		renewChan: make(chan struct{}),
	}
}

func (tm *TokenManager) GetValidToken(ctx context.Context) (string, error) {
	tm.mu.RLock()

	if time.Now().Before(tm.expiry.Add(-RenewalAdvantageWindow * time.Minute)) {
		token := tm.accessToken
		tm.mu.RUnlock()
		return token, nil
	}

	tm.mu.RUnlock()

	// Acquire total lock
	tm.mu.Lock()

	// Recheck after acquiring the lock to prevent competition
	if time.Now().Before(tm.expiry.Add(-RenewalAdvantageWindow * time.Minute)) {
		token := tm.accessToken
		tm.mu.Unlock()
		return token, nil
	}

	// From here, the token was about to expiry even after the lock acquisition

	if tm.renewing {
		// another go routine is already refreshing, so we wait for it to finish
		ch := tm.renewChan
		tm.mu.Unlock()
		select {
		case <-ch:
			// Renewal finished, fetch the new token recursively or read it
			return tm.GetValidToken(ctx)
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	// From here, there was no routine refreshing, so it's this one's job.
	tm.renewing = true
	currentRenewChan := tm.renewChan
	tm.mu.Unlock()

	newToken, newExpiry, err := tm.fetchNewTokenFromAPI(ctx, tm.targetUser, tm.scopes)
	tm.mu.Lock()
	if err != nil {
		return "", err
	}

	tm.accessToken = newToken
	tm.expiry = newExpiry
	tm.renewing = false

	// Create a new channel for future waiters and close the old one to broadcast to current waiters
	tm.renewChan = make(chan struct{})
	close(currentRenewChan)

	tm.mu.Unlock()
	return newToken, nil
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type jwtClaims struct {
	Iss   string `json:"iss"`
	Sub   string `json:"sub"`
	Scope string `json:"scope"`
	Aud   string `json:"aud"`
	Exp   int64  `json:"exp"`
	Iat   int64  `json:"iat"`
}

func (tm *TokenManager) fetchNewTokenFromAPI(ctx context.Context, target string, scopes string) (string, time.Time, error) {

	block, _ := pem.Decode([]byte(tm.credentials.PrivateKey))
	if block == nil {
		return "", time.Time{}, errors.New("failed to decode PEM block containing the private key")
	}

	privKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {

		privKeyPKCS1, errPKCS1 := x509.ParsePKCS1PrivateKey(block.Bytes)
		if errPKCS1 != nil {
			return "", time.Time{}, fmt.Errorf("failed to parse private key (PKCS1 & PKCS8 failed): %w - %w", err, errPKCS1)
		}

		privKey = privKeyPKCS1

		return "", time.Time{}, errors.New("failed to parse private key")
	}

	rsaPrivateKey, ok := privKey.(*rsa.PrivateKey)
	if !ok {
		return "", time.Time{}, errors.New("not an RSA private key")
	}

	now := time.Now()
	expiry := now.Add(1 * time.Hour)

	// building jwt header and claims
	headerBytes, _ := json.Marshal(jwtHeader{Alg: "RS256", Typ: "JWT"})
	claimsBytes, _ := json.Marshal(jwtClaims{
		Iss:   tm.credentials.ClientEmail,
		Sub:   target,
		Scope: scopes,
		Aud:   "https://oauth2.googleapis.com/token",
		Exp:   expiry.Unix(),
		Iat:   now.Unix(),
	})

	encodedHeader := base64URLEncode(headerBytes)
	encodedClaims := base64URLEncode(claimsBytes)

	signingInput := encodedHeader + "." + encodedClaims

	// Signing the input using RS256 (SHA256 + RSA PKCS1v15)
	hashed := sha256Sum([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, rsaPrivateKey, crypto.SHA256, hashed)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign JWT: %w", err)
	}

	encodedSignature := base64URLEncode(signature)
	signedJWT := signingInput + "." + encodedSignature

	// Exchanging the signed JWT for an oauth2.0 access token
	data := url.Values{}
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	data.Set("assertion", signedJWT)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", strings.NewReader(data.Encode()))
	if err != nil {
		return "", time.Time{}, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("token endpoint request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("token endpoint returned non-200 status: %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", time.Time{}, fmt.Errorf("failed to decode token response: %w", err)
	}

	return tokenResp.AccessToken, now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second), nil
}

func (tm *TokenManager) Expiry() time.Time {
	return tm.expiry
}
func base64URLEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

func sha256Sum(data []byte) []byte {
	h := sha256.New()
	h.Write(data)
	return h.Sum(nil)
}
