package xaccounts

import "errors"

var (
	ErrAccountNotConnected = errors.New("x account not connected")
	ErrOAuthStateNotFound  = errors.New("x oauth state not found")
)
