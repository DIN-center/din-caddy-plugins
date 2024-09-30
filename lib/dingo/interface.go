package dingo

import (
	"github.com/DIN-center/din-sc/apps/din-go/lib/din"
)

type IDingoClient interface {
	GetDataFromRegistry() (*din.DinRegistryData, error)
}
