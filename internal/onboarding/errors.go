package onboarding

import "errors"

var (
	ErrInvalidSessionInput = errors.New("invalid onboarding session input")
	ErrSessionNotFound     = errors.New("onboarding session not found")
	ErrQuestionNotFound    = errors.New("onboarding question not found")
)
