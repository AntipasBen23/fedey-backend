package xaccounts

import "time"

type Account struct {
	ID           string    `json:"id"`
	Provider     string    `json:"provider"`
	UserID       string    `json:"userId"`
	Username     string    `json:"username"`
	AccessToken  string    `json:"-"`
	RefreshToken string    `json:"-"`
	Scopes       []string  `json:"scopes"`
	TokenType    string    `json:"tokenType"`
	ExpiresAt    time.Time `json:"expiresAt"`
	ConnectedAt  time.Time `json:"connectedAt"`
}

type OAuthState struct {
	State        string
	CodeVerifier string
	CreatedAt    time.Time
}
