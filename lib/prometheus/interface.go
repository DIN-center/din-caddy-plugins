package prometheus

import "time"

type IPrometheusClient interface {
	HandleRequestMetrics(data *PromRequestMetricData, reqBodyBytes []byte, duration time.Duration)
	HandleLatestBlockMetric(data *PromLatestBlockMetricData)
}
