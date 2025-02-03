package watcher

type IWatcherAPIClient interface {
	GetCheck(params CheckQueryParams) Result[CheckResponse]
	GetLatency(params LatencyQueryParams) Result[LatencyResponse]
}
