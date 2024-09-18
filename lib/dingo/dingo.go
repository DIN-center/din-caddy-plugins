package dingo

import (
	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type DingoClient struct {
	Client *din.DinClient
	logger *zap.Logger
}

func NewDingoClient(logger *zap.Logger) (*DingoClient, error) {
	dingoClient, err := din.NewDinClient()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create dingo client")
	}
	return &DingoClient{
		Client: dingoClient,
		logger: logger,
	}, nil
}

func (d *DingoClient) GetDataFromRegistry() {
	networks, err := d.Client.GetRegistryData()
	if err != nil {
		d.logger.Error("Failed to get all networks", zap.Error(err))
		return
	}
	spew.Dump(networks)
}
