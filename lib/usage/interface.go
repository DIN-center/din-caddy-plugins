package usage

import (
	"errors"
)

var (
	ErrRequestLimit      = errors.New("request limit exceeded")
)

type UsageTracker interface {
	Use() error
}