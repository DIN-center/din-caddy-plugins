package runtime

type IRuntimeClient interface {
	GetLatestBlock(url string) (*int64, error)
}
