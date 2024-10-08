package modules

import (
	"testing"

	dinsdk "github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"github.com/golang/mock/gomock"
	"go.uber.org/zap"
)

func TestSyncRegistryWithLatestBlock(t *testing.T) {
	logger := zap.NewNop()
	mockCtrl := gomock.NewController(t)
	mockDingoClient := dinsdk.NewMockIDingoClient(mockCtrl)
	dinMiddleware := &DinMiddleware{
		RegistryBlockEpoch:                  10,
		registryLastUpdatedEpochBlockNumber: 40,
		logger:                              logger,
		DingoClient:                         mockDingoClient,
	}

	tests := []struct {
		name                                string
		registryLastUpdatedEpochBlockNumber uint64
		latestBlockNumber                   uint64
		expectedUpdateCall                  bool
		expectedBlockFloorByEpoch           uint64
	}{
		{
			name:                                "Sync should update as block difference is equal to or exceeds epoch 50",
			registryLastUpdatedEpochBlockNumber: uint64(40),
			latestBlockNumber:                   uint64(50),
			expectedUpdateCall:                  true,
			expectedBlockFloorByEpoch:           uint64(50),
		},
		{
			name:                                "Sync should update as block difference is equal to or exceeds epoch 52",
			registryLastUpdatedEpochBlockNumber: uint64(40),
			latestBlockNumber:                   uint64(52),
			expectedUpdateCall:                  true,
			expectedBlockFloorByEpoch:           uint64(50),
		},
		{
			name:                                "Sync should update as block difference is equal to or exceeds epoch 1000",
			registryLastUpdatedEpochBlockNumber: uint64(40),
			latestBlockNumber:                   uint64(1001),
			expectedUpdateCall:                  true,
			expectedBlockFloorByEpoch:           uint64(1000),
		},
		{
			name:                                "Sync should not update as block difference is less than epoch 48",
			registryLastUpdatedEpochBlockNumber: uint64(40),
			latestBlockNumber:                   uint64(48),
			expectedUpdateCall:                  false,
			expectedBlockFloorByEpoch:           uint64(40),
		},
		{
			name:                                "Sync should not update as block difference is less than epoch 30",
			registryLastUpdatedEpochBlockNumber: uint64(40),
			latestBlockNumber:                   uint64(30),
			expectedUpdateCall:                  false,
			expectedBlockFloorByEpoch:           uint64(40),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			dinMiddleware.Networks = map[string]*network{}
			dinMiddleware.registryLastUpdatedEpochBlockNumber = tt.registryLastUpdatedEpochBlockNumber

			// Check if update was called as expected
			if tt.expectedUpdateCall {
				mockDingoClient.EXPECT().GetRegistryData().Return(&dinsdk.DinRegistryData{}, nil).Times(1)
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
