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

func getMethodNameByBit(methodsByBit map[uint8]*Method, bit uint8) string {
	method, ok := methodsByBit[bit]
	if !ok {
		return ""
	}
	return method.Name
}

func getMethodBitByName(methodsByName map[string]*Method, name string) (uint8, error) {
	method, ok := methodsByName[name]
	if !ok {
		return 0, errors.New("method not found: " + name)
	}
	return method.Bit, nil
}
