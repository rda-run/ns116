package oidc

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"ns116/internal/config"
)

const (
	requestCookieName = "ns116_oidc_req"
	requestMaxAge     = 10 * time.Minute
)

type ProviderButton struct {
	Name  string
	Label string
}

type Service struct {
	secret    []byte
	providers map[string]config.OIDCProviderConfig
}

func NewService(sessionSecret string, providers map[string]config.OIDCProviderConfig) *Service {
	return &Service{
		secret:    []byte(sessionSecret),
		providers: providers,
	}
}

func (s *Service) ProviderButtons() []ProviderButton {
	out := make([]ProviderButton, 0, len(s.providers))
	for name, p := range s.providers {
		label := p.Label
		if strings.TrimSpace(label) == "" {
			label = name
		}
		out = append(out, ProviderButton{Name: name, Label: label})
	}
	return out
}

func (s *Service) ProviderConfig(name string) (config.OIDCProviderConfig, bool) {
	p, ok := s.providers[name]
	return p, ok
}

type authRequest struct {
	Provider     string `json:"provider"`
	State        string `json:"state"`
	Nonce        string `json:"nonce"`
	PKCEVerifier string `json:"pkce_verifier"`
	CreatedAtUTC int64  `json:"created_at_utc"`
}

func (s *Service) StartAuth(w http.ResponseWriter, r *http.Request, providerName string) (string, error) {
	p, ok := s.providers[providerName]
	if !ok {
		return "", fmt.Errorf("unknown OIDC provider")
	}

	state, err := randHex(32)
	if err != nil {
		return "", err
	}
	nonce, err := randHex(32)
	if err != nil {
		return "", err
	}
	verifier, err := randHex(64)
	if err != nil {
		return "", err
	}
	challenge := pkceS256(verifier)

	provider, err := oidc.NewProvider(r.Context(), p.IssuerURL)
	if err != nil {
		return "", fmt.Errorf("failed to discover provider: %w", err)
	}

	oauthCfg := oauth2.Config{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  p.RedirectURL,
		Scopes:       p.Scopes,
	}

	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	}
	for k, v := range p.ExtraAuthParams {
		if strings.TrimSpace(k) == "" {
			continue
		}
		opts = append(opts, oauth2.SetAuthURLParam(k, v))
	}

	ar := authRequest{
		Provider:     providerName,
		State:        state,
		Nonce:        nonce,
		PKCEVerifier: verifier,
		CreatedAtUTC: time.Now().UTC().Unix(),
	}
	if err := s.setRequestCookie(w, &ar); err != nil {
		return "", err
	}

	return oauthCfg.AuthCodeURL(state, opts...), nil
}

type VerifiedIdentity struct {
	Provider string
	Username string
	Claims   map[string]any
}

func (s *Service) HandleCallback(ctx context.Context, r *http.Request) (*VerifiedIdentity, error) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		return nil, errors.New("missing code/state")
	}

	ar, err := s.getRequestCookie(r)
	if err != nil {
		return nil, err
	}
	if ar.State != state {
		return nil, errors.New("invalid state")
	}
	created := time.Unix(ar.CreatedAtUTC, 0)
	if time.Since(created) > requestMaxAge {
		return nil, errors.New("expired login request")
	}

	p, ok := s.providers[ar.Provider]
	if !ok {
		return nil, errors.New("unknown provider")
	}

	provider, err := oidc.NewProvider(ctx, p.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to discover provider: %w", err)
	}

	oauthCfg := oauth2.Config{
		ClientID:     p.ClientID,
		ClientSecret: p.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  p.RedirectURL,
		Scopes:       p.Scopes,
	}

	token, err := oauthCfg.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", ar.PKCEVerifier))
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, errors.New("missing id_token")
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: p.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("id_token verification failed: %w", err)
	}
	if idToken.Nonce != ar.Nonce {
		return nil, errors.New("invalid nonce")
	}

	claims := map[string]any{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	username, err := claimString(claims, p.UsernameClaim)
	if err != nil || strings.TrimSpace(username) == "" {
		return nil, fmt.Errorf("missing username claim %q", p.UsernameClaim)
	}

	return &VerifiedIdentity{
		Provider: ar.Provider,
		Username: username,
		Claims:   claims,
	}, nil
}

func (s *Service) ClearRequestCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     requestCookieName,
		Value:    "",
		Path:     "/oidc/callback",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (s *Service) setRequestCookie(w http.ResponseWriter, ar *authRequest) error {
	b, err := json.Marshal(ar)
	if err != nil {
		return err
	}
	payload := base64.RawURLEncoding.EncodeToString(b)
	sig := s.sign(payload)
	val := payload + "." + sig

	http.SetCookie(w, &http.Cookie{
		Name:     requestCookieName,
		Value:    val,
		Path:     "/oidc/callback",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(requestMaxAge.Seconds()),
	})
	return nil
}

func (s *Service) getRequestCookie(r *http.Request) (*authRequest, error) {
	c, err := r.Cookie(requestCookieName)
	if err != nil {
		return nil, errors.New("missing login request cookie")
	}
	parts := strings.Split(c.Value, ".")
	if len(parts) != 2 {
		return nil, errors.New("invalid login request cookie")
	}
	payload, sig := parts[0], parts[1]
	if !hmac.Equal([]byte(sig), []byte(s.sign(payload))) {
		return nil, errors.New("invalid login request signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, errors.New("invalid login request encoding")
	}
	var ar authRequest
	if err := json.Unmarshal(raw, &ar); err != nil {
		return nil, errors.New("invalid login request payload")
	}
	return &ar, nil
}

func (s *Service) sign(payload string) string {
	m := hmac.New(sha256.New, s.secret)
	m.Write([]byte(payload))
	return hex.EncodeToString(m.Sum(nil))
}

func randHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func pkceS256(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func claimString(claims map[string]any, key string) (string, error) {
	if key == "" {
		return "", errors.New("empty claim key")
	}

	// Support dotted paths like "user.preferred_username"
	cur := any(claims)
	for _, part := range strings.Split(key, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", fmt.Errorf("claim %q not found", key)
		}
		cur, ok = m[part]
		if !ok {
			return "", fmt.Errorf("claim %q not found", key)
		}
	}

	switch v := cur.(type) {
	case string:
		return v, nil
	case fmt.Stringer:
		return v.String(), nil
	default:
		return "", fmt.Errorf("claim %q is not a string", key)
	}
}

func claimStrings(claims map[string]any, key string) ([]string, error) {
	if key == "" {
		return nil, errors.New("empty claim key")
	}

	cur := any(claims)
	for _, part := range strings.Split(key, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("claim %q not found", key)
		}
		cur, ok = m[part]
		if !ok {
			return nil, fmt.Errorf("claim %q not found", key)
		}
	}

	switch v := cur.(type) {
	case []string:
		return v, nil
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			if strings.TrimSpace(s) == "" {
				continue
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("claim %q is not a string array", key)
	}
}

func RedirectToLoginWithError(w http.ResponseWriter, r *http.Request, msg string) {
	q := url.Values{}
	q.Set("error", msg)
	http.Redirect(w, r, "/login?"+q.Encode(), http.StatusSeeOther)
}
