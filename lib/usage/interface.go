package usage

import (
	"errors"
	"time"
)

var (
	ErrRequestLimit      = errors.New("request limit exceeded")
)

type UsageTracker interface {
	Use() error
}

type TrackerManager interface {
	Create(int64, time.Time) (string, error)
	Get(string) (UsageTracker, bool)
}