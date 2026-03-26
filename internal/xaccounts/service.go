package xaccounts

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	xplatform "github.com/AntipasBen23/fedey-backend/internal/platform/x"
)

type Service struct {
	repository  Repository
	xClient     *xplatform.Client
	clientID    string
	redirectURI string
	webAppURL   string
}

func NewService(repository Repository, xClient *xplatform.Client, clientID, redirectURI, webAppURL string) *Service {
	return &Service{
		repository:  repository,
		xClient:     xClient,
		clientID:    strings.TrimSpace(clientID),
		redirectURI: strings.TrimSpace(redirectURI),
		webAppURL:   strings.TrimSpace(webAppURL),
	}
}

func (s *Service) GetActive(ctx context.Context) (Account, error) {
	account, err := s.repository.GetActive(ctx)
	if err != nil {
		return Account{}, err
	}

	return s.ensureFreshToken(ctx, account)
}

func (s *Service) StartAuth(ctx context.Context) (string, error) {
	state, err := randomString(32)
	if err != nil {
		return "", err
	}
	verifier, err := randomString(64)
	if err != nil {
		return "", err
	}

	if err := s.repository.SaveState(ctx, OAuthState{
		State:        state,
		CodeVerifier: verifier,
		CreatedAt:    time.Now().UTC(),
	}); err != nil {
		return "", err
	}

	challenge := codeChallenge(verifier)
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", s.clientID)
	params.Set("redirect_uri", s.redirectURI)
	params.Set("scope", "tweet.read users.read tweet.write offline.access")
	params.Set("state", state)
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256")

	return "https://x.com/i/oauth2/authorize?" + params.Encode(), nil
}

func (s *Service) HandleCallback(ctx context.Context, code, state string) (string, error) {
	savedState, err := s.repository.GetState(ctx, strings.TrimSpace(state))
	if err != nil {
		return "", err
	}
	defer s.repository.DeleteState(ctx, state)

	tokenResponse, err := s.xClient.ExchangeCode(ctx, s.clientID, s.redirectURI, code, savedState.CodeVerifier)
	if err != nil {
		return "", err
	}

	user, err := s.xClient.GetAuthenticatedUser(ctx, tokenResponse.AccessToken)
	if err != nil {
		return "", err
	}

	account := Account{
		ID:           "x-active",
		Provider:     "x",
		UserID:       user.ID,
		Username:     user.Username,
		AccessToken:  tokenResponse.AccessToken,
		RefreshToken: tokenResponse.RefreshToken,
		Scopes:       strings.Fields(tokenResponse.Scope),
		TokenType:    tokenResponse.TokenType,
		ExpiresAt:    time.Now().UTC().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second),
		ConnectedAt:  time.Now().UTC(),
	}

	if err := s.repository.UpsertActive(ctx, account); err != nil {
		return "", err
	}

	if strings.TrimSpace(s.webAppURL) == "" {
		return "/", nil
	}

	return s.webAppURL, nil
}

func randomString(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate random string: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func codeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func (s *Service) ensureFreshToken(ctx context.Context, account Account) (Account, error) {
	if s.xClient == nil {
		return account, nil
	}
	if time.Until(account.ExpiresAt) > 5*time.Minute {
		return account, nil
	}
	if strings.TrimSpace(account.RefreshToken) == "" || strings.TrimSpace(s.clientID) == "" {
		return account, nil
	}

	tokenResponse, err := s.xClient.RefreshToken(ctx, s.clientID, account.RefreshToken)
	if err != nil {
		return Account{}, err
	}

	account.AccessToken = tokenResponse.AccessToken
	if strings.TrimSpace(tokenResponse.RefreshToken) != "" {
		account.RefreshToken = tokenResponse.RefreshToken
	}
	account.TokenType = tokenResponse.TokenType
	if scopes := strings.Fields(tokenResponse.Scope); len(scopes) > 0 {
		account.Scopes = scopes
	}
	if tokenResponse.ExpiresIn > 0 {
		account.ExpiresAt = time.Now().UTC().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)
	}

	if err := s.repository.UpsertActive(ctx, account); err != nil {
		return Account{}, err
	}

	return account, nil
}
