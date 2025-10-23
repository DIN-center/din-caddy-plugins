package siwe

import (
	"context"
	"math/big"
	"net/url"
	"testing"
	"time"

	sessions "github.com/DIN-center/din-caddy-plugins/lib/auth/siwe/sessions"
	snft "github.com/DIN-center/din-sc/apps/din-go/lib/superfluidnft"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestValidateFlow_WithMemoryTrackers(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	baseMetadata := &snft.NFTMetadata{
		StreamRecipient:       common.HexToAddress("0xabc"),
		TokenAddress:          common.HexToAddress("0xdef"),
		StreamMax:             big.NewInt(1000),
		RequestsPerSecondLimit: 100,
		TokenId:               new(big.Int),
		NFTAddress:            common.HexToAddress("0x123"),
	}

	tests := []struct {
		name         string
		setup        func(ft, rt *sessions.Tracker)
		query        url.Values
		totalFlow    *big.Int
		expectCode   int
		expectErrMsg string
		expectSuccess bool
	}{
		{
			name: "Missing RPS in query",
			setup: func(ft, rt *sessions.Tracker) {},
			query:        url.Values{},
			totalFlow:    big.NewInt(100),
			expectCode:   401,
			expectErrMsg: "must specify a number of rps",
		},
		{
			name: "Requested RPS exceeds available",
			setup: func(ft, rt *sessions.Tracker) {
				err := rt.AddSession(ctx, "existing", "https://example.com", big.NewInt(90), time.Now().Add(time.Hour))
				require.NoError(t, err)
			},
			query:        url.Values{"rps": {"120"}},
			totalFlow:    big.NewInt(100),
			expectCode:   401,
			expectErrMsg: "requested more RPS",
		},
		{
			name: "Required flow exceeds available",
			setup: func(ft, rt *sessions.Tracker) {
				err := ft.AddSession(ctx, "existing", "flowkey", big.NewInt(90), time.Now().Add(time.Hour))
				require.NoError(t, err)
			},
			query:        url.Values{"rps": {"50"}},
			totalFlow:    big.NewInt(100),
			expectCode:   401,
			expectErrMsg: "needs more flow",
		},
		{
			name: "Successful validation",
			setup: func(ft, rt *sessions.Tracker) {},
			query:        url.Values{"rps": {"10"}},
			totalFlow:    big.NewInt(100),
			expectCode:   0,
			expectSuccess: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			flowTracker := sessions.NewMemoryTracker()
			rpsTracker := sessions.NewMemoryTracker()

			if tc.setup != nil {
				tc.setup(flowTracker, rpsTracker)
			}

			mw := &SIWEAuthMiddleware{
				FlowrateTracker: flowTracker,
				RPSTracker:      rpsTracker,
				logger:          logger,
			}

			code, err := mw.validateFlow(ctx, "session", time.Now(), tc.query, common.Address{}, baseMetadata, tc.totalFlow)

			if tc.expectSuccess {
				require.NoError(t, err)
				require.Equal(t, 0, code)
				return
			}

			require.Equal(t, tc.expectCode, code)
			if tc.expectErrMsg != "" {
				require.ErrorContains(t, err, tc.expectErrMsg)
			}
		})
	}
}
