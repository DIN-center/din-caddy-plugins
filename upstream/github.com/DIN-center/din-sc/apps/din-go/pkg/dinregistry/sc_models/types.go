// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package sc_models

type Method struct {
	Name        string
	Bit         uint8
	Deactivated bool
}

type NetworkOperationsConfig struct {
	HealthcheckMethodBit      uint8
	HealthcheckIntervalSec    uint8
	ChainIdMethodBit          uint8
	GetBlockByNumberMethodBit uint8
	CallContractMethodBit     uint8
	BlockLagLimit             uint8
	BlockJumpLimit            uint8
	RequestAttemptCount       uint8
	MaxRequestPayloadSizeKb   uint16
	RegistryBlockEpoch        uint32
	ArchiveEnabled            bool
	ChainId                   string
}

type ProviderAuthConfig struct {
	Auth              uint8
	Url               string
	ApiKeyPlaceholder string
	UseHeader         bool
}

