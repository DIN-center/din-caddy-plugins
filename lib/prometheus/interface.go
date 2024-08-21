package prometheus

type IPrometheusClient interface {
	HandleRequestMetric(reqBodyBytes []byte, data *PromRequestMetricData)
	HandleLatestBlockMetric(data *PromLatestBlockMetricData)
}
