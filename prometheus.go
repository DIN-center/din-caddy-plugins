package din

import "github.com/prometheus/client_golang/prometheus"

// prometheus metric initialization
var dinRequestCount = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "din_http_request_count",
		Help: "",
	},
	[]string{"service", "method", "provider"},
)
