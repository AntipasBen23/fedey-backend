package linkedinaccounts

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	platform "github.com/AntipasBen23/fedey-backend/internal/platform/linkedin"
)

type Service struct {
	repository   Repository
	client       *platform.Client
	clientID     string
	clientSecret string
	redirectURI  string
	webAppURL    string
}

func NewService(repository Repository, client *platform.Client, clientID, clientSecret, redirectURI, webAppURL string) *Service {
	return &Service{
		repository:   repository,
		client:       client,
		clientID:     strings.TrimSpace(clientID),
		clientSecret: strings.TrimSpace(clientSecret),
		redirectURI:  strings.TrimSpace(redirectURI),
		webAppURL:    strings.TrimSpace(webAppURL),
	}
}

func (s *Service) GetActive(ctx context.Context) (Account, error) {
	account, err := s.repository.GetActive(ctx)
	if err != nil {
		return Account{}, err
	}

	if !account.ExpiresAt.IsZero() && time.Now().UTC().After(account.ExpiresAt) {
		return Account{}, ErrAccountNotConnected
	}

	return account, nil
}

func (s *Service) StartAuth(ctx context.Context) (string, error) {
	state, err := randomString(32)
	if err != nil {
		return "", err
	}

	if err := s.repository.SaveState(ctx, OAuthState{
		State:     state,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		return "", err
	}

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", s.clientID)
	params.Set("redirect_uri", s.redirectURI)
	params.Set("state", state)
	params.Set("scope", "r_liteprofile w_member_social")

	return "https://www.linkedin.com/oauth/v2/authorization?" + params.Encode(), nil
}

func (s *Service) HandleCallback(ctx context.Context, code, state string) (string, error) {
	if _, err := s.repository.GetState(ctx, strings.TrimSpace(state)); err != nil {
		return "", err
	}
	defer s.repository.DeleteState(ctx, state)

	tokenResponse, err := s.client.ExchangeCode(ctx, s.clientID, s.clientSecret, s.redirectURI, code)
	if err != nil {
		return "", err
	}

	member, err := s.client.GetMember(ctx, tokenResponse.AccessToken)
	if err != nil {
		return "", err
	}

	displayName := strings.TrimSpace(strings.TrimSpace(member.FirstName) + " " + strings.TrimSpace(member.LastName))
	if displayName == "" {
		displayName = member.ID
	}

	account := Account{
		ID:           "linkedin-active",
		Provider:     "linkedin",
		MemberID:     member.ID,
		DisplayName:  displayName,
		AuthorURN:    "urn:li:person:" + member.ID,
		AccessToken:  tokenResponse.AccessToken,
		RefreshToken: tokenResponse.RefreshToken,
		Scopes:       strings.Fields(tokenResponse.Scope),
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
