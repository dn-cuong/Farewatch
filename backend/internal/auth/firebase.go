package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const firebaseCertURL = "https://www.googleapis.com/robot/v1/metadata/x509/securetoken@system.gserviceaccount.com"

type FirebaseIdentity struct {
	UID   string
	Email string
	Name  string
}

type firebaseCertCache struct {
	mu    sync.Mutex
	certs map[string]*rsa.PublicKey
	until time.Time
}

var firebaseCerts firebaseCertCache

func (s *Service) VerifyFirebaseToken(ctx context.Context, idToken, projectID string) (*FirebaseIdentity, error) {
	if projectID == "" {
		return nil, errors.New("firebase project id not configured")
	}
	keys, err := loadFirebaseKeys(ctx)
	if err != nil {
		return nil, err
	}

	parsed, err := jwt.Parse(idToken, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != "RS256" {
			return nil, fmt.Errorf("unexpected alg %s", t.Method.Alg())
		}
		kid, _ := t.Header["kid"].(string)
		key, ok := keys[kid]
		if !ok {
			return nil, errors.New("unknown firebase kid")
		}
		return key, nil
	}, jwt.WithAudience(projectID), jwt.WithIssuer("https://securetoken.google.com/"+projectID))
	if err != nil {
		return nil, fmt.Errorf("invalid firebase token: %w", err)
	}
	if !parsed.Valid {
		return nil, errors.New("invalid firebase token")
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid firebase claims")
	}
	uid, _ := claims["user_id"].(string)
	if uid == "" {
		uid, _ = claims["sub"].(string)
	}
	email, _ := claims["email"].(string)
	name, _ := claims["name"].(string)
	if uid == "" {
		return nil, errors.New("firebase token missing user id")
	}
	if email == "" {
		return nil, errors.New("firebase token missing email")
	}
	if name == "" {
		name = email
	}
	return &FirebaseIdentity{UID: uid, Email: email, Name: name}, nil
}

func loadFirebaseKeys(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	firebaseCerts.mu.Lock()
	defer firebaseCerts.mu.Unlock()
	if firebaseCerts.certs != nil && time.Now().Before(firebaseCerts.until) {
		return firebaseCerts.certs, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, firebaseCertURL, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("firebase certs status %d", res.StatusCode)
	}
	var raw map[string]string
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, err
	}
	out := make(map[string]*rsa.PublicKey, len(raw))
	for kid, pemStr := range raw {
		block, _ := pem.Decode([]byte(pemStr))
		if block == nil {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		pub, ok := cert.PublicKey.(*rsa.PublicKey)
		if ok {
			out[kid] = pub
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no firebase certs parsed")
	}
	firebaseCerts.certs = out
	firebaseCerts.until = time.Now().Add(6 * time.Hour)
	return out, nil
}
