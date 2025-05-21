package prometheus

import (
	"time"

	dinHttp "github.com/DIN-center/din-caddy-plugins/lib/http"
)

type IPrometheusClient interface {
	HandleRequestMetrics(data *PromRequestMetricData, duration time.Duration, requestBody *dinHttp.JSONRPCRequest)
	HandleHealthCheckMetric(data *PromHealthCheckMetricData)
	HandleNetworkHealthCheckMetric(data *PromNetworkHealthCheckMetricData)
}
