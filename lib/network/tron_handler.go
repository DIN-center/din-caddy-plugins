package network

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/logger"
	"github.com/cenkalti/backoff/v5"
)

const (
	tronHandlerVersion      = "1.0.0"
	tronHealthCheckEndpoint = "/wallet/getnowblock" // Get the height from the block info
	tronBlockInfoEndpoint   = "/wallet/getblock"
	tronChainIDMethod       = "/wallet/getblock"
)

// ErrNotTronGenesisBlock is returned when the block retrieved to calculate
// the chain id is not the genesis block (the block with "number":0).
var ErrNotTronGenesisBlock = errors.New("the parsed block is not a Tron genesis block")

// ErrRetrievingTronBlock is returned when the HTTP request fails or when
// the returned HTTP body can't be unmarshaled into a TronBlock.
var ErrRetrievingTronBlock = errors.New("failed to retrieve Tron block")

// ErrTronAPI is returned when an error code (and possibly and associated
// message) is returned in the body of an HTTP response.
var ErrTronAPI = errors.New("an error was returned from the Tron API")

// ErrUnderflow is returned when an int can't be safely cast to a uint.
var ErrUnderflow = errors.New("underflow converting from int to uint")

// ErrUnexpectedHexFieldLength is returned if the string being validated
// doesn't match the expected number of hex digits.
var ErrUnexpectedHexFieldLength = errors.New("unexpected hex field length")

// ErrUnexpectedZeroValue is returned when a value returned from the
// upstream is the Go zero-value but should contain "non-zero" data.
var ErrUnexpectedZeroValue = errors.New("unexpected zero value")

// ErrUnexpectedChainID is returned when the chain id returned from the
// tested chain doesn't match the value in the Caddyfile configuration.
var ErrUnexpectedChainID = errors.New("unexpected chain id")

// ErrUnexpectedStatusCode is returned when the HTTP response status
// code doesn't match the value expected per the API specification.
var ErrUnexpectedStatusCode = errors.New("unexpected HTTP status code")

// ErrUnexpectedHTTPMethod is returned when the HTTP method is not
// supported.
var ErrUnexpectedHTTPMethod = errors.New("unexpected HTTP method")

// ErrUnsupportedFeature is returned when a method included in the Handler
// interface is not implemented.
var ErrUnsupportedFeature = errors.New("unsupported feature")

var _ NetworkHandler = (*TronHandler)(nil)

type TronHandler struct {
	// EVMHandler
	config *NetworkConfig
	logger *logger.LoggerClient
}

// NewTronHandler creates a new Tron full node handler instance.
func NewTronHandler(config *NetworkConfig) *TronHandler {
	return &TronHandler{
		config: config,
		logger: config.Logger,
	}
}

//
// Lifecycle methods
//

// Initialize implements the Handler interface.
func (h *TronHandler) Initialize(config *NetworkConfig) error {
	h.config = config

	// Update logger from config if available
	if config.Logger != nil {
		h.logger = config.Logger
	}

	return nil
}

//
// Core identification and metadata methods for registry
//

// GetType implements the Handler interface.
func (h *TronHandler) GetType() string {
	return "tron-full-node"
}

// GetName implements the Handler interface.
func (h *TronHandler) GetName() string {
	return "Tron Full Node Handler"
}

// GetRequestType implements the Handler interface.
func (h *TronHandler) GetRequestType() RequestType {
	return RequestTypeREST
}

// GetVersion returns the version number for the TronHandler.
func (h *TronHandler) GetVersion() string {
	return tronHandlerVersion
}

//
// Request processing
//

// ProcessRequest implements the Handler interface.
func (h *TronHandler) ProcessRequest(req *http.Request) error {
	if req.Method != http.MethodGet && req.Method != http.MethodPost {
		return fmt.Errorf("%w: GET or POST are allowed but received %s", ErrUnexpectedHTTPMethod, req.Method)
	}

	return nil
}

// ExtractMethod implements the Handler interface.
func (h *TronHandler) ExtractMethod(req *http.Request, body []byte) (string, error) {
	return req.URL.Path, nil
}

// ConfigureRequestPath implements the Handler interface.
func (h *TronHandler) ConfigureRequestPath(req *http.Request, providerPath string, providerQuery string, networkName string) error {
	ConfigureRESTRequestPath(req, providerPath, networkName)
	// Note: providerQuery is not used for Tron currently. Can be implemented if needed.
	return nil
}

// ParseResponse implements the Handler interface.
func (h *TronHandler) ParseResponse(body []byte, statusCode int) error {
	if statusCode >= 400 {
		return fmt.Errorf("HTTP error: %d, body: %s", statusCode, string(body))
	}

	// All the methods that return error information in the body do so as
	// JSON objects.
	if !strings.HasPrefix(string(body), "{") || !strings.HasSuffix(string(body), "}") {
		return nil
	}

	var fields map[string]interface{}
	if err := json.Unmarshal(body, &fields); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	msg, _ := fields["message"].(string)
	if code, codeOK := fields["code"]; codeOK {
		return fmt.Errorf("%w: code: %s, message: %s", ErrTronAPI, code, msg)
	}

	return nil
}

// IsRetryableError implements the Handler interface.
//
// The following Tron Full Node HTTP API methods return additional fields to
// further analyze returned errors:
//
//   - /wallet/validateaddress: message
//   - /wallet/broadcasttransaction: code, message
//   - /wallet/broadcasthex: code, message
//
// The documentation doesn't describe when these errors are returned or
// what the expected errors might be.  In addition, the /wallet/validateAddress
// method returns either the format of the validated address or an error
// message depending on the value of the HTTP response status code.  Per
// the above analysis, the HTTP response body should be returned to the
// user along with the status code.
func (h *TronHandler) IsRetryableError(err error, statusCode int) bool {
	// Server errors and rate limits are retryable
	if statusCode >= 500 || statusCode == 429 {
		return true
	}

	// Network errors are retryable
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return true
		}

		switch {
		case errors.Is(err, io.EOF),
			errors.Is(err, syscall.ECONNRESET),
			errors.Is(err, syscall.ECONNREFUSED):
			return true
		}
	}

	return false
}

//
// Block operations
//

// FormatBlockHeight implements the Handler interface.
func (h *TronHandler) FormatBlockHeight(blockNum int64) string {
	return fmt.Sprintf("%d", blockNum) // Decimal format for Tron
}

// CreateBlockRequest implements the Handler interface.
func (h *TronHandler) CreateBlockRequest(_ string, blockNum int64, includeTransactions bool) ([]byte, error) {
	return json.Marshal(
		struct {
			IDOrNum string `json:"id_or_num"`
			Detail  bool   `json:"detail"`
		}{
			IDOrNum: h.FormatBlockHeight(blockNum),
			Detail:  includeTransactions,
		},
	)
}

// ParseBlockResponse implements the Handler interface.
func (h *TronHandler) ParseBlockResponse(body []byte) (interface{}, error) {
	return h.parseBlockResponse(body)
}

// ExtractBlockHash implements the Handler interface.
func (h *TronHandler) ExtractBlockHash(blockData interface{}) string {
	block, _ := blockData.(*TronBlock)

	return block.BlockID
}

// SupportsGetBlockByNumber implements the Handler interface.
func (h *TronHandler) SupportsGetBlockByNumber() bool {
	return true
}

// GetSupportedMethods implements the Handler interface.
func (h *TronHandler) GetSupportedMethods() []string {
	return []string{
		// Address Utilities - https://developers.tron.network/reference/full-node-address-utilities (1)
		"/wallet/validateaddress",
		// Transactions - https://developers.tron.network/reference/broadcasttransaction (3)
		"/wallet/broadcasttransaction",
		"/wallet/broadcasthex",
		"/wallet/createtransaction",
		// Accounts - https://developers.tron.network/reference/account-createaccount (5)
		"/wallet/createaccount",
		"/wallet/getaccount",
		"/wallet/updateaccount",
		"/wallet/accountpermissionupdate",
		"/wallet/getaccountbalance",
		// Account Resources - https://developers.tron.network/reference/getaccountresource (17)
		"/wallet/getaccountresource",
		"/wallet/getaccountnet",
		"/wallet/freezebalance",
		"/wallet/unfreezebalance",
		"/wallet/getdelegatedresource",
		"/wallet/getdelegatedresourceaccountindex",
		"/wallet/freezebalancev2",
		"/wallet/unfreezebalancev2",
		"/wallet/cancelallunfreezev2",
		"/wallet/delegateresource",
		"/wallet/undelegateresource",
		"/wallet/withdrawexpireunfreeze",
		"/wallet/getavailableunfreezecount",
		"/wallet/getcanwithdrawunfreezeamount",
		"/wallet/getcandelegatedmaxsize",
		"/wallet/getdelegatedresourcev2",
		"/wallet/getdelegatedresourceaccountindexv2",
		// Query The Network - https://developers.tron.network/reference/getblock-1 (17)
		"/wallet/getblock",
		"/wallet/getblockbynum",
		"/wallet/getblockbyid",
		"/wallet/getblockbylatestnum",
		"/wallet/getblockbylimitnext",
		"/wallet/getnowblock",
		"/wallet/gettransactionbyid",
		"/wallet/gettransactioninfobyid",
		"/wallet/gettransactioninfobyblocknum",
		"/wallet/listnodes",
		"/wallet/getnodeinfo",
		"/wallet/getchainparameters",
		"/wallet/getblockbalance",
		"/wallet/getenergyprices",
		"/wallet/getbandwidthprices",
		"/wallet/getburntrx",
		"/getapprovedlist",
		// TRC10 Token - https://developers.tron.network/reference/getassetissuebyaccount (11)
		"/wallet/getassetissuebyaccount",
		"/wallet/getassetissuebyid",
		"/wallet/getassetissuebyname",
		"/wallet/getassetissuelist",
		"/wallet/getassetissuelistbyname",
		"/wallet/getpaginatedassetissuelist",
		"/wallet/transferasset",
		"/wallet/createassetissue",
		"/wallet/participateassetissue",
		"/wallet/unfreezeasset",
		"/wallet/updateasset",
		// Smart Contracts - https://developers.tron.network/reference/wallet-getcontract (9)
		"/wallet/getcontract",
		"/wallet/getcontractinfo",
		"/wallet/triggersmartcontract",
		"/wallet/triggerconstantcontract",
		"/wallet/deploycontract",
		"/wallet/updatesetting",
		"/wallet/updateenergylimit",
		"/wallet/clearabi",
		"/wallet/estimateenergy",
		// TRONZ Shielded Smart Contract - https://developers.tron.network/reference/tronz-shielded-smart-contract ()
		// TODO:
		// Voting & SRs - https://developers.tron.network/reference/listwitnesses (9)
		"/wallet/listwitnesses",
		"/wallet/createwitness",
		"/wallet/updatewitness",
		"/wallet/getBrokerage",
		"/wallet/updateBrokerage",
		"/wallet/votewitnessaccount",
		"/wallet/getReward",
		"/wallet/withdrawbalance",
		"/wallet/getnextmaintenancetime",
		// Proposals - https://developers.tron.network/reference/wallet-listproposals (5)
		"/wallet/listproposals",
		"/wallet/getproposalbyid",
		"/wallet/proposalcreate",
		"/wallet/proposalapprove",
		"/wallet/proposaldelete",
		// DEX Exchange - https://developers.tron.network/reference/wallet-listexchanges (6)
		"/wallet/listexchanges",
		"/wallet/getexchangebyid",
		"/wallet/exchangecreate",
		"/wallet/exchangeinject",
		"/wallet/exchangewithdraw",
		"/wallet/exchangetransaction",
		// Pending Pool - https://developers.tron.network/reference/gettransactionlistfrompending (3)
		"/wallet/gettransactionlistfrompending",
		"/wallet/gettransactionfrompending",
		"/wallet/getpendingsize",
	}
}

// GetBlockByNumberMethod implements the Handler interface.
func (h *TronHandler) GetBlockByNumberMethod() string {
	return tronBlockInfoEndpoint
}

// GetLatestBlockNumber implements the Handler interface.
func (h *TronHandler) GetLatestBlockNumber(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (*LatestBlockResult, error) {
	if requestAttempts < 0 {
		return nil, fmt.Errorf("%w: %d", ErrUnderflow, requestAttempts)
	}

	tries := uint(requestAttempts)

	op := func() (*LatestBlockResult, error) {
		body, statusCode, err := httpClient.Post(httpUrl+h.GetBlockByNumberMethod(), headers, []byte{}, authClient)
		if err != nil {
			return nil, err
		}

		if *statusCode >= http.StatusBadRequest && *statusCode < http.StatusInternalServerError && *statusCode != http.StatusTooManyRequests {
			return nil, &backoff.PermanentError{
				Err: fmt.Errorf("%w: (Client error) %d", ErrUnexpectedStatusCode, *statusCode),
			}
		}

		if *statusCode <= http.StatusOK && *statusCode >= http.StatusMultipleChoices {
			return nil, fmt.Errorf("%w: %d", ErrUnexpectedStatusCode, *statusCode)
		}

		block, err := h.parseBlockResponse(body)
		if err != nil {
			return nil, err
		}

		return &LatestBlockResult{
			BlockNumber:    block.BlockHeader.RawData.Number,
			HealthStatus:   Healthy,
			ResponseStatus: *statusCode,
		}, nil
	}

	return backoff.Retry(
		context.Background(),
		op,
		backoff.WithBackOff(backoff.NewExponentialBackOff()),
		backoff.WithMaxTries(tries),
	)
}

// RequiresSeparateBlockInfoCall implements the Handler interface.
func (h *TronHandler) RequiresSeparateBlockInfoCall() bool {
	return false
}

// GetBlockInfoMethod implements the Handler interface.
func (h *TronHandler) GetBlockInfoMethod() string {
	return tronBlockInfoEndpoint
}

// ParseBlockNumberResponse implements the Handler interface.
func (h *TronHandler) ParseBlockNumberResponse(body []byte, statusCode int) (int64, error) {
	if statusCode != http.StatusOK {
		return 0, fmt.Errorf("%w: expected %d, got %d", ErrUnexpectedStatusCode, http.StatusOK, statusCode)
	}

	block, err := h.parseBlockResponse(body)
	if err != nil {
		return 0, err
	}

	return block.BlockHeader.RawData.Number, nil
}

// PerformGetBlockByNumber implements the Handler interface.
func (h *TronHandler) PerformGetBlockByNumber(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int, blockNumber int64) (interface{}, error) {
	if requestAttempts < 0 {
		return nil, fmt.Errorf("%w: %d", ErrUnderflow, requestAttempts)
	}

	tries := uint(requestAttempts)

	op := func() (*TronBlock, error) {
		reqBody, err := h.CreateBlockRequest("", blockNumber, false)
		if err != nil {
			return nil, err
		}

		respBody, statusCode, err := httpClient.Post(httpUrl+h.GetBlockByNumberMethod(), headers, reqBody, authClient)
		if err != nil {
			return nil, err
		}

		if *statusCode >= http.StatusBadRequest && *statusCode < http.StatusInternalServerError && *statusCode != http.StatusTooManyRequests {
			return nil, &backoff.PermanentError{
				Err: fmt.Errorf("%w: (Client error) %d", ErrUnexpectedStatusCode, *statusCode),
			}
		}

		if *statusCode != http.StatusOK {
			return nil, fmt.Errorf("%w: %d", ErrUnexpectedStatusCode, *statusCode)
		}

		return h.parseBlockResponse(respBody)
	}

	return backoff.Retry(
		context.Background(),
		op,
		backoff.WithBackOff(backoff.NewExponentialBackOff()),
		backoff.WithMaxTries(tries),
	)
}

//
// Healthcheck methods
//

// GetHealthCheckMethod implements the Handler interface.
func (h *TronHandler) GetHealthCheckMethod() string {
	return tronHealthCheckEndpoint
}

// GetHealthcheckHTTPMethod implements the Handler interface.
func (h *TronHandler) GetHealthCheckHTTPMethod() string {
	return "POST"
}

// CreateHealthCheckPayload implements the Handler interface.
func (h *TronHandler) CreateHealthCheckPayload(method string) ([]byte, error) {
	// The getnowblock method takes no parameters
	return nil, nil
}

// ParseHealthCheckResponse implements the Handler interface.
func (h *TronHandler) ParseHealthCheckResponse(body []byte) (*BlockInfo, error) {
	var blockInfo TronBlock
	if err := json.Unmarshal(body, &blockInfo); err != nil {
		return nil, fmt.Errorf("failed to parse Tron block response: %w", err)
	}

	return &BlockInfo{
		Number:    blockInfo.BlockHeader.RawData.Number,
		Hash:      blockInfo.BlockID,
		Timestamp: time.Unix(blockInfo.BlockHeader.RawData.Timestamp/1000, 0),
		Metadata: map[string]interface{}{
			"tx_trie_root":       blockInfo.BlockHeader.RawData.TXTrieRoot,
			"parent_hash":        blockInfo.BlockHeader.RawData.ParentHash,
			"witness_id":         blockInfo.BlockHeader.RawData.WitnessID,
			"witness_address":    blockInfo.BlockHeader.RawData.WitnessAddress,
			"version":            blockInfo.BlockHeader.RawData.Version,
			"account_state_root": blockInfo.BlockHeader.RawData.AccountStateRoot,
			"witness_signature":  blockInfo.BlockHeader.WitnessSignature,
		},
	}, nil
}

//
// ChainID operations
//

// GetChainIDMethod implements the Handler interface.
func (h *TronHandler) GetChainIDMethod() string {
	return tronChainIDMethod
}

// ParseChainIDResponse implements the Handler interface.
//
// [Per the documentation], the chain id for each Tron network is the last
// four bytes of the genesis block hash in hexadecimal format and with
// the 0x prefix.
//
// [Per the documenttation]: https://developers.tron.network/reference/eth_chainid
func (h *TronHandler) ParseChainIDResponse(body []byte, statusCode int) (string, error) {
	if statusCode != http.StatusOK {
		return "", ErrRetrievingTronBlock
	}

	block, err := h.parseBlockResponse(body)
	if err != nil {
		return "", err
	}

	if !block.IsGenesis() {
		return "", fmt.Errorf("%w: block number %d", ErrNotTronGenesisBlock, block.BlockHeader.RawData.Number)
	}

	return "0x" + block.BlockID[len(block.BlockID)-8:], nil
}

// ValidateChainID implements the Handler interface.
func (h *TronHandler) ValidateChainID(chainID string) error {
	if len(chainID) != 10 || !strings.HasPrefix(chainID, "0x") {
		return fmt.Errorf("%w: expected 0x plus 8 hexadecimal digits but received %s", ErrUnexpectedChainID, chainID)
	}

	if _, err := hex.DecodeString(chainID[2:]); err != nil {
		return fmt.Errorf("%w: %w: contains invalid hexadecimal digits %s", ErrUnexpectedChainID, err, chainID)
	}

	return nil
}

// GetChainID implements the Handler interface.
func (h *TronHandler) GetChainID(httpUrl string, headers map[string]string, httpClient din_http.IHTTPClient, authClient auth.IAuthClient, requestAttempts int) (string, error) {
	if requestAttempts < 0 {
		return "", fmt.Errorf("%w: %d", ErrUnderflow, requestAttempts)
	}

	tries := uint(requestAttempts)

	op := func() (string, error) {
		reqBody, err := h.CreateBlockRequest("", 0, false)
		if err != nil {
			return "", err
		}

		respBody, statusCode, err := httpClient.Post(httpUrl+h.GetChainIDMethod(), headers, reqBody, authClient)
		if err != nil {
			return "", err
		}

		if *statusCode >= http.StatusBadRequest && *statusCode < http.StatusInternalServerError && *statusCode != http.StatusTooManyRequests {
			return "", &backoff.PermanentError{
				Err: fmt.Errorf("%w: (Client error) %d", ErrUnexpectedStatusCode, *statusCode),
			}
		}

		return h.ParseChainIDResponse(respBody, *statusCode)
	}

	return backoff.Retry(
		context.Background(),
		op,
		backoff.WithBackOff(backoff.NewExponentialBackOff()),
		backoff.WithMaxTries(tries),
	)
}

//
// Archive mode methods (not supported by the Tron Full Node API)
//

// SupportsArchiveMode implements the Handler interface.
func (h *TronHandler) SupportsArchiveMode() bool {
	return false
}

// GetArchiveMethod implements the Handler interface.
func (h *TronHandler) GetArchiveMethod() string {
	return ""
}

// CreateArchivePayload implements the Handler interface.
func (h *TronHandler) CreateArchivePayload(_ string, _ string) ([]byte, error) {
	return nil, fmt.Errorf("%w: the TronHandler does not support CreateCarchivePayload", ErrUnsupportedFeature)
}

// ParseArchiveResponse implements the Handler interface.
func (h *TronHandler) ParseArchiveResponse(_ []byte) error {
	return fmt.Errorf("%w: the TronHandler does not support ParseArchiveResponse", ErrUnsupportedFeature)
}

// ParseArchiveCheck implements the Handler interface
func (h *TronHandler) PerformArchiveCheck(_ string, _ map[string]string, _ din_http.IHTTPClient, _ auth.IAuthClient, _ int, _ string) error {
	return fmt.Errorf("%w: the TronHandler does not support PerformArchiveCheck", ErrUnsupportedFeature)
}

//
//
//

func (h *TronHandler) parseBlockResponse(body []byte) (*TronBlock, error) {
	var block TronBlock
	if err := json.Unmarshal(body, &block); err != nil {
		return nil, err
	}

	if err := block.Validate(); err != nil {
		return nil, err
	}

	return &block, nil
}

//
// Supporting types
//

// TronBlock represents the JSON structure of a Tron block.
type TronBlock struct {
	BlockID     string          `json:"blockID"`
	BlockHeader TronBlockHeader `json:"block_header"`
}

// IsGenesis returns true if the block's (buried) number is zero.
func (b *TronBlock) IsGenesis() bool {
	return b.BlockHeader.RawData.Number == 0
}

// Validate returns an error if the block's required fields are not
// present, the field data is the wrong type or the field data is in the
// wrong format.
func (b *TronBlock) Validate() error {
	if b.IsGenesis() {
		return errors.Join(
			b.validateHex("BlockID", b.BlockID, 64),
			b.validateHex("TXTrieRoot", b.BlockHeader.RawData.TXTrieRoot, 64),
			b.validateNotZero("WitnessAddress", b.BlockHeader.RawData.WitnessAddress),
			b.validateHex("ParentHash", b.BlockHeader.RawData.ParentHash, 64),
		)
	}

	return errors.Join(
		b.validateHex("BlockID", b.BlockID, 64),
		b.validateHex("TXTrieRoot", b.BlockHeader.RawData.TXTrieRoot, 64),
		b.validateNotZero("WitnessAddress", b.BlockHeader.RawData.WitnessAddress),
		b.validateHex("ParentHash", b.BlockHeader.RawData.ParentHash, 64),
		b.validateNotZero("Version", b.BlockHeader.RawData.Version),
		b.validateNotZero("Timestamp", b.BlockHeader.RawData.Timestamp),
		b.validateHex("WitnessSignature", b.BlockHeader.WitnessSignature, 130),
	)
}

func (b *TronBlock) validateHex(key, val string, expectedLength int) error {
	_, err := hex.DecodeString(val)
	if err != nil {
		return err
	}

	if len(val) != expectedLength {
		return fmt.Errorf("%w: %s has length %d but expected %d", ErrUnexpectedHexFieldLength, key, len(val), expectedLength)
	}

	return nil
}

func (h *TronBlock) validateNotZero(key string, val any) error {
	if reflect.ValueOf(val).IsZero() {
		return fmt.Errorf("%w: %s", ErrUnexpectedZeroValue, key)
	}

	return nil
}

// TronBlockHeader contains the block's metadata (a fully populated and
// decoded TronBlock would also have an array of transactions.)
type TronBlockHeader struct {
	RawData          TronBlockRawData `json:"raw_data"`
	WitnessSignature string           `json:"witness_signature"`
}

// TronBlockRawData is the message that's "signed over" by the witness
// if a WitnessSignature is present.
type TronBlockRawData struct {
	Timestamp        int64  `json:"timestamp"`
	TXTrieRoot       string `json:"txTrieRoot"`
	ParentHash       string `json:"parentHash"`
	Number           int64  `json:"number"`
	WitnessID        int64  `json:"witness_id"`
	WitnessAddress   string `json:"witness_address"`
	Version          int64  `json:"version"`
	AccountStateRoot string `json:"accountStateRoot"`
}
