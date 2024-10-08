package modules

import (
	"testing"

	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"github.com/golang/mock/gomock"
	"go.uber.org/zap"
)

func TestSyncRegistryWithLatestBlock(t *testing.T) {
	logger := zap.NewNop()
	mockCtrl := gomock.NewController(t)
	mockDingoClient := din.NewMockIDingoClient(mockCtrl)
	dinMiddleware := &DinMiddleware{
		RegistryEnv:                         LineaMainnet,
		RegistryBlockEpoch:                  10,
		registryLastUpdatedEpochBlockNumber: 40,
		logger:                              logger,
		DingoClient:                         mockDingoClient,
	}

	tests := []struct {
		name                                string
		registryLastUpdatedEpochBlockNumber int64
		latestBlockNumber                   int64
		expectedUpdateCall                  bool
		expectedBlockFloorByEpoch           int64
	}{
		{
			name:                                "Sync should update as block difference is equal to or exceeds epoch 50",
			registryLastUpdatedEpochBlockNumber: int64(40),
			latestBlockNumber:                   int64(50),
			expectedUpdateCall:                  true,
			expectedBlockFloorByEpoch:           int64(50),
		},
		{
			name:                                "Sync should update as block difference is equal to or exceeds epoch 52",
			registryLastUpdatedEpochBlockNumber: int64(40),
			latestBlockNumber:                   int64(52),
			expectedUpdateCall:                  true,
			expectedBlockFloorByEpoch:           int64(50),
		},
		{
			name:                                "Sync should update as block difference is equal to or exceeds epoch 1000",
			registryLastUpdatedEpochBlockNumber: int64(40),
			latestBlockNumber:                   int64(1001),
			expectedUpdateCall:                  true,
			expectedBlockFloorByEpoch:           int64(1000),
		},
		{
			name:                                "Sync should not update as block difference is less than epoch 48",
			registryLastUpdatedEpochBlockNumber: int64(40),
			latestBlockNumber:                   int64(48),
			expectedUpdateCall:                  false,
			expectedBlockFloorByEpoch:           int64(40),
		},
		{
			name:                                "Sync should not update as block difference is less than epoch 30",
			registryLastUpdatedEpochBlockNumber: int64(40),
			latestBlockNumber:                   int64(30),
			expectedUpdateCall:                  false,
			expectedBlockFloorByEpoch:           int64(40),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			dinMiddleware.Networks = map[string]*network{
				LineaMainnet: {
					latestBlockNumber: tt.latestBlockNumber,
				},
			}
			dinMiddleware.registryLastUpdatedEpochBlockNumber = tt.registryLastUpdatedEpochBlockNumber

			mockDingoClient.EXPECT().GetLatestBlockNumber().Return(tt.latestBlockNumber, nil).Times(1)

			// Check if update was called as expected
			if tt.expectedUpdateCall {
				mockDingoClient.EXPECT().GetRegistryData().Return(&din.DinRegistryData{}, nil).Times(1)
			}
			// Call the function
			dinMiddleware.syncRegistryWithLatestBlock()

			// Validate that registryLastUpdatedEpochBlockNumber is updated correctly
			if dinMiddleware.registryLastUpdatedEpochBlockNumber != tt.expectedBlockFloorByEpoch {
				t.Errorf("Expected registryLastUpdatedEpochBlockNumber = %v, got %v", tt.expectedBlockFloorByEpoch, dinMiddleware.registryLastUpdatedEpochBlockNumber)
			}
		})
	}
}
