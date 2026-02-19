package prometheus

import (
	"time"
)

type IPrometheusClient interface {
	HandleRequestMetrics(data *PromRequestMetricData, duration time.Duration)
	HandleHealthCheckMetric(data *PromHealthCheckMetricData)
	HandleNetworkHealthCheckMetric(data *PromNetworkHealthCheckMetricData)
}
