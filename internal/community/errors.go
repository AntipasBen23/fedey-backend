package community

import "errors"

var (
	ErrInvalidInboxInput = errors.New("invalid community inbox input")
	ErrItemNotFound      = errors.New("community item not found")
)
