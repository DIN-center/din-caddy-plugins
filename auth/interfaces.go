package auth

import (
	"net/http"
	"time"
	"encoding/json"
	"sync/atomic"
	"errors"
)

type UnixTime time.Time

var (
	ErrRequestLimit = errors.New("request limit exceeded")
	ErrSessionExpired = errors.New("session expired")
	ErrSessionClosed = errors.New("session closed")
)

func (t UnixTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(t).Unix())
}

func (ut *UnixTime) UnmarshalJSON(data []byte) error {
    var timestamp int64
    if err := json.Unmarshal(data, &timestamp); err != nil {
        return err
    }
    t := time.Unix(timestamp, 0)
    *ut = UnixTime(t)
    return nil
}

type AuthToken struct {
	Headers    map[string]string `json:"headers`
	Expiration *UnixTime `json:"exp,omitempty"`
	Uses       *int64    `json:"uses,omitempty"`
}

func (at *AuthToken) Use() error {
	if at.Uses != nil && atomic.AddInt64(at.Uses, -1) < 0 {
			return ErrRequestLimit
	}
	if at.Expiration != nil && time.Since(time.Time(*at.Expiration)) > 0 {
		return ErrSessionExpired
	}
	return nil
}

type AuthClient interface {
	// Start a series of sessions with the provider. The AuthClient should automatically 
	// establish new sessions as they near expiration
	Start() error
	// Error will return an error if the AuthClient is unhealthy, or nil if it should be able to sign a valid request
	Error() error
	// GetToken should take a map of parameters for a token, and return a map of Header -> Value for a session
	GetToken(map[string]interface{}) (AuthToken, error)
	// Sign should add headers to the client request such that it would be accepted by the server
	Sign(*http.Request) error
	// Stop should end any Goroutines associated with this client. Once an AuthClient is stopped it cannot be started again
	Stop()
}