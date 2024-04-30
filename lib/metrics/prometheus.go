package metrics

import "github.com/prometheus/client_golang/prometheus"

// prometheus metric initialization
var DinRequestCount = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "din_http_request_count",
		Help: "",
	},
	[]string{"service", "method", "provider"},
)
