// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package sc_models

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// Method is an auto generated low-level Go binding around an user-defined struct.

// NetworkOperationsConfig is an auto generated low-level Go binding around an user-defined struct.

// ProviderAuthConfig is an auto generated low-level Go binding around an user-defined struct.

// DinRegistryHandlerMetaData contains all meta data concerning the DinRegistryHandler contract.
var DinRegistryHandlerMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[{\"name\":\"dinName\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"_owner\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"addMethodToNetwork\",\"inputs\":[{\"name\":\"name\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"method\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[{\"name\":\"bit\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"addMethodsToNetwork\",\"inputs\":[{\"name\":\"name\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"names\",\"type\":\"string[]\",\"internalType\":\"string[]\"}],\"outputs\":[{\"name\":\"capabilities\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"createNetwork\",\"inputs\":[{\"name\":\"name\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"description\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"config\",\"type\":\"tuple\",\"internalType\":\"structNetworkOperationsConfig\",\"components\":[{\"name\":\"healthcheckMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"healthcheckIntervalSec\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"chainIdMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"getBlockByNumberMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"callContractMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"blockLagLimit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"blockJumpLimit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"requestAttemptCount\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"maxRequestPayloadSizeKb\",\"type\":\"uint16\",\"internalType\":\"uint16\"},{\"name\":\"registryBlockEpoch\",\"type\":\"uint32\",\"internalType\":\"uint32\"},{\"name\":\"archiveEnabled\",\"type\":\"bool\",\"internalType\":\"bool\"},{\"name\":\"chainId\",\"type\":\"string\",\"internalType\":\"string\"}]},{\"name\":\"initialStatus\",\"type\":\"uint8\",\"internalType\":\"enumNetworkStatus\"}],\"outputs\":[{\"name\":\"newNetwork\",\"type\":\"address\",\"internalType\":\"contractINetwork\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"createNetworkService\",\"inputs\":[{\"name\":\"name\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"initialCaps\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"serviceUrl\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"serviceStatus\",\"type\":\"uint8\",\"internalType\":\"enumNetworkServiceStatus\"},{\"name\":\"locations\",\"type\":\"uint8[]\",\"internalType\":\"enumLocationType[]\"},{\"name\":\"provider\",\"type\":\"address\",\"internalType\":\"contractProvider\"}],\"outputs\":[{\"name\":\"networkService\",\"type\":\"address\",\"internalType\":\"contractNetworkService\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"createProvider\",\"inputs\":[{\"name\":\"providerEoa\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"name\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"authConfig\",\"type\":\"tuple\",\"internalType\":\"structProviderAuthConfig\",\"components\":[{\"name\":\"auth\",\"type\":\"uint8\",\"internalType\":\"enumProviderAuthType\"},{\"name\":\"url\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"apiKeyPlaceholder\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"useHeader\",\"type\":\"bool\",\"internalType\":\"bool\"}]},{\"name\":\"providerStatus\",\"type\":\"uint8\",\"internalType\":\"enumProviderStatus\"}],\"outputs\":[{\"name\":\"provider\",\"type\":\"address\",\"internalType\":\"contractProvider\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"dinOwner\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getAllNetworkMethodNames\",\"inputs\":[{\"name\":\"networkName\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[{\"name\":\"methods\",\"type\":\"string[]\",\"internalType\":\"string[]\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getAllNetworkMethods\",\"inputs\":[{\"name\":\"networkName\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[{\"name\":\"methods\",\"type\":\"tuple[]\",\"internalType\":\"structMethod[]\",\"components\":[{\"name\":\"name\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"bit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"deactivated\",\"type\":\"bool\",\"internalType\":\"bool\"}]}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getAllNetworks\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address[]\",\"internalType\":\"contractINetwork[]\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getAllProviders\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address[]\",\"internalType\":\"contractProvider[]\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getNetworkCapabilities\",\"inputs\":[{\"name\":\"networkName\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[{\"name\":\"caps\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getNetworkFromName\",\"inputs\":[{\"name\":\"networkName\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractINetwork\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getNetworkOperationsConfig\",\"inputs\":[{\"name\":\"networkName\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[{\"name\":\"opsConfig\",\"type\":\"tuple\",\"internalType\":\"structNetworkOperationsConfig\",\"components\":[{\"name\":\"healthcheckMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"healthcheckIntervalSec\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"chainIdMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"getBlockByNumberMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"callContractMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"blockLagLimit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"blockJumpLimit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"requestAttemptCount\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"maxRequestPayloadSizeKb\",\"type\":\"uint16\",\"internalType\":\"uint16\"},{\"name\":\"registryBlockEpoch\",\"type\":\"uint32\",\"internalType\":\"uint32\"},{\"name\":\"archiveEnabled\",\"type\":\"bool\",\"internalType\":\"bool\"},{\"name\":\"chainId\",\"type\":\"string\",\"internalType\":\"string\"}]}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getNetworkStatus\",\"inputs\":[{\"name\":\"networkName\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[{\"name\":\"status\",\"type\":\"uint8\",\"internalType\":\"enumNetworkStatus\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getProviders\",\"inputs\":[{\"name\":\"networkName\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[{\"name\":\"\",\"type\":\"address[]\",\"internalType\":\"contractProvider[]\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"network2providers\",\"inputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractINetwork\"},{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractProvider\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"networkMap\",\"inputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"networks\",\"inputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractINetwork\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"providerMap\",\"inputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractProvider\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"providerNetworkMap\",\"inputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"providers\",\"inputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractProvider\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"setNetworkOperationsConfig\",\"inputs\":[{\"name\":\"networkName\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"_opsConfig\",\"type\":\"tuple\",\"internalType\":\"structNetworkOperationsConfig\",\"components\":[{\"name\":\"healthcheckMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"healthcheckIntervalSec\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"chainIdMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"getBlockByNumberMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"callContractMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"blockLagLimit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"blockJumpLimit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"requestAttemptCount\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"maxRequestPayloadSizeKb\",\"type\":\"uint16\",\"internalType\":\"uint16\"},{\"name\":\"registryBlockEpoch\",\"type\":\"uint32\",\"internalType\":\"uint32\"},{\"name\":\"archiveEnabled\",\"type\":\"bool\",\"internalType\":\"bool\"},{\"name\":\"chainId\",\"type\":\"string\",\"internalType\":\"string\"}]}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setNetworkStatus\",\"inputs\":[{\"name\":\"networkName\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"status\",\"type\":\"uint8\",\"internalType\":\"enumNetworkStatus\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"unregisterProvider\",\"inputs\":[{\"name\":\"provider\",\"type\":\"address\",\"internalType\":\"contractProvider\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"error\",\"name\":\"AuthRequireDINOwner\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"AuthRequireProviderOwner\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"AuthRequireRegisteredProvider\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"NetworkExists\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"UnknownNetwork\",\"inputs\":[]}]",
}

// DinRegistryHandlerABI is the input ABI used to generate the binding from.
// Deprecated: Use DinRegistryHandlerMetaData.ABI instead.
var DinRegistryHandlerABI = DinRegistryHandlerMetaData.ABI

// DinRegistryHandler is an auto generated Go binding around an Ethereum contract.
type DinRegistryHandler struct {
	DinRegistryHandlerCaller     // Read-only binding to the contract
	DinRegistryHandlerTransactor // Write-only binding to the contract
	DinRegistryHandlerFilterer   // Log filterer for contract events
}

// DinRegistryHandlerCaller is an auto generated read-only Go binding around an Ethereum contract.
type DinRegistryHandlerCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// DinRegistryHandlerTransactor is an auto generated write-only Go binding around an Ethereum contract.
type DinRegistryHandlerTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// DinRegistryHandlerFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type DinRegistryHandlerFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// DinRegistryHandlerSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type DinRegistryHandlerSession struct {
	Contract     *DinRegistryHandler // Generic contract binding to set the session for
	CallOpts     bind.CallOpts       // Call options to use throughout this session
	TransactOpts bind.TransactOpts   // Transaction auth options to use throughout this session
}

// DinRegistryHandlerCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type DinRegistryHandlerCallerSession struct {
	Contract *DinRegistryHandlerCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts             // Call options to use throughout this session
}

// DinRegistryHandlerTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type DinRegistryHandlerTransactorSession struct {
	Contract     *DinRegistryHandlerTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts             // Transaction auth options to use throughout this session
}

// DinRegistryHandlerRaw is an auto generated low-level Go binding around an Ethereum contract.
type DinRegistryHandlerRaw struct {
	Contract *DinRegistryHandler // Generic contract binding to access the raw methods on
}

// DinRegistryHandlerCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type DinRegistryHandlerCallerRaw struct {
	Contract *DinRegistryHandlerCaller // Generic read-only contract binding to access the raw methods on
}

// DinRegistryHandlerTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type DinRegistryHandlerTransactorRaw struct {
	Contract *DinRegistryHandlerTransactor // Generic write-only contract binding to access the raw methods on
}

// NewDinRegistryHandler creates a new instance of DinRegistryHandler, bound to a specific deployed contract.
func NewDinRegistryHandler(address common.Address, backend bind.ContractBackend) (*DinRegistryHandler, error) {
	contract, err := bindDinRegistryHandler(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &DinRegistryHandler{DinRegistryHandlerCaller: DinRegistryHandlerCaller{contract: contract}, DinRegistryHandlerTransactor: DinRegistryHandlerTransactor{contract: contract}, DinRegistryHandlerFilterer: DinRegistryHandlerFilterer{contract: contract}}, nil
}

// NewDinRegistryHandlerCaller creates a new read-only instance of DinRegistryHandler, bound to a specific deployed contract.
func NewDinRegistryHandlerCaller(address common.Address, caller bind.ContractCaller) (*DinRegistryHandlerCaller, error) {
	contract, err := bindDinRegistryHandler(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &DinRegistryHandlerCaller{contract: contract}, nil
}

// NewDinRegistryHandlerTransactor creates a new write-only instance of DinRegistryHandler, bound to a specific deployed contract.
func NewDinRegistryHandlerTransactor(address common.Address, transactor bind.ContractTransactor) (*DinRegistryHandlerTransactor, error) {
	contract, err := bindDinRegistryHandler(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &DinRegistryHandlerTransactor{contract: contract}, nil
}

// NewDinRegistryHandlerFilterer creates a new log filterer instance of DinRegistryHandler, bound to a specific deployed contract.
func NewDinRegistryHandlerFilterer(address common.Address, filterer bind.ContractFilterer) (*DinRegistryHandlerFilterer, error) {
	contract, err := bindDinRegistryHandler(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &DinRegistryHandlerFilterer{contract: contract}, nil
}

// bindDinRegistryHandler binds a generic wrapper to an already deployed contract.
func bindDinRegistryHandler(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := DinRegistryHandlerMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_DinRegistryHandler *DinRegistryHandlerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _DinRegistryHandler.Contract.DinRegistryHandlerCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_DinRegistryHandler *DinRegistryHandlerRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _DinRegistryHandler.Contract.DinRegistryHandlerTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_DinRegistryHandler *DinRegistryHandlerRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _DinRegistryHandler.Contract.DinRegistryHandlerTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_DinRegistryHandler *DinRegistryHandlerCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _DinRegistryHandler.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_DinRegistryHandler *DinRegistryHandlerTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _DinRegistryHandler.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_DinRegistryHandler *DinRegistryHandlerTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _DinRegistryHandler.Contract.contract.Transact(opts, method, params...)
}

// DinOwner is a free data retrieval call binding the contract method 0xad158647.
//
// Solidity: function dinOwner() view returns(address)
func (_DinRegistryHandler *DinRegistryHandlerCaller) DinOwner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _DinRegistryHandler.contract.Call(opts, &out, "dinOwner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// DinOwner is a free data retrieval call binding the contract method 0xad158647.
//
// Solidity: function dinOwner() view returns(address)
func (_DinRegistryHandler *DinRegistryHandlerSession) DinOwner() (common.Address, error) {
	return _DinRegistryHandler.Contract.DinOwner(&_DinRegistryHandler.CallOpts)
}

// DinOwner is a free data retrieval call binding the contract method 0xad158647.
//
// Solidity: function dinOwner() view returns(address)
func (_DinRegistryHandler *DinRegistryHandlerCallerSession) DinOwner() (common.Address, error) {
	return _DinRegistryHandler.Contract.DinOwner(&_DinRegistryHandler.CallOpts)
}

// GetAllNetworkMethodNames is a free data retrieval call binding the contract method 0x5a079f71.
//
// Solidity: function getAllNetworkMethodNames(string networkName) view returns(string[] methods)
func (_DinRegistryHandler *DinRegistryHandlerCaller) GetAllNetworkMethodNames(opts *bind.CallOpts, networkName string) ([]string, error) {
	var out []interface{}
	err := _DinRegistryHandler.contract.Call(opts, &out, "getAllNetworkMethodNames", networkName)

	if err != nil {
		return *new([]string), err
	}

	out0 := *abi.ConvertType(out[0], new([]string)).(*[]string)

	return out0, err

}

// GetAllNetworkMethodNames is a free data retrieval call binding the contract method 0x5a079f71.
//
// Solidity: function getAllNetworkMethodNames(string networkName) view returns(string[] methods)
func (_DinRegistryHandler *DinRegistryHandlerSession) GetAllNetworkMethodNames(networkName string) ([]string, error) {
	return _DinRegistryHandler.Contract.GetAllNetworkMethodNames(&_DinRegistryHandler.CallOpts, networkName)
}

// GetAllNetworkMethodNames is a free data retrieval call binding the contract method 0x5a079f71.
//
// Solidity: function getAllNetworkMethodNames(string networkName) view returns(string[] methods)
func (_DinRegistryHandler *DinRegistryHandlerCallerSession) GetAllNetworkMethodNames(networkName string) ([]string, error) {
	return _DinRegistryHandler.Contract.GetAllNetworkMethodNames(&_DinRegistryHandler.CallOpts, networkName)
}

// GetAllNetworkMethods is a free data retrieval call binding the contract method 0x848f12ee.
//
// Solidity: function getAllNetworkMethods(string networkName) view returns((string,uint8,bool)[] methods)
func (_DinRegistryHandler *DinRegistryHandlerCaller) GetAllNetworkMethods(opts *bind.CallOpts, networkName string) ([]Method, error) {
	var out []interface{}
	err := _DinRegistryHandler.contract.Call(opts, &out, "getAllNetworkMethods", networkName)

	if err != nil {
		return *new([]Method), err
	}

	out0 := *abi.ConvertType(out[0], new([]Method)).(*[]Method)

	return out0, err

}

// GetAllNetworkMethods is a free data retrieval call binding the contract method 0x848f12ee.
//
// Solidity: function getAllNetworkMethods(string networkName) view returns((string,uint8,bool)[] methods)
func (_DinRegistryHandler *DinRegistryHandlerSession) GetAllNetworkMethods(networkName string) ([]Method, error) {
	return _DinRegistryHandler.Contract.GetAllNetworkMethods(&_DinRegistryHandler.CallOpts, networkName)
}

// GetAllNetworkMethods is a free data retrieval call binding the contract method 0x848f12ee.
//
// Solidity: function getAllNetworkMethods(string networkName) view returns((string,uint8,bool)[] methods)
func (_DinRegistryHandler *DinRegistryHandlerCallerSession) GetAllNetworkMethods(networkName string) ([]Method, error) {
	return _DinRegistryHandler.Contract.GetAllNetworkMethods(&_DinRegistryHandler.CallOpts, networkName)
}

// GetAllNetworks is a free data retrieval call binding the contract method 0x830c9850.
//
// Solidity: function getAllNetworks() view returns(address[])
func (_DinRegistryHandler *DinRegistryHandlerCaller) GetAllNetworks(opts *bind.CallOpts) ([]common.Address, error) {
	var out []interface{}
	err := _DinRegistryHandler.contract.Call(opts, &out, "getAllNetworks")

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// GetAllNetworks is a free data retrieval call binding the contract method 0x830c9850.
//
// Solidity: function getAllNetworks() view returns(address[])
func (_DinRegistryHandler *DinRegistryHandlerSession) GetAllNetworks() ([]common.Address, error) {
	return _DinRegistryHandler.Contract.GetAllNetworks(&_DinRegistryHandler.CallOpts)
}

// GetAllNetworks is a free data retrieval call binding the contract method 0x830c9850.
//
// Solidity: function getAllNetworks() view returns(address[])
func (_DinRegistryHandler *DinRegistryHandlerCallerSession) GetAllNetworks() ([]common.Address, error) {
	return _DinRegistryHandler.Contract.GetAllNetworks(&_DinRegistryHandler.CallOpts)
}

// GetAllProviders is a free data retrieval call binding the contract method 0x3bb4497c.
//
// Solidity: function getAllProviders() view returns(address[])
func (_DinRegistryHandler *DinRegistryHandlerCaller) GetAllProviders(opts *bind.CallOpts) ([]common.Address, error) {
	var out []interface{}
	err := _DinRegistryHandler.contract.Call(opts, &out, "getAllProviders")

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// GetAllProviders is a free data retrieval call binding the contract method 0x3bb4497c.
//
// Solidity: function getAllProviders() view returns(address[])
func (_DinRegistryHandler *DinRegistryHandlerSession) GetAllProviders() ([]common.Address, error) {
	return _DinRegistryHandler.Contract.GetAllProviders(&_DinRegistryHandler.CallOpts)
}

// GetAllProviders is a free data retrieval call binding the contract method 0x3bb4497c.
//
// Solidity: function getAllProviders() view returns(address[])
func (_DinRegistryHandler *DinRegistryHandlerCallerSession) GetAllProviders() ([]common.Address, error) {
	return _DinRegistryHandler.Contract.GetAllProviders(&_DinRegistryHandler.CallOpts)
}

// GetNetworkCapabilities is a free data retrieval call binding the contract method 0xb345cd16.
//
// Solidity: function getNetworkCapabilities(string networkName) view returns(uint256 caps)
func (_DinRegistryHandler *DinRegistryHandlerCaller) GetNetworkCapabilities(opts *bind.CallOpts, networkName string) (*big.Int, error) {
	var out []interface{}
	err := _DinRegistryHandler.contract.Call(opts, &out, "getNetworkCapabilities", networkName)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetNetworkCapabilities is a free data retrieval call binding the contract method 0xb345cd16.
//
// Solidity: function getNetworkCapabilities(string networkName) view returns(uint256 caps)
func (_DinRegistryHandler *DinRegistryHandlerSession) GetNetworkCapabilities(networkName string) (*big.Int, error) {
	return _DinRegistryHandler.Contract.GetNetworkCapabilities(&_DinRegistryHandler.CallOpts, networkName)
}

// GetNetworkCapabilities is a free data retrieval call binding the contract method 0xb345cd16.
//
// Solidity: function getNetworkCapabilities(string networkName) view returns(uint256 caps)
func (_DinRegistryHandler *DinRegistryHandlerCallerSession) GetNetworkCapabilities(networkName string) (*big.Int, error) {
	return _DinRegistryHandler.Contract.GetNetworkCapabilities(&_DinRegistryHandler.CallOpts, networkName)
}

// GetNetworkFromName is a free data retrieval call binding the contract method 0x47ea4dcc.
//
// Solidity: function getNetworkFromName(string networkName) view returns(address)
func (_DinRegistryHandler *DinRegistryHandlerCaller) GetNetworkFromName(opts *bind.CallOpts, networkName string) (common.Address, error) {
	var out []interface{}
	err := _DinRegistryHandler.contract.Call(opts, &out, "getNetworkFromName", networkName)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// GetNetworkFromName is a free data retrieval call binding the contract method 0x47ea4dcc.
//
// Solidity: function getNetworkFromName(string networkName) view returns(address)
func (_DinRegistryHandler *DinRegistryHandlerSession) GetNetworkFromName(networkName string) (common.Address, error) {
	return _DinRegistryHandler.Contract.GetNetworkFromName(&_DinRegistryHandler.CallOpts, networkName)
}

// GetNetworkFromName is a free data retrieval call binding the contract method 0x47ea4dcc.
//
// Solidity: function getNetworkFromName(string networkName) view returns(address)
func (_DinRegistryHandler *DinRegistryHandlerCallerSession) GetNetworkFromName(networkName string) (common.Address, error) {
	return _DinRegistryHandler.Contract.GetNetworkFromName(&_DinRegistryHandler.CallOpts, networkName)
}

// GetNetworkOperationsConfig is a free data retrieval call binding the contract method 0x5103b2c7.
//
// Solidity: function getNetworkOperationsConfig(string networkName) view returns((uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint16,uint32,bool,string) opsConfig)
func (_DinRegistryHandler *DinRegistryHandlerCaller) GetNetworkOperationsConfig(opts *bind.CallOpts, networkName string) (NetworkOperationsConfig, error) {
	var out []interface{}
	err := _DinRegistryHandler.contract.Call(opts, &out, "getNetworkOperationsConfig", networkName)

	if err != nil {
		return *new(NetworkOperationsConfig), err
	}

	out0 := *abi.ConvertType(out[0], new(NetworkOperationsConfig)).(*NetworkOperationsConfig)

	return out0, err

}

// GetNetworkOperationsConfig is a free data retrieval call binding the contract method 0x5103b2c7.
//
// Solidity: function getNetworkOperationsConfig(string networkName) view returns((uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint16,uint32,bool,string) opsConfig)
func (_DinRegistryHandler *DinRegistryHandlerSession) GetNetworkOperationsConfig(networkName string) (NetworkOperationsConfig, error) {
	return _DinRegistryHandler.Contract.GetNetworkOperationsConfig(&_DinRegistryHandler.CallOpts, networkName)
}

// GetNetworkOperationsConfig is a free data retrieval call binding the contract method 0x5103b2c7.
//
// Solidity: function getNetworkOperationsConfig(string networkName) view returns((uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint16,uint32,bool,string) opsConfig)
func (_DinRegistryHandler *DinRegistryHandlerCallerSession) GetNetworkOperationsConfig(networkName string) (NetworkOperationsConfig, error) {
	return _DinRegistryHandler.Contract.GetNetworkOperationsConfig(&_DinRegistryHandler.CallOpts, networkName)
}

// GetNetworkStatus is a free data retrieval call binding the contract method 0x0554b2e1.
//
// Solidity: function getNetworkStatus(string networkName) view returns(uint8 status)
func (_DinRegistryHandler *DinRegistryHandlerCaller) GetNetworkStatus(opts *bind.CallOpts, networkName string) (uint8, error) {
	var out []interface{}
	err := _DinRegistryHandler.contract.Call(opts, &out, "getNetworkStatus", networkName)

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// GetNetworkStatus is a free data retrieval call binding the contract method 0x0554b2e1.
//
// Solidity: function getNetworkStatus(string networkName) view returns(uint8 status)
func (_DinRegistryHandler *DinRegistryHandlerSession) GetNetworkStatus(networkName string) (uint8, error) {
	return _DinRegistryHandler.Contract.GetNetworkStatus(&_DinRegistryHandler.CallOpts, networkName)
}

// GetNetworkStatus is a free data retrieval call binding the contract method 0x0554b2e1.
//
// Solidity: function getNetworkStatus(string networkName) view returns(uint8 status)
func (_DinRegistryHandler *DinRegistryHandlerCallerSession) GetNetworkStatus(networkName string) (uint8, error) {
	return _DinRegistryHandler.Contract.GetNetworkStatus(&_DinRegistryHandler.CallOpts, networkName)
}

// GetProviders is a free data retrieval call binding the contract method 0x4633d7a0.
//
// Solidity: function getProviders(string networkName) view returns(address[])
func (_DinRegistryHandler *DinRegistryHandlerCaller) GetProviders(opts *bind.CallOpts, networkName string) ([]common.Address, error) {
	var out []interface{}
	err := _DinRegistryHandler.contract.Call(opts, &out, "getProviders", networkName)

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// GetProviders is a free data retrieval call binding the contract method 0x4633d7a0.
//
// Solidity: function getProviders(string networkName) view returns(address[])
func (_DinRegistryHandler *DinRegistryHandlerSession) GetProviders(networkName string) ([]common.Address, error) {
	return _DinRegistryHandler.Contract.GetProviders(&_DinRegistryHandler.CallOpts, networkName)
}

// GetProviders is a free data retrieval call binding the contract method 0x4633d7a0.
//
// Solidity: function getProviders(string networkName) view returns(address[])
func (_DinRegistryHandler *DinRegistryHandlerCallerSession) GetProviders(networkName string) ([]common.Address, error) {
	return _DinRegistryHandler.Contract.GetProviders(&_DinRegistryHandler.CallOpts, networkName)
}

// Network2providers is a free data retrieval call binding the contract method 0x7bc788cc.
//
// Solidity: function network2providers(address , uint256 ) view returns(address)
func (_DinRegistryHandler *DinRegistryHandlerCaller) Network2providers(opts *bind.CallOpts, arg0 common.Address, arg1 *big.Int) (common.Address, error) {
	var out []interface{}
	err := _DinRegistryHandler.contract.Call(opts, &out, "network2providers", arg0, arg1)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Network2providers is a free data retrieval call binding the contract method 0x7bc788cc.
//
// Solidity: function network2providers(address , uint256 ) view returns(address)
func (_DinRegistryHandler *DinRegistryHandlerSession) Network2providers(arg0 common.Address, arg1 *big.Int) (common.Address, error) {
	return _DinRegistryHandler.Contract.Network2providers(&_DinRegistryHandler.CallOpts, arg0, arg1)
}

// Network2providers is a free data retrieval call binding the contract method 0x7bc788cc.
//
// Solidity: function network2providers(address , uint256 ) view returns(address)
func (_DinRegistryHandler *DinRegistryHandlerCallerSession) Network2providers(arg0 common.Address, arg1 *big.Int) (common.Address, error) {
	return _DinRegistryHandler.Contract.Network2providers(&_DinRegistryHandler.CallOpts, arg0, arg1)
}

// NetworkMap is a free data retrieval call binding the contract method 0xb2f4eee8.
//
// Solidity: function networkMap(string ) view returns(bool)
func (_DinRegistryHandler *DinRegistryHandlerCaller) NetworkMap(opts *bind.CallOpts, arg0 string) (bool, error) {
	var out []interface{}
	err := _DinRegistryHandler.contract.Call(opts, &out, "networkMap", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// NetworkMap is a free data retrieval call binding the contract method 0xb2f4eee8.
//
// Solidity: function networkMap(string ) view returns(bool)
func (_DinRegistryHandler *DinRegistryHandlerSession) NetworkMap(arg0 string) (bool, error) {
	return _DinRegistryHandler.Contract.NetworkMap(&_DinRegistryHandler.CallOpts, arg0)
}

// NetworkMap is a free data retrieval call binding the contract method 0xb2f4eee8.
//
// Solidity: function networkMap(string ) view returns(bool)
func (_DinRegistryHandler *DinRegistryHandlerCallerSession) NetworkMap(arg0 string) (bool, error) {
	return _DinRegistryHandler.Contract.NetworkMap(&_DinRegistryHandler.CallOpts, arg0)
}

// Networks is a free data retrieval call binding the contract method 0x8bb0a17c.
//
// Solidity: function networks(uint256 ) view returns(address)
func (_DinRegistryHandler *DinRegistryHandlerCaller) Networks(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var out []interface{}
	err := _DinRegistryHandler.contract.Call(opts, &out, "networks", arg0)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Networks is a free data retrieval call binding the contract method 0x8bb0a17c.
//
// Solidity: function networks(uint256 ) view returns(address)
func (_DinRegistryHandler *DinRegistryHandlerSession) Networks(arg0 *big.Int) (common.Address, error) {
	return _DinRegistryHandler.Contract.Networks(&_DinRegistryHandler.CallOpts, arg0)
}

// Networks is a free data retrieval call binding the contract method 0x8bb0a17c.
//
// Solidity: function networks(uint256 ) view returns(address)
func (_DinRegistryHandler *DinRegistryHandlerCallerSession) Networks(arg0 *big.Int) (common.Address, error) {
	return _DinRegistryHandler.Contract.Networks(&_DinRegistryHandler.CallOpts, arg0)
}

// ProviderMap is a free data retrieval call binding the contract method 0xa6c87915.
//
// Solidity: function providerMap(address ) view returns(bool)
func (_DinRegistryHandler *DinRegistryHandlerCaller) ProviderMap(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var out []interface{}
	err := _DinRegistryHandler.contract.Call(opts, &out, "providerMap", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// ProviderMap is a free data retrieval call binding the contract method 0xa6c87915.
//
// Solidity: function providerMap(address ) view returns(bool)
func (_DinRegistryHandler *DinRegistryHandlerSession) ProviderMap(arg0 common.Address) (bool, error) {
	return _DinRegistryHandler.Contract.ProviderMap(&_DinRegistryHandler.CallOpts, arg0)
}

// ProviderMap is a free data retrieval call binding the contract method 0xa6c87915.
//
// Solidity: function providerMap(address ) view returns(bool)
func (_DinRegistryHandler *DinRegistryHandlerCallerSession) ProviderMap(arg0 common.Address) (bool, error) {
	return _DinRegistryHandler.Contract.ProviderMap(&_DinRegistryHandler.CallOpts, arg0)
}

// ProviderNetworkMap is a free data retrieval call binding the contract method 0x35707248.
//
// Solidity: function providerNetworkMap(bytes32 ) view returns(bool)
func (_DinRegistryHandler *DinRegistryHandlerCaller) ProviderNetworkMap(opts *bind.CallOpts, arg0 [32]byte) (bool, error) {
	var out []interface{}
	err := _DinRegistryHandler.contract.Call(opts, &out, "providerNetworkMap", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// ProviderNetworkMap is a free data retrieval call binding the contract method 0x35707248.
//
// Solidity: function providerNetworkMap(bytes32 ) view returns(bool)
func (_DinRegistryHandler *DinRegistryHandlerSession) ProviderNetworkMap(arg0 [32]byte) (bool, error) {
	return _DinRegistryHandler.Contract.ProviderNetworkMap(&_DinRegistryHandler.CallOpts, arg0)
}

// ProviderNetworkMap is a free data retrieval call binding the contract method 0x35707248.
//
// Solidity: function providerNetworkMap(bytes32 ) view returns(bool)
func (_DinRegistryHandler *DinRegistryHandlerCallerSession) ProviderNetworkMap(arg0 [32]byte) (bool, error) {
	return _DinRegistryHandler.Contract.ProviderNetworkMap(&_DinRegistryHandler.CallOpts, arg0)
}

// Providers is a free data retrieval call binding the contract method 0x50f3fc81.
//
// Solidity: function providers(uint256 ) view returns(address)
func (_DinRegistryHandler *DinRegistryHandlerCaller) Providers(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var out []interface{}
	err := _DinRegistryHandler.contract.Call(opts, &out, "providers", arg0)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Providers is a free data retrieval call binding the contract method 0x50f3fc81.
//
// Solidity: function providers(uint256 ) view returns(address)
func (_DinRegistryHandler *DinRegistryHandlerSession) Providers(arg0 *big.Int) (common.Address, error) {
	return _DinRegistryHandler.Contract.Providers(&_DinRegistryHandler.CallOpts, arg0)
}

// Providers is a free data retrieval call binding the contract method 0x50f3fc81.
//
// Solidity: function providers(uint256 ) view returns(address)
func (_DinRegistryHandler *DinRegistryHandlerCallerSession) Providers(arg0 *big.Int) (common.Address, error) {
	return _DinRegistryHandler.Contract.Providers(&_DinRegistryHandler.CallOpts, arg0)
}

// AddMethodToNetwork is a paid mutator transaction binding the contract method 0xda62a865.
//
// Solidity: function addMethodToNetwork(string name, string method) returns(uint8 bit)
func (_DinRegistryHandler *DinRegistryHandlerTransactor) AddMethodToNetwork(opts *bind.TransactOpts, name string, method string) (*types.Transaction, error) {
	return _DinRegistryHandler.contract.Transact(opts, "addMethodToNetwork", name, method)
}

// AddMethodToNetwork is a paid mutator transaction binding the contract method 0xda62a865.
//
// Solidity: function addMethodToNetwork(string name, string method) returns(uint8 bit)
func (_DinRegistryHandler *DinRegistryHandlerSession) AddMethodToNetwork(name string, method string) (*types.Transaction, error) {
	return _DinRegistryHandler.Contract.AddMethodToNetwork(&_DinRegistryHandler.TransactOpts, name, method)
}

// AddMethodToNetwork is a paid mutator transaction binding the contract method 0xda62a865.
//
// Solidity: function addMethodToNetwork(string name, string method) returns(uint8 bit)
func (_DinRegistryHandler *DinRegistryHandlerTransactorSession) AddMethodToNetwork(name string, method string) (*types.Transaction, error) {
	return _DinRegistryHandler.Contract.AddMethodToNetwork(&_DinRegistryHandler.TransactOpts, name, method)
}

// AddMethodsToNetwork is a paid mutator transaction binding the contract method 0x64cd3258.
//
// Solidity: function addMethodsToNetwork(string name, string[] names) returns(uint256 capabilities)
func (_DinRegistryHandler *DinRegistryHandlerTransactor) AddMethodsToNetwork(opts *bind.TransactOpts, name string, names []string) (*types.Transaction, error) {
	return _DinRegistryHandler.contract.Transact(opts, "addMethodsToNetwork", name, names)
}

// AddMethodsToNetwork is a paid mutator transaction binding the contract method 0x64cd3258.
//
// Solidity: function addMethodsToNetwork(string name, string[] names) returns(uint256 capabilities)
func (_DinRegistryHandler *DinRegistryHandlerSession) AddMethodsToNetwork(name string, names []string) (*types.Transaction, error) {
	return _DinRegistryHandler.Contract.AddMethodsToNetwork(&_DinRegistryHandler.TransactOpts, name, names)
}

// AddMethodsToNetwork is a paid mutator transaction binding the contract method 0x64cd3258.
//
// Solidity: function addMethodsToNetwork(string name, string[] names) returns(uint256 capabilities)
func (_DinRegistryHandler *DinRegistryHandlerTransactorSession) AddMethodsToNetwork(name string, names []string) (*types.Transaction, error) {
	return _DinRegistryHandler.Contract.AddMethodsToNetwork(&_DinRegistryHandler.TransactOpts, name, names)
}

// CreateNetwork is a paid mutator transaction binding the contract method 0xaf57c050.
//
// Solidity: function createNetwork(string name, string description, (uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint16,uint32,bool,string) config, uint8 initialStatus) returns(address newNetwork)
func (_DinRegistryHandler *DinRegistryHandlerTransactor) CreateNetwork(opts *bind.TransactOpts, name string, description string, config NetworkOperationsConfig, initialStatus uint8) (*types.Transaction, error) {
	return _DinRegistryHandler.contract.Transact(opts, "createNetwork", name, description, config, initialStatus)
}

// CreateNetwork is a paid mutator transaction binding the contract method 0xaf57c050.
//
// Solidity: function createNetwork(string name, string description, (uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint16,uint32,bool,string) config, uint8 initialStatus) returns(address newNetwork)
func (_DinRegistryHandler *DinRegistryHandlerSession) CreateNetwork(name string, description string, config NetworkOperationsConfig, initialStatus uint8) (*types.Transaction, error) {
	return _DinRegistryHandler.Contract.CreateNetwork(&_DinRegistryHandler.TransactOpts, name, description, config, initialStatus)
}

// CreateNetwork is a paid mutator transaction binding the contract method 0xaf57c050.
//
// Solidity: function createNetwork(string name, string description, (uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint16,uint32,bool,string) config, uint8 initialStatus) returns(address newNetwork)
func (_DinRegistryHandler *DinRegistryHandlerTransactorSession) CreateNetwork(name string, description string, config NetworkOperationsConfig, initialStatus uint8) (*types.Transaction, error) {
	return _DinRegistryHandler.Contract.CreateNetwork(&_DinRegistryHandler.TransactOpts, name, description, config, initialStatus)
}

// CreateNetworkService is a paid mutator transaction binding the contract method 0xc8991014.
//
// Solidity: function createNetworkService(string name, uint256 initialCaps, string serviceUrl, uint8 serviceStatus, uint8[] locations, address provider) returns(address networkService)
func (_DinRegistryHandler *DinRegistryHandlerTransactor) CreateNetworkService(opts *bind.TransactOpts, name string, initialCaps *big.Int, serviceUrl string, serviceStatus uint8, locations []uint8, provider common.Address) (*types.Transaction, error) {
	return _DinRegistryHandler.contract.Transact(opts, "createNetworkService", name, initialCaps, serviceUrl, serviceStatus, locations, provider)
}

// CreateNetworkService is a paid mutator transaction binding the contract method 0xc8991014.
//
// Solidity: function createNetworkService(string name, uint256 initialCaps, string serviceUrl, uint8 serviceStatus, uint8[] locations, address provider) returns(address networkService)
func (_DinRegistryHandler *DinRegistryHandlerSession) CreateNetworkService(name string, initialCaps *big.Int, serviceUrl string, serviceStatus uint8, locations []uint8, provider common.Address) (*types.Transaction, error) {
	return _DinRegistryHandler.Contract.CreateNetworkService(&_DinRegistryHandler.TransactOpts, name, initialCaps, serviceUrl, serviceStatus, locations, provider)
}

// CreateNetworkService is a paid mutator transaction binding the contract method 0xc8991014.
//
// Solidity: function createNetworkService(string name, uint256 initialCaps, string serviceUrl, uint8 serviceStatus, uint8[] locations, address provider) returns(address networkService)
func (_DinRegistryHandler *DinRegistryHandlerTransactorSession) CreateNetworkService(name string, initialCaps *big.Int, serviceUrl string, serviceStatus uint8, locations []uint8, provider common.Address) (*types.Transaction, error) {
	return _DinRegistryHandler.Contract.CreateNetworkService(&_DinRegistryHandler.TransactOpts, name, initialCaps, serviceUrl, serviceStatus, locations, provider)
}

// CreateProvider is a paid mutator transaction binding the contract method 0xe89a7fb4.
//
// Solidity: function createProvider(address providerEoa, string name, (uint8,string,string,bool) authConfig, uint8 providerStatus) returns(address provider)
func (_DinRegistryHandler *DinRegistryHandlerTransactor) CreateProvider(opts *bind.TransactOpts, providerEoa common.Address, name string, authConfig ProviderAuthConfig, providerStatus uint8) (*types.Transaction, error) {
	return _DinRegistryHandler.contract.Transact(opts, "createProvider", providerEoa, name, authConfig, providerStatus)
}

// CreateProvider is a paid mutator transaction binding the contract method 0xe89a7fb4.
//
// Solidity: function createProvider(address providerEoa, string name, (uint8,string,string,bool) authConfig, uint8 providerStatus) returns(address provider)
func (_DinRegistryHandler *DinRegistryHandlerSession) CreateProvider(providerEoa common.Address, name string, authConfig ProviderAuthConfig, providerStatus uint8) (*types.Transaction, error) {
	return _DinRegistryHandler.Contract.CreateProvider(&_DinRegistryHandler.TransactOpts, providerEoa, name, authConfig, providerStatus)
}

// CreateProvider is a paid mutator transaction binding the contract method 0xe89a7fb4.
//
// Solidity: function createProvider(address providerEoa, string name, (uint8,string,string,bool) authConfig, uint8 providerStatus) returns(address provider)
func (_DinRegistryHandler *DinRegistryHandlerTransactorSession) CreateProvider(providerEoa common.Address, name string, authConfig ProviderAuthConfig, providerStatus uint8) (*types.Transaction, error) {
	return _DinRegistryHandler.Contract.CreateProvider(&_DinRegistryHandler.TransactOpts, providerEoa, name, authConfig, providerStatus)
}

// SetNetworkOperationsConfig is a paid mutator transaction binding the contract method 0x2b804766.
//
// Solidity: function setNetworkOperationsConfig(string networkName, (uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint16,uint32,bool,string) _opsConfig) returns()
func (_DinRegistryHandler *DinRegistryHandlerTransactor) SetNetworkOperationsConfig(opts *bind.TransactOpts, networkName string, _opsConfig NetworkOperationsConfig) (*types.Transaction, error) {
	return _DinRegistryHandler.contract.Transact(opts, "setNetworkOperationsConfig", networkName, _opsConfig)
}

// SetNetworkOperationsConfig is a paid mutator transaction binding the contract method 0x2b804766.
//
// Solidity: function setNetworkOperationsConfig(string networkName, (uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint16,uint32,bool,string) _opsConfig) returns()
func (_DinRegistryHandler *DinRegistryHandlerSession) SetNetworkOperationsConfig(networkName string, _opsConfig NetworkOperationsConfig) (*types.Transaction, error) {
	return _DinRegistryHandler.Contract.SetNetworkOperationsConfig(&_DinRegistryHandler.TransactOpts, networkName, _opsConfig)
}

// SetNetworkOperationsConfig is a paid mutator transaction binding the contract method 0x2b804766.
//
// Solidity: function setNetworkOperationsConfig(string networkName, (uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint16,uint32,bool,string) _opsConfig) returns()
func (_DinRegistryHandler *DinRegistryHandlerTransactorSession) SetNetworkOperationsConfig(networkName string, _opsConfig NetworkOperationsConfig) (*types.Transaction, error) {
	return _DinRegistryHandler.Contract.SetNetworkOperationsConfig(&_DinRegistryHandler.TransactOpts, networkName, _opsConfig)
}

// SetNetworkStatus is a paid mutator transaction binding the contract method 0x34800cd9.
//
// Solidity: function setNetworkStatus(string networkName, uint8 status) returns()
func (_DinRegistryHandler *DinRegistryHandlerTransactor) SetNetworkStatus(opts *bind.TransactOpts, networkName string, status uint8) (*types.Transaction, error) {
	return _DinRegistryHandler.contract.Transact(opts, "setNetworkStatus", networkName, status)
}

// SetNetworkStatus is a paid mutator transaction binding the contract method 0x34800cd9.
//
// Solidity: function setNetworkStatus(string networkName, uint8 status) returns()
func (_DinRegistryHandler *DinRegistryHandlerSession) SetNetworkStatus(networkName string, status uint8) (*types.Transaction, error) {
	return _DinRegistryHandler.Contract.SetNetworkStatus(&_DinRegistryHandler.TransactOpts, networkName, status)
}

// SetNetworkStatus is a paid mutator transaction binding the contract method 0x34800cd9.
//
// Solidity: function setNetworkStatus(string networkName, uint8 status) returns()
func (_DinRegistryHandler *DinRegistryHandlerTransactorSession) SetNetworkStatus(networkName string, status uint8) (*types.Transaction, error) {
	return _DinRegistryHandler.Contract.SetNetworkStatus(&_DinRegistryHandler.TransactOpts, networkName, status)
}

// UnregisterProvider is a paid mutator transaction binding the contract method 0x05240e8b.
//
// Solidity: function unregisterProvider(address provider) returns()
func (_DinRegistryHandler *DinRegistryHandlerTransactor) UnregisterProvider(opts *bind.TransactOpts, provider common.Address) (*types.Transaction, error) {
	return _DinRegistryHandler.contract.Transact(opts, "unregisterProvider", provider)
}

// UnregisterProvider is a paid mutator transaction binding the contract method 0x05240e8b.
//
// Solidity: function unregisterProvider(address provider) returns()
func (_DinRegistryHandler *DinRegistryHandlerSession) UnregisterProvider(provider common.Address) (*types.Transaction, error) {
	return _DinRegistryHandler.Contract.UnregisterProvider(&_DinRegistryHandler.TransactOpts, provider)
}

// UnregisterProvider is a paid mutator transaction binding the contract method 0x05240e8b.
//
// Solidity: function unregisterProvider(address provider) returns()
func (_DinRegistryHandler *DinRegistryHandlerTransactorSession) UnregisterProvider(provider common.Address) (*types.Transaction, error) {
	return _DinRegistryHandler.Contract.UnregisterProvider(&_DinRegistryHandler.TransactOpts, provider)
}
