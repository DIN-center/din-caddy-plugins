// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package sc_models

type ProviderAuthConfig struct {
	Auth              uint8
	Url               string
	ApiKeyPlaceholder string
	UseHeader         bool
}

type Method struct {
	Name        string
	Bit         uint8
	Deactivated bool
}

type NetworkOperationsConfig struct {
	Handler                  string
	HealthcheckIntervalSec   uint8
	HealthcheckThreshold     uint8
	HealthcheckTimeout       uint16
	BlockLagLimit            uint8
	BlockJumpLimit           uint8
	RequestAttemptCount      uint8
	MaxRequestPayloadSizeKb  uint16
	RegistryBlockEpoch       uint32
	ArchiveEnabled           bool
	ProviderBlockHistorySize uint16
	NetworkBlockHistorySize  uint16
	ChainId                  string
}
