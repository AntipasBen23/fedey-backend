package linkedinaccounts

import "errors"

var (
	ErrAccountNotConnected = errors.New("linkedin account not connected")
	ErrOAuthStateNotFound  = errors.New("linkedin oauth state not found")
)
