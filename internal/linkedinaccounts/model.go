package linkedinaccounts

import "time"

type Account struct {
	ID           string    `json:"id"`
	Provider     string    `json:"provider"`
	MemberID     string    `json:"memberId"`
	DisplayName  string    `json:"displayName"`
	AuthorURN    string    `json:"authorUrn"`
	AccessToken  string    `json:"-"`
	RefreshToken string    `json:"-"`
	Scopes       []string  `json:"scopes"`
	ExpiresAt    time.Time `json:"expiresAt"`
	ConnectedAt  time.Time `json:"connectedAt"`
}

type OAuthState struct {
	State     string
	CreatedAt time.Time
}
