package prometheus

import "time"

type IPrometheusClient interface {
	HandleRequestMetrics(data *PromRequestMetricData, reqBodyBytes []byte, duration time.Duration)
	HandleHealthCheckMetric(data *PromHealthCheckMetricData)
	HandleNetworkHealthCheckMetric(data *PromNetworkHealthCheckMetricData)
}
