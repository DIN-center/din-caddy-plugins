package watcherscore

import (
	"time"

	"github.com/DIN-center/din-sc/apps/din-go/lib/watcher"
)

// IWatcherScoreManager defines the responsibilities of the component that computes and stores the scores for providers on a given network.
type IWatcherScoreManager interface {
	// ComputeScores computes the scores for all providers on all networks
	ComputeScores() error
	// GetScore gets the score for a given provider on a given network
	GetScore(network string, providerID string) *Score
	// StartPeriodicUpdates starts a background process that periodically computes the scores for all providers on all networks
	StartPeriodicUpdates(frequency time.Duration) chan struct{}
	// GetAllScores gets all the scores for all providers on a given network
	GetAllScores(network string) map[string]*Score
	// AddNetworkWithBuiltInFormula adds a network with a built-in formula that uses the default formula (see the documentation for the default formula)
	AddNetworkWithBuiltInFormula(network string, client watcher.IWatcherAPIClient) error
	// AddNetworkFormula adds a network with a custom formula
	AddNetworkFormula(network string, formula ScoreFormula)
	// RemoveNetwork removes a network
	RemoveNetwork(network string)
	// GetNetworkFormula gets the formula for a given network
	GetNetworkFormula(network string) *ScoreFormula
}
