package din

import (
	"strings"

	"github.com/pkg/errors"
)

// convertNetworkName converts the network name to a format that can be used in the DIN proxy
func convertNetworkName(name string) string {
	// Special cases for ethereum and polygon
	nameMap := map[string]string{
		"ethereum://mainnet": "eth",
		"ethereum://holesky": "holesky",
		"polygon://mainnet":  "polygon",
	}

	val, ok := nameMap[name]
	if ok {
		return val
	}

	parts := strings.Split(name, "://")
	result := strings.Join(parts, "-")
	return result
}

func mapRawLocationsCodeToName(locationsCode []uint8) ([]NetworkServiceLocation, error) {
	locationsName := make([]NetworkServiceLocation, len(locationsCode))
	for i, code := range locationsCode {
		locationName, err := NetworkServiceLocationFromCode(code)
		if err != nil {
			return nil, errors.Wrap(err, "failed to map location code")
		}
		locationsName[i] = locationName
	}
	return locationsName, nil
}
