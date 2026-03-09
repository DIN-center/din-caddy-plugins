package network_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"syscall"
	"testing"

	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/DIN-center/din-caddy-plugins/lib/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
	"gotest.tools/v3/golden"
)

func TestTronHandler_GetType(t *testing.T) {
	handler := network.NewTronHandler(&network.NetworkConfig{})

	assert.Equal(t, network.TronHandlerType, handler.GetType())
}

func TestTronHandler_GetName(t *testing.T) {
	handler := network.NewTronHandler(&network.NetworkConfig{})

	expected := "Tron Full Node Handler"
	assert.Equal(t, expected, handler.GetName())
}

//
// Request processing
//

func TestHandler_ProcessRequest(t *testing.T) {
	t.Parallel()

	t.Run("passes with allowed HTTP method", func(t *testing.T) {
		testcases := []string{http.MethodGet, http.MethodPost}

		for _, testcase := range testcases {
			testcase := testcase

			t.Run(testcase, func(t *testing.T) {
				t.Parallel()

				h := &network.TronHandler{}
				require.NoError(t, h.ProcessRequest(&http.Request{
					Method: testcase,
				}))
			})
		}
	})

	t.Run("fails with disallowed HTTP method", func(t *testing.T) {
		testcases := []string{
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodHead,
			http.MethodOptions,
			http.MethodConnect,
			http.MethodTrace,
		}

		for _, testcase := range testcases {
			testcase := testcase

			t.Run(testcase, func(t *testing.T) {
				t.Parallel()

				h := &network.TronHandler{}
				require.ErrorIs(t, h.ProcessRequest(&http.Request{
					Method: testcase,
				}), network.ErrUnexpectedHTTPMethod)
			})
		}
	})
}

// Custom error that implements net.Error with Timeout() returning true
type timeoutError struct {
	error
}

func (e timeoutError) Timeout() bool {
	return true
}

func (e timeoutError) Temporary() bool {
	return false
}

func TestHandler_IsRetryableError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		statusCode int
		expected   bool
	}{
		{
			name:       "retryable status code 500",
			statusCode: http.StatusInternalServerError,
			expected:   true,
		},
		{
			name:       "retryable status code 503",
			statusCode: http.StatusServiceUnavailable,
			expected:   true,
		},
		{
			name:       "retryable status code 429",
			statusCode: http.StatusTooManyRequests,
			expected:   true,
		},
		{
			name:       "non-retryable status code 400",
			statusCode: http.StatusBadRequest,
			expected:   false,
		},
		{
			name:       "non-retryable status code 404",
			statusCode: http.StatusNotFound,
			expected:   false,
		},
		{
			name:       "retryable net.Error with timeout",
			err:        timeoutError{errors.New("i/o timeout")},
			statusCode: http.StatusOK,
			expected:   true,
		},
		{
			name:       "retryable io.EOF",
			err:        io.EOF,
			statusCode: http.StatusOK,
			expected:   true,
		},
		{
			name:       "retryable syscall.ECONNRESET",
			err:        syscall.ECONNRESET,
			statusCode: http.StatusOK,
			expected:   true,
		},
		{
			name:       "retryable syscall.ECONNREFUSED",
			err:        syscall.ECONNREFUSED,
			statusCode: http.StatusOK,
			expected:   true,
		},
		{
			name:       "non-retryable generic error",
			err:        errors.New("some random error"),
			statusCode: http.StatusOK,
			expected:   false,
		},
		{
			name:       "no error and non-retryable status code",
			err:        nil,
			statusCode: http.StatusOK,
			expected:   false,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			h := &network.TronHandler{}
			assert.Equal(t, test.expected, h.IsRetryableError(test.err, test.statusCode))
		})
	}
}

//
// Block operations
//

func TestHandler_CreateBlockRequest(t *testing.T) {
	t.Parallel()

	h := &network.TronHandler{}

	exp := golden.Get(t, "tron_get_block_params_golden.json")

	act, err := h.CreateBlockRequest("", 1000000, false)
	require.NoError(t, err)
	assert.JSONEq(t, string(exp), string(act))
}

func TestHandler_ParseBlockResponse(t *testing.T) {
	t.Parallel()

	body := golden.Get(t, "tron_block.json")

	h := &network.TronHandler{}

	i, err := h.ParseBlockResponse(body)
	require.NoError(t, err)
	assert.IsType(t, (*network.TronBlock)(nil), i)
}

func TestHandler_GetLatestBlockNumber(t *testing.T) {
	t.Parallel()

	body := golden.Get(t, "tron_block.json")
	statusCode := http.StatusOK

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cl := din_http.NewMockIHTTPClient(ctrl)
	cl.EXPECT().Post(
		"/wallet/getblock",
		map[string]string{"Content-Type": "application/json"},
		gomock.Any(),
		nil,
	).Return(body, &statusCode, nil)

	h := &network.TronHandler{}

	blockNum, err := h.GetLatestBlockNumber("", map[string]string{"Content-Type": "application/json"}, cl, nil, 5)
	require.NoError(t, err)
	assert.Equal(t, int64(1000000), blockNum.BlockNumber)
	assert.Equal(t, network.Healthy, blockNum.HealthStatus)
	assert.Equal(t, http.StatusOK, blockNum.ResponseStatus)
	assert.Empty(t, blockNum.Metadata)
}

//
// Healthcheck methods
//

//
// ChainID operations
//

func TestHandler_GetChainIDMethod(t *testing.T) {
	t.Parallel()

	h := &network.TronHandler{}
	require.NotEmpty(t, h.GetChainIDMethod())
}

func TestHandler_ParseChainResponse(t *testing.T) {
	t.Parallel()

	t.Run("passes with Nile genesis block", func(t *testing.T) {
		body := golden.Get(t, "tron_genesis.json")
		h := &network.TronHandler{}

		chainID, err := h.ParseChainIDResponse(body, http.StatusOK)
		require.NoError(t, err)
		assert.Equal(t, "0xcd8690dc", chainID)
	})

	tests := []struct {
		name       string
		body       []byte
		statusCode int
		err        error
	}{
		{
			name:       "fails if not Nile genesis block",
			body:       golden.Get(t, "tron_block.json"),
			statusCode: http.StatusOK,
			err:        network.ErrNotTronGenesisBlock,
		},
		{
			name:       "fails if not 200 OK",
			body:       golden.Get(t, "tron_genesis.json"),
			statusCode: http.StatusNotFound,
			err:        network.ErrRetrievingTronBlock,
		},
		{
			name:       "fails if block hash is not present",
			body:       []byte("{}"),
			statusCode: http.StatusOK,
			err:        network.ErrUnexpectedHexFieldLength,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			h := &network.TronHandler{}

			_, err := h.ParseChainIDResponse(test.body, test.statusCode)
			require.ErrorIs(t, err, test.err)
		})
	}
}

func TestHandler_GetChainID(t *testing.T) {
	t.Parallel()

	body := golden.Get(t, "tron_genesis.json")
	statusCode := http.StatusOK

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cl := din_http.NewMockIHTTPClient(ctrl)
	cl.EXPECT().Post(
		"/wallet/getblock",
		map[string]string{"Content-Type": "application/json"},
		gomock.Any(),
		nil,
	).Return(body, &statusCode, nil)

	h := &network.TronHandler{}

	chainID, err := h.GetChainID("", map[string]string{"Content-Type": "application/json"}, cl, nil, 5)
	require.NoError(t, err)
	assert.Equal(t, "0xcd8690dc", chainID)
}

func TestHandler_ValidateChainID(t *testing.T) {
	t.Parallel()

	h := &network.TronHandler{}

	t.Run("passes with valid IDs", func(t *testing.T) {
		t.Parallel()

		require.Nil(t, h.ValidateChainID("0xdeadbeef"))
	})

	testcases := []struct {
		name    string
		chainID string
	}{
		{name: "invalid prefix", chainID: "0ydeadbeef"},
		{name: "hex too short", chainID: "0xdead"},
		{name: "hex too long", chainID: "0xdeadbeefface"},
		{name: "invalid hex digit", chainID: "0xdeadbeeh"},
	}

	for _, testcase := range testcases {
		testcase := testcase

		t.Run("errors with "+testcase.name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, h.ValidateChainID(testcase.chainID), network.ErrUnexpectedChainID)
		})
	}
}

//
// Archive mode methods (not supported by the Tron Full Node API)
//

func TestHandler_SupportsArchiveMode(t *testing.T) {
	t.Parallel()

	h := &network.TronHandler{}
	require.False(t, h.SupportsArchiveMode())
}

func TestHandler_GetArchiveMethod(t *testing.T) {
	t.Parallel()

	h := &network.TronHandler{}
	assert.Empty(t, h.GetArchiveMethod())
}

func TestHandler_CreateArchivePayload(t *testing.T) {
	t.Parallel()

	h := &network.TronHandler{}
	_, err := h.CreateArchivePayload("", "")
	require.ErrorIs(t, err, network.ErrUnsupportedFeature)
}

func TestHandler_ParseArchiveResponse(t *testing.T) {
	t.Parallel()

	h := &network.TronHandler{}
	require.ErrorIs(t, h.ParseArchiveResponse([]byte{}), network.ErrUnsupportedFeature)
}

func TestHandler_PerformArchiveCheck(t *testing.T) {
	t.Parallel()

	h := &network.TronHandler{}
	require.ErrorIs(t, h.PerformArchiveCheck("", nil, nil, nil, 0, ""), network.ErrUnsupportedFeature)
}

//
// Supporting types
//

func TestTronBlock_Unmarshal(t *testing.T) {
	t.Parallel()

	// This test is really about whether we've modeled the Go block types
	// correctly.  The test block data was retrieved from
	// https://developers.tron.network/reference/getblock-1 and updated
	// test data can be retrieved if the model changes in significant ways -
	// this is unlikely for our use cases.
	data := golden.Get(t, "tron_block.json")

	var block network.TronBlock
	require.NoError(t, json.Unmarshal(data, &block))
	assert.Equal(t, "00000000000f424013e51b18e0782a32fa079ddafdb2f4c343468cf8896dc887", block.BlockID)
	assert.Equal(t, int64(1000000), block.BlockHeader.RawData.Number)
	assert.Equal(t, "e2dd42daa9c853e070df1e4fc927851a7438c601fb12cf945922629d5cec187b", block.BlockHeader.RawData.TXTrieRoot)
	assert.Equal(t, "41f16412b9a17ee9408646e2a21e16478f72ed1e95", block.BlockHeader.RawData.WitnessAddress)
	assert.Equal(t, "00000000000f423fbccd9cdb9e410eabdf9f94145cf5b71a678b8b9616619125", block.BlockHeader.RawData.ParentHash)
	assert.Equal(t, int64(9), block.BlockHeader.RawData.Version)
	assert.Equal(t, int64(1578594852000), block.BlockHeader.RawData.Timestamp)
	assert.Equal(t, "179caed28b3d129dee6d553ae6761a8c8698f890e5f38e062d56e2d0d0b8c9a20317b7753b3fd1be261935d6182434a6c057f8926f25027b9859cde3da3a655300", block.BlockHeader.WitnessSignature)
}
