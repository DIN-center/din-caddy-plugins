package http

type IHTTPClient interface {
	Post(url string, headers map[string]string, payload []byte) ([]byte, *int, error)
}
