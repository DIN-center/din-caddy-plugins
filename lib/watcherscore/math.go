package watcherscore

import (
	"fmt"
	"math"
	"time"
)

// Implementing a Normalized Exponential Function: f(x) = (e^(kx) - 1) / (e^k - 1)
// This is a normalized exponential function — a version of the exponential function that’s been rescaled so that:
// f(0) = 0 and f(1) = 1
//
// The parameter 𝑘 controls the curvature:
// - If 𝑘>1 :the curve rises sharply at the end (more exponential).
// - If 0 < 𝑘 < 1 :the curve is gentler (closer to linear).
// - If 𝑘<1 :the function becomes decreasing instead of increasing.

func NormalizedExponentialFunction(x float64, k float64) float64 {
	return (math.Exp(k*x) - 1.0) / (math.Exp(k) - 1.0)
}

// ExponentialPullToMidpoint returns the value converged toward midpoint given an elapsed time and a convergence time constant.
// This function is capped so that after elapsed >= tau, the score becomes exactly the midpoint:
//
//	f(s0, t; tau) = midpoint + (s0 - midpoint)*exp(-t/tau), for t < tau
//
//	f(s0, t; tau) = midpoint                           for t >= tau
//
// Arguments:
//   - init: initial value in [0,1]
//   - elapsed: time elapsed since the init value was updated (e.g., 15*time.Minute), should be non-negative
//   - tau: convergence time constant (e.g., 30*time.Minute), should be non-negative
//
// Notes:
//   - Always converges smoothly to midpoint regardless of starting value.
func ExponentialPullToMidpoint(init float64, elapsed, tau time.Duration, midpoint float64) (float64, error) {
	if init < 0 || init > 1 {
		return 0, fmt.Errorf("init must be between 0 and 1, got %v", init)
	}
	if tau < 0 {
		return 0, fmt.Errorf("tau must be greater than or equal to 0, got %d", tau)
	}
	if elapsed < 0 {
		return 0, fmt.Errorf("elapsed must be greater than or equal to 0, got %d", elapsed)
	}
	if elapsed >= tau {
		return midpoint, nil
	}
	t := elapsed.Minutes()
	tc := tau.Minutes()
	return midpoint + (init-midpoint)*math.Exp(-t/tc), nil
}
