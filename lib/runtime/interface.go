package runtime

type IRuntimeClient interface {
	GetLatestBlockNumber(httpUrl string, headers map[string]string) (int64, int, error)
}
