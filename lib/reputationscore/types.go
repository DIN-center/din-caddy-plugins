package reputationscore

import (
	"fmt"
	"math"
	"net/url"
	"time"

	"github.com/pkg/errors"
)

// A provider metric is a number between 0 and 1 that can be used to measure the quality of a provider for a given criteria.
// For example, the block number consistency metric ensures the consistency rate of a provider's block number.
type ProviderMetric struct {
	metricID     string
	providerName string
	providerURL  *url.URL
	value        float64
	lastUpdated  time.Time
}

// Constructor function to create a new ProviderMetric
func NewProviderMetric(metricID, providerID, providerEndpoint string, value float64, lastUpdated time.Time) (*ProviderMetric, error) {
	if value < 0 || value > 1 {
		return nil, fmt.Errorf("value must be between 0 and 1, got %f", value)
	}

	providerURL, err := url.Parse(providerEndpoint)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse provider endpoint")
	}

	return &ProviderMetric{
		metricID:     metricID,
		providerName: providerID,
		providerURL:  providerURL,
		value:        value,
		lastUpdated:  lastUpdated,
	}, nil
}

func (m *ProviderMetric) MetricID() string {
	return m.metricID
}

func (m *ProviderMetric) ProviderName() string {
	return m.providerName
}

func (m *ProviderMetric) ProviderID() string {
	return m.providerURL.Hostname()
}

func (m *ProviderMetric) ProviderURL() *url.URL {
	return m.providerURL
}

func (m *ProviderMetric) Value() float64 {
	return m.value
}

func (m *ProviderMetric) LastUpdated() time.Time {
	return m.lastUpdated
}

// Equal returns true if the two ProviderMetrics are equal
func (m *ProviderMetric) Equal(other *ProviderMetric) bool {
	if m == nil || other == nil {
		return m == other
	}
	return m.metricID == other.metricID &&
		m.providerName == other.providerName &&
		m.providerURL.String() == other.providerURL.String() &&
		math.Abs(m.value-other.value) < 0.00001 &&
		m.lastUpdated.Equal(other.lastUpdated)
}

func (m *ProviderMetric) String() string {
	return fmt.Sprintf("ProviderMetric{metricID: %s, providerID: %s, providerURL: %s, value: %f, lastUpdated: %s}", m.metricID, m.ProviderID(), m.providerURL.String(), m.value, m.lastUpdated.Format(time.RFC3339))
}

var EmptyScore = &Score{value: 0, hasValue: false, lastUpdated: time.Time{}}

// Score is a value ranging from 0 to 1. Score is immutable.
type Score struct {
	value       float64
	hasValue    bool
	lastUpdated time.Time
}

func NewScore(value float64, lastUpdated time.Time) (*Score, error) {
	if value < 0 || value > 1 {
		return EmptyScore, fmt.Errorf("score value must be between 0 and 1, got %f", value)
	}
	return &Score{value: value, hasValue: true, lastUpdated: lastUpdated}, nil
}

func (s *Score) Clone() *Score {
	return &Score{value: s.value, hasValue: s.hasValue, lastUpdated: s.lastUpdated}
}

func NewEmptyScore() *Score {
	return EmptyScore
}

func (s *Score) HasValue() bool {
	return s.hasValue
}

func (s *Score) Value() float64 {
	return s.value
}

func (s *Score) LastUpdated() time.Time {
	return s.lastUpdated
}

func (s *Score) String() string {
	return fmt.Sprintf("Score{value: %f, hasValue: %t, lastUpdated: %s}", s.value, s.hasValue, s.lastUpdated.Format(time.RFC3339))
}

// A provider metric generator is responsible for generating metrics for all providers in a given network.
type ProviderMetricGenerator interface {
	GenerateMetrics(network string) ([]*ProviderMetric, error)
	MetricID() string
}

// A provider metric combiner is responsible for combining (reducing) metrics for all providers in a given network into a single score.
type ProviderMetricCombiner interface {
	CombineMetrics([]*ProviderMetric) (*Score, error)
}

// A score transformer is responsible for transforming the scores for all providers in a given network to a different set of scores.
// For example, a score transformer could be used to transform the scores to a different scale.
type ScoreTransformer interface {
	TransformScore(scores map[string]*Score) (map[string]*Score, error)
}
