package runtime

type IRuntimeClient interface {
	GetLatestBlockNumber(hcRPCMethod string, httpUrl string, headers map[string]string) (int64, int, error)
}
