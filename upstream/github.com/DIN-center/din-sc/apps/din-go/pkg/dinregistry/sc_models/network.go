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

// NetworkHandlerMetaData contains all meta data concerning the NetworkHandler contract.
var NetworkHandlerMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[{\"name\":\"name\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"description\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"config\",\"type\":\"tuple\",\"internalType\":\"structNetworkOperationsConfig\",\"components\":[{\"name\":\"healthcheckMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"healthcheckIntervalSec\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"chainIdMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"getBlockByNumberMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"callContractMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"blockLagLimit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"blockJumpLimit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"requestAttemptCount\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"maxRequestPayloadSizeKb\",\"type\":\"uint16\",\"internalType\":\"uint16\"},{\"name\":\"registryBlockEpoch\",\"type\":\"uint32\",\"internalType\":\"uint32\"},{\"name\":\"archiveEnabled\",\"type\":\"bool\",\"internalType\":\"bool\"},{\"name\":\"chainId\",\"type\":\"string\",\"internalType\":\"string\"}]},{\"name\":\"initialStatus\",\"type\":\"uint8\",\"internalType\":\"enumNetworkStatus\"},{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"addMethod\",\"inputs\":[{\"name\":\"name\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[{\"name\":\"bit\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"addMethods\",\"inputs\":[{\"name\":\"names\",\"type\":\"string[]\",\"internalType\":\"string[]\"}],\"outputs\":[{\"name\":\"updatedCapabilities\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"allMethods\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"tuple[]\",\"internalType\":\"structMethod[]\",\"components\":[{\"name\":\"name\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"bit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"deactivated\",\"type\":\"bool\",\"internalType\":\"bool\"}]}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"areCapabilitiesSupported\",\"inputs\":[{\"name\":\"caps\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"supported\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"authenticated\",\"inputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getCapabilities\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getDescription\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getMethodId\",\"inputs\":[{\"name\":\"name\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[{\"name\":\"bit\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getMethodName\",\"inputs\":[{\"name\":\"bit\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"outputs\":[{\"name\":\"name\",\"type\":\"string\",\"internalType\":\"string\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getName\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getNetworkOperationsConfig\",\"inputs\":[],\"outputs\":[{\"name\":\"config\",\"type\":\"tuple\",\"internalType\":\"structNetworkOperationsConfig\",\"components\":[{\"name\":\"healthcheckMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"healthcheckIntervalSec\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"chainIdMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"getBlockByNumberMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"callContractMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"blockLagLimit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"blockJumpLimit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"requestAttemptCount\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"maxRequestPayloadSizeKb\",\"type\":\"uint16\",\"internalType\":\"uint16\"},{\"name\":\"registryBlockEpoch\",\"type\":\"uint32\",\"internalType\":\"uint32\"},{\"name\":\"archiveEnabled\",\"type\":\"bool\",\"internalType\":\"bool\"},{\"name\":\"chainId\",\"type\":\"string\",\"internalType\":\"string\"}]}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getNetworkStatus\",\"inputs\":[],\"outputs\":[{\"name\":\"status\",\"type\":\"uint8\",\"internalType\":\"enumNetworkStatus\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"isMethodSupported\",\"inputs\":[{\"name\":\"bit\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"outputs\":[{\"name\":\"supported\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"methods\",\"inputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"name\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"bit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"deactivated\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"networkOwner\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"removeMethod\",\"inputs\":[{\"name\":\"bit\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"removeMethod\",\"inputs\":[{\"name\":\"name\",\"type\":\"string\",\"internalType\":\"string\"}],\"outputs\":[{\"name\":\"bit\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"s_bit2method\",\"inputs\":[{\"name\":\"\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"outputs\":[{\"name\":\"name\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"bit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"deactivated\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"setNetworkOperationsConfig\",\"inputs\":[{\"name\":\"config\",\"type\":\"tuple\",\"internalType\":\"structNetworkOperationsConfig\",\"components\":[{\"name\":\"healthcheckMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"healthcheckIntervalSec\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"chainIdMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"getBlockByNumberMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"callContractMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"blockLagLimit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"blockJumpLimit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"requestAttemptCount\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"maxRequestPayloadSizeKb\",\"type\":\"uint16\",\"internalType\":\"uint16\"},{\"name\":\"registryBlockEpoch\",\"type\":\"uint32\",\"internalType\":\"uint32\"},{\"name\":\"archiveEnabled\",\"type\":\"bool\",\"internalType\":\"bool\"},{\"name\":\"chainId\",\"type\":\"string\",\"internalType\":\"string\"}]}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setNetworkStatus\",\"inputs\":[{\"name\":\"status\",\"type\":\"uint8\",\"internalType\":\"enumNetworkStatus\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"event\",\"name\":\"AddMethodToNetwork\",\"inputs\":[{\"name\":\"network\",\"type\":\"address\",\"indexed\":false,\"internalType\":\"contractNetwork\"},{\"name\":\"method\",\"type\":\"string\",\"indexed\":false,\"internalType\":\"string\"},{\"name\":\"bit\",\"type\":\"uint8\",\"indexed\":false,\"internalType\":\"uint8\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"NetworkOperationConfigUpdated\",\"inputs\":[{\"name\":\"network\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"contractNetwork\"},{\"name\":\"newConfig\",\"type\":\"tuple\",\"indexed\":false,\"internalType\":\"structNetworkOperationsConfig\",\"components\":[{\"name\":\"healthcheckMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"healthcheckIntervalSec\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"chainIdMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"getBlockByNumberMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"callContractMethodBit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"blockLagLimit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"blockJumpLimit\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"requestAttemptCount\",\"type\":\"uint8\",\"internalType\":\"uint8\"},{\"name\":\"maxRequestPayloadSizeKb\",\"type\":\"uint16\",\"internalType\":\"uint16\"},{\"name\":\"registryBlockEpoch\",\"type\":\"uint32\",\"internalType\":\"uint32\"},{\"name\":\"archiveEnabled\",\"type\":\"bool\",\"internalType\":\"bool\"},{\"name\":\"chainId\",\"type\":\"string\",\"internalType\":\"string\"}]}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"NetworkStatusUpdated\",\"inputs\":[{\"name\":\"network\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"contractNetwork\"},{\"name\":\"newStatus\",\"type\":\"uint8\",\"indexed\":false,\"internalType\":\"enumNetworkStatus\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RemoveMethodFromNetwork\",\"inputs\":[{\"name\":\"network\",\"type\":\"address\",\"indexed\":false,\"internalType\":\"contractNetwork\"},{\"name\":\"method\",\"type\":\"string\",\"indexed\":false,\"internalType\":\"string\"},{\"name\":\"bit\",\"type\":\"uint8\",\"indexed\":false,\"internalType\":\"uint8\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"NetworkAuthNotAuthenticated\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"NetworkAuthNotOwner\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"NetworkInvalidHealthcheckMethod\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"NetworkInvalidStatus\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"NetworkMethodAlreadyExists\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"NetworkMethodDeactivated\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"NetworkMethodNotFound\",\"inputs\":[]}]",
}

// NetworkHandlerABI is the input ABI used to generate the binding from.
// Deprecated: Use NetworkHandlerMetaData.ABI instead.
var NetworkHandlerABI = NetworkHandlerMetaData.ABI

// NetworkHandler is an auto generated Go binding around an Ethereum contract.
type NetworkHandler struct {
	NetworkHandlerCaller     // Read-only binding to the contract
	NetworkHandlerTransactor // Write-only binding to the contract
	NetworkHandlerFilterer   // Log filterer for contract events
}

// NetworkHandlerCaller is an auto generated read-only Go binding around an Ethereum contract.
type NetworkHandlerCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NetworkHandlerTransactor is an auto generated write-only Go binding around an Ethereum contract.
type NetworkHandlerTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NetworkHandlerFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type NetworkHandlerFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NetworkHandlerSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type NetworkHandlerSession struct {
	Contract     *NetworkHandler   // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// NetworkHandlerCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type NetworkHandlerCallerSession struct {
	Contract *NetworkHandlerCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts         // Call options to use throughout this session
}

// NetworkHandlerTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type NetworkHandlerTransactorSession struct {
	Contract     *NetworkHandlerTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts         // Transaction auth options to use throughout this session
}

// NetworkHandlerRaw is an auto generated low-level Go binding around an Ethereum contract.
type NetworkHandlerRaw struct {
	Contract *NetworkHandler // Generic contract binding to access the raw methods on
}

// NetworkHandlerCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type NetworkHandlerCallerRaw struct {
	Contract *NetworkHandlerCaller // Generic read-only contract binding to access the raw methods on
}

// NetworkHandlerTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type NetworkHandlerTransactorRaw struct {
	Contract *NetworkHandlerTransactor // Generic write-only contract binding to access the raw methods on
}

// NewNetworkHandler creates a new instance of NetworkHandler, bound to a specific deployed contract.
func NewNetworkHandler(address common.Address, backend bind.ContractBackend) (*NetworkHandler, error) {
	contract, err := bindNetworkHandler(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &NetworkHandler{NetworkHandlerCaller: NetworkHandlerCaller{contract: contract}, NetworkHandlerTransactor: NetworkHandlerTransactor{contract: contract}, NetworkHandlerFilterer: NetworkHandlerFilterer{contract: contract}}, nil
}

// NewNetworkHandlerCaller creates a new read-only instance of NetworkHandler, bound to a specific deployed contract.
func NewNetworkHandlerCaller(address common.Address, caller bind.ContractCaller) (*NetworkHandlerCaller, error) {
	contract, err := bindNetworkHandler(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &NetworkHandlerCaller{contract: contract}, nil
}

// NewNetworkHandlerTransactor creates a new write-only instance of NetworkHandler, bound to a specific deployed contract.
func NewNetworkHandlerTransactor(address common.Address, transactor bind.ContractTransactor) (*NetworkHandlerTransactor, error) {
	contract, err := bindNetworkHandler(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &NetworkHandlerTransactor{contract: contract}, nil
}

// NewNetworkHandlerFilterer creates a new log filterer instance of NetworkHandler, bound to a specific deployed contract.
func NewNetworkHandlerFilterer(address common.Address, filterer bind.ContractFilterer) (*NetworkHandlerFilterer, error) {
	contract, err := bindNetworkHandler(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &NetworkHandlerFilterer{contract: contract}, nil
}

// bindNetworkHandler binds a generic wrapper to an already deployed contract.
func bindNetworkHandler(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := NetworkHandlerMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_NetworkHandler *NetworkHandlerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _NetworkHandler.Contract.NetworkHandlerCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_NetworkHandler *NetworkHandlerRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NetworkHandler.Contract.NetworkHandlerTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_NetworkHandler *NetworkHandlerRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _NetworkHandler.Contract.NetworkHandlerTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_NetworkHandler *NetworkHandlerCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _NetworkHandler.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_NetworkHandler *NetworkHandlerTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NetworkHandler.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_NetworkHandler *NetworkHandlerTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _NetworkHandler.Contract.contract.Transact(opts, method, params...)
}

// AllMethods is a free data retrieval call binding the contract method 0x6dadf1a2.
//
// Solidity: function allMethods() view returns((string,uint8,bool)[])
func (_NetworkHandler *NetworkHandlerCaller) AllMethods(opts *bind.CallOpts) ([]Method, error) {
	var out []interface{}
	err := _NetworkHandler.contract.Call(opts, &out, "allMethods")

	if err != nil {
		return *new([]Method), err
	}

	out0 := *abi.ConvertType(out[0], new([]Method)).(*[]Method)

	return out0, err

}

// AllMethods is a free data retrieval call binding the contract method 0x6dadf1a2.
//
// Solidity: function allMethods() view returns((string,uint8,bool)[])
func (_NetworkHandler *NetworkHandlerSession) AllMethods() ([]Method, error) {
	return _NetworkHandler.Contract.AllMethods(&_NetworkHandler.CallOpts)
}

// AllMethods is a free data retrieval call binding the contract method 0x6dadf1a2.
//
// Solidity: function allMethods() view returns((string,uint8,bool)[])
func (_NetworkHandler *NetworkHandlerCallerSession) AllMethods() ([]Method, error) {
	return _NetworkHandler.Contract.AllMethods(&_NetworkHandler.CallOpts)
}

// AreCapabilitiesSupported is a free data retrieval call binding the contract method 0x50e80f47.
//
// Solidity: function areCapabilitiesSupported(uint256 caps) view returns(bool supported)
func (_NetworkHandler *NetworkHandlerCaller) AreCapabilitiesSupported(opts *bind.CallOpts, caps *big.Int) (bool, error) {
	var out []interface{}
	err := _NetworkHandler.contract.Call(opts, &out, "areCapabilitiesSupported", caps)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// AreCapabilitiesSupported is a free data retrieval call binding the contract method 0x50e80f47.
//
// Solidity: function areCapabilitiesSupported(uint256 caps) view returns(bool supported)
func (_NetworkHandler *NetworkHandlerSession) AreCapabilitiesSupported(caps *big.Int) (bool, error) {
	return _NetworkHandler.Contract.AreCapabilitiesSupported(&_NetworkHandler.CallOpts, caps)
}

// AreCapabilitiesSupported is a free data retrieval call binding the contract method 0x50e80f47.
//
// Solidity: function areCapabilitiesSupported(uint256 caps) view returns(bool supported)
func (_NetworkHandler *NetworkHandlerCallerSession) AreCapabilitiesSupported(caps *big.Int) (bool, error) {
	return _NetworkHandler.Contract.AreCapabilitiesSupported(&_NetworkHandler.CallOpts, caps)
}

// Authenticated is a free data retrieval call binding the contract method 0xd9a1693b.
//
// Solidity: function authenticated(address ) view returns(bool)
func (_NetworkHandler *NetworkHandlerCaller) Authenticated(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var out []interface{}
	err := _NetworkHandler.contract.Call(opts, &out, "authenticated", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// Authenticated is a free data retrieval call binding the contract method 0xd9a1693b.
//
// Solidity: function authenticated(address ) view returns(bool)
func (_NetworkHandler *NetworkHandlerSession) Authenticated(arg0 common.Address) (bool, error) {
	return _NetworkHandler.Contract.Authenticated(&_NetworkHandler.CallOpts, arg0)
}

// Authenticated is a free data retrieval call binding the contract method 0xd9a1693b.
//
// Solidity: function authenticated(address ) view returns(bool)
func (_NetworkHandler *NetworkHandlerCallerSession) Authenticated(arg0 common.Address) (bool, error) {
	return _NetworkHandler.Contract.Authenticated(&_NetworkHandler.CallOpts, arg0)
}

// GetCapabilities is a free data retrieval call binding the contract method 0xddbe4f82.
//
// Solidity: function getCapabilities() view returns(uint256)
func (_NetworkHandler *NetworkHandlerCaller) GetCapabilities(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _NetworkHandler.contract.Call(opts, &out, "getCapabilities")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetCapabilities is a free data retrieval call binding the contract method 0xddbe4f82.
//
// Solidity: function getCapabilities() view returns(uint256)
func (_NetworkHandler *NetworkHandlerSession) GetCapabilities() (*big.Int, error) {
	return _NetworkHandler.Contract.GetCapabilities(&_NetworkHandler.CallOpts)
}

// GetCapabilities is a free data retrieval call binding the contract method 0xddbe4f82.
//
// Solidity: function getCapabilities() view returns(uint256)
func (_NetworkHandler *NetworkHandlerCallerSession) GetCapabilities() (*big.Int, error) {
	return _NetworkHandler.Contract.GetCapabilities(&_NetworkHandler.CallOpts)
}

// GetDescription is a free data retrieval call binding the contract method 0x1a092541.
//
// Solidity: function getDescription() view returns(string)
func (_NetworkHandler *NetworkHandlerCaller) GetDescription(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _NetworkHandler.contract.Call(opts, &out, "getDescription")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// GetDescription is a free data retrieval call binding the contract method 0x1a092541.
//
// Solidity: function getDescription() view returns(string)
func (_NetworkHandler *NetworkHandlerSession) GetDescription() (string, error) {
	return _NetworkHandler.Contract.GetDescription(&_NetworkHandler.CallOpts)
}

// GetDescription is a free data retrieval call binding the contract method 0x1a092541.
//
// Solidity: function getDescription() view returns(string)
func (_NetworkHandler *NetworkHandlerCallerSession) GetDescription() (string, error) {
	return _NetworkHandler.Contract.GetDescription(&_NetworkHandler.CallOpts)
}

// GetMethodId is a free data retrieval call binding the contract method 0x4e86dc8c.
//
// Solidity: function getMethodId(string name) view returns(uint8 bit)
func (_NetworkHandler *NetworkHandlerCaller) GetMethodId(opts *bind.CallOpts, name string) (uint8, error) {
	var out []interface{}
	err := _NetworkHandler.contract.Call(opts, &out, "getMethodId", name)

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// GetMethodId is a free data retrieval call binding the contract method 0x4e86dc8c.
//
// Solidity: function getMethodId(string name) view returns(uint8 bit)
func (_NetworkHandler *NetworkHandlerSession) GetMethodId(name string) (uint8, error) {
	return _NetworkHandler.Contract.GetMethodId(&_NetworkHandler.CallOpts, name)
}

// GetMethodId is a free data retrieval call binding the contract method 0x4e86dc8c.
//
// Solidity: function getMethodId(string name) view returns(uint8 bit)
func (_NetworkHandler *NetworkHandlerCallerSession) GetMethodId(name string) (uint8, error) {
	return _NetworkHandler.Contract.GetMethodId(&_NetworkHandler.CallOpts, name)
}

// GetMethodName is a free data retrieval call binding the contract method 0x2bed395c.
//
// Solidity: function getMethodName(uint8 bit) view returns(string name)
func (_NetworkHandler *NetworkHandlerCaller) GetMethodName(opts *bind.CallOpts, bit uint8) (string, error) {
	var out []interface{}
	err := _NetworkHandler.contract.Call(opts, &out, "getMethodName", bit)

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// GetMethodName is a free data retrieval call binding the contract method 0x2bed395c.
//
// Solidity: function getMethodName(uint8 bit) view returns(string name)
func (_NetworkHandler *NetworkHandlerSession) GetMethodName(bit uint8) (string, error) {
	return _NetworkHandler.Contract.GetMethodName(&_NetworkHandler.CallOpts, bit)
}

// GetMethodName is a free data retrieval call binding the contract method 0x2bed395c.
//
// Solidity: function getMethodName(uint8 bit) view returns(string name)
func (_NetworkHandler *NetworkHandlerCallerSession) GetMethodName(bit uint8) (string, error) {
	return _NetworkHandler.Contract.GetMethodName(&_NetworkHandler.CallOpts, bit)
}

// GetName is a free data retrieval call binding the contract method 0x17d7de7c.
//
// Solidity: function getName() view returns(string)
func (_NetworkHandler *NetworkHandlerCaller) GetName(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _NetworkHandler.contract.Call(opts, &out, "getName")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// GetName is a free data retrieval call binding the contract method 0x17d7de7c.
//
// Solidity: function getName() view returns(string)
func (_NetworkHandler *NetworkHandlerSession) GetName() (string, error) {
	return _NetworkHandler.Contract.GetName(&_NetworkHandler.CallOpts)
}

// GetName is a free data retrieval call binding the contract method 0x17d7de7c.
//
// Solidity: function getName() view returns(string)
func (_NetworkHandler *NetworkHandlerCallerSession) GetName() (string, error) {
	return _NetworkHandler.Contract.GetName(&_NetworkHandler.CallOpts)
}

// GetNetworkOperationsConfig is a free data retrieval call binding the contract method 0x4ed9b978.
//
// Solidity: function getNetworkOperationsConfig() view returns((uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint16,uint32,bool,string) config)
func (_NetworkHandler *NetworkHandlerCaller) GetNetworkOperationsConfig(opts *bind.CallOpts) (NetworkOperationsConfig, error) {
	var out []interface{}
	err := _NetworkHandler.contract.Call(opts, &out, "getNetworkOperationsConfig")

	if err != nil {
		return *new(NetworkOperationsConfig), err
	}

	out0 := *abi.ConvertType(out[0], new(NetworkOperationsConfig)).(*NetworkOperationsConfig)

	return out0, err

}

// GetNetworkOperationsConfig is a free data retrieval call binding the contract method 0x4ed9b978.
//
// Solidity: function getNetworkOperationsConfig() view returns((uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint16,uint32,bool,string) config)
func (_NetworkHandler *NetworkHandlerSession) GetNetworkOperationsConfig() (NetworkOperationsConfig, error) {
	return _NetworkHandler.Contract.GetNetworkOperationsConfig(&_NetworkHandler.CallOpts)
}

// GetNetworkOperationsConfig is a free data retrieval call binding the contract method 0x4ed9b978.
//
// Solidity: function getNetworkOperationsConfig() view returns((uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint16,uint32,bool,string) config)
func (_NetworkHandler *NetworkHandlerCallerSession) GetNetworkOperationsConfig() (NetworkOperationsConfig, error) {
	return _NetworkHandler.Contract.GetNetworkOperationsConfig(&_NetworkHandler.CallOpts)
}

// GetNetworkStatus is a free data retrieval call binding the contract method 0x70c073f2.
//
// Solidity: function getNetworkStatus() view returns(uint8 status)
func (_NetworkHandler *NetworkHandlerCaller) GetNetworkStatus(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _NetworkHandler.contract.Call(opts, &out, "getNetworkStatus")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// GetNetworkStatus is a free data retrieval call binding the contract method 0x70c073f2.
//
// Solidity: function getNetworkStatus() view returns(uint8 status)
func (_NetworkHandler *NetworkHandlerSession) GetNetworkStatus() (uint8, error) {
	return _NetworkHandler.Contract.GetNetworkStatus(&_NetworkHandler.CallOpts)
}

// GetNetworkStatus is a free data retrieval call binding the contract method 0x70c073f2.
//
// Solidity: function getNetworkStatus() view returns(uint8 status)
func (_NetworkHandler *NetworkHandlerCallerSession) GetNetworkStatus() (uint8, error) {
	return _NetworkHandler.Contract.GetNetworkStatus(&_NetworkHandler.CallOpts)
}

// IsMethodSupported is a free data retrieval call binding the contract method 0xbd822ebf.
//
// Solidity: function isMethodSupported(uint8 bit) view returns(bool supported)
func (_NetworkHandler *NetworkHandlerCaller) IsMethodSupported(opts *bind.CallOpts, bit uint8) (bool, error) {
	var out []interface{}
	err := _NetworkHandler.contract.Call(opts, &out, "isMethodSupported", bit)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsMethodSupported is a free data retrieval call binding the contract method 0xbd822ebf.
//
// Solidity: function isMethodSupported(uint8 bit) view returns(bool supported)
func (_NetworkHandler *NetworkHandlerSession) IsMethodSupported(bit uint8) (bool, error) {
	return _NetworkHandler.Contract.IsMethodSupported(&_NetworkHandler.CallOpts, bit)
}

// IsMethodSupported is a free data retrieval call binding the contract method 0xbd822ebf.
//
// Solidity: function isMethodSupported(uint8 bit) view returns(bool supported)
func (_NetworkHandler *NetworkHandlerCallerSession) IsMethodSupported(bit uint8) (bool, error) {
	return _NetworkHandler.Contract.IsMethodSupported(&_NetworkHandler.CallOpts, bit)
}

// Methods is a free data retrieval call binding the contract method 0x887bf3ef.
//
// Solidity: function methods(uint256 ) view returns(string name, uint8 bit, bool deactivated)
func (_NetworkHandler *NetworkHandlerCaller) Methods(opts *bind.CallOpts, arg0 *big.Int) (struct {
	Name        string
	Bit         uint8
	Deactivated bool
}, error) {
	var out []interface{}
	err := _NetworkHandler.contract.Call(opts, &out, "methods", arg0)

	outstruct := new(struct {
		Name        string
		Bit         uint8
		Deactivated bool
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Name = *abi.ConvertType(out[0], new(string)).(*string)
	outstruct.Bit = *abi.ConvertType(out[1], new(uint8)).(*uint8)
	outstruct.Deactivated = *abi.ConvertType(out[2], new(bool)).(*bool)

	return *outstruct, err

}

// Methods is a free data retrieval call binding the contract method 0x887bf3ef.
//
// Solidity: function methods(uint256 ) view returns(string name, uint8 bit, bool deactivated)
func (_NetworkHandler *NetworkHandlerSession) Methods(arg0 *big.Int) (struct {
	Name        string
	Bit         uint8
	Deactivated bool
}, error) {
	return _NetworkHandler.Contract.Methods(&_NetworkHandler.CallOpts, arg0)
}

// Methods is a free data retrieval call binding the contract method 0x887bf3ef.
//
// Solidity: function methods(uint256 ) view returns(string name, uint8 bit, bool deactivated)
func (_NetworkHandler *NetworkHandlerCallerSession) Methods(arg0 *big.Int) (struct {
	Name        string
	Bit         uint8
	Deactivated bool
}, error) {
	return _NetworkHandler.Contract.Methods(&_NetworkHandler.CallOpts, arg0)
}

// NetworkOwner is a free data retrieval call binding the contract method 0x2e1b181e.
//
// Solidity: function networkOwner() view returns(address)
func (_NetworkHandler *NetworkHandlerCaller) NetworkOwner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _NetworkHandler.contract.Call(opts, &out, "networkOwner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// NetworkOwner is a free data retrieval call binding the contract method 0x2e1b181e.
//
// Solidity: function networkOwner() view returns(address)
func (_NetworkHandler *NetworkHandlerSession) NetworkOwner() (common.Address, error) {
	return _NetworkHandler.Contract.NetworkOwner(&_NetworkHandler.CallOpts)
}

// NetworkOwner is a free data retrieval call binding the contract method 0x2e1b181e.
//
// Solidity: function networkOwner() view returns(address)
func (_NetworkHandler *NetworkHandlerCallerSession) NetworkOwner() (common.Address, error) {
	return _NetworkHandler.Contract.NetworkOwner(&_NetworkHandler.CallOpts)
}

// SBit2method is a free data retrieval call binding the contract method 0x6388f37f.
//
// Solidity: function s_bit2method(uint8 ) view returns(string name, uint8 bit, bool deactivated)
func (_NetworkHandler *NetworkHandlerCaller) SBit2method(opts *bind.CallOpts, arg0 uint8) (struct {
	Name        string
	Bit         uint8
	Deactivated bool
}, error) {
	var out []interface{}
	err := _NetworkHandler.contract.Call(opts, &out, "s_bit2method", arg0)

	outstruct := new(struct {
		Name        string
		Bit         uint8
		Deactivated bool
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Name = *abi.ConvertType(out[0], new(string)).(*string)
	outstruct.Bit = *abi.ConvertType(out[1], new(uint8)).(*uint8)
	outstruct.Deactivated = *abi.ConvertType(out[2], new(bool)).(*bool)

	return *outstruct, err

}

// SBit2method is a free data retrieval call binding the contract method 0x6388f37f.
//
// Solidity: function s_bit2method(uint8 ) view returns(string name, uint8 bit, bool deactivated)
func (_NetworkHandler *NetworkHandlerSession) SBit2method(arg0 uint8) (struct {
	Name        string
	Bit         uint8
	Deactivated bool
}, error) {
	return _NetworkHandler.Contract.SBit2method(&_NetworkHandler.CallOpts, arg0)
}

// SBit2method is a free data retrieval call binding the contract method 0x6388f37f.
//
// Solidity: function s_bit2method(uint8 ) view returns(string name, uint8 bit, bool deactivated)
func (_NetworkHandler *NetworkHandlerCallerSession) SBit2method(arg0 uint8) (struct {
	Name        string
	Bit         uint8
	Deactivated bool
}, error) {
	return _NetworkHandler.Contract.SBit2method(&_NetworkHandler.CallOpts, arg0)
}

// AddMethod is a paid mutator transaction binding the contract method 0x51542e3a.
//
// Solidity: function addMethod(string name) returns(uint8 bit)
func (_NetworkHandler *NetworkHandlerTransactor) AddMethod(opts *bind.TransactOpts, name string) (*types.Transaction, error) {
	return _NetworkHandler.contract.Transact(opts, "addMethod", name)
}

// AddMethod is a paid mutator transaction binding the contract method 0x51542e3a.
//
// Solidity: function addMethod(string name) returns(uint8 bit)
func (_NetworkHandler *NetworkHandlerSession) AddMethod(name string) (*types.Transaction, error) {
	return _NetworkHandler.Contract.AddMethod(&_NetworkHandler.TransactOpts, name)
}

// AddMethod is a paid mutator transaction binding the contract method 0x51542e3a.
//
// Solidity: function addMethod(string name) returns(uint8 bit)
func (_NetworkHandler *NetworkHandlerTransactorSession) AddMethod(name string) (*types.Transaction, error) {
	return _NetworkHandler.Contract.AddMethod(&_NetworkHandler.TransactOpts, name)
}

// AddMethods is a paid mutator transaction binding the contract method 0xbd1d9d7c.
//
// Solidity: function addMethods(string[] names) returns(uint256 updatedCapabilities)
func (_NetworkHandler *NetworkHandlerTransactor) AddMethods(opts *bind.TransactOpts, names []string) (*types.Transaction, error) {
	return _NetworkHandler.contract.Transact(opts, "addMethods", names)
}

// AddMethods is a paid mutator transaction binding the contract method 0xbd1d9d7c.
//
// Solidity: function addMethods(string[] names) returns(uint256 updatedCapabilities)
func (_NetworkHandler *NetworkHandlerSession) AddMethods(names []string) (*types.Transaction, error) {
	return _NetworkHandler.Contract.AddMethods(&_NetworkHandler.TransactOpts, names)
}

// AddMethods is a paid mutator transaction binding the contract method 0xbd1d9d7c.
//
// Solidity: function addMethods(string[] names) returns(uint256 updatedCapabilities)
func (_NetworkHandler *NetworkHandlerTransactorSession) AddMethods(names []string) (*types.Transaction, error) {
	return _NetworkHandler.Contract.AddMethods(&_NetworkHandler.TransactOpts, names)
}

// RemoveMethod is a paid mutator transaction binding the contract method 0x3e2e5c6e.
//
// Solidity: function removeMethod(uint8 bit) returns()
func (_NetworkHandler *NetworkHandlerTransactor) RemoveMethod(opts *bind.TransactOpts, bit uint8) (*types.Transaction, error) {
	return _NetworkHandler.contract.Transact(opts, "removeMethod", bit)
}

// RemoveMethod is a paid mutator transaction binding the contract method 0x3e2e5c6e.
//
// Solidity: function removeMethod(uint8 bit) returns()
func (_NetworkHandler *NetworkHandlerSession) RemoveMethod(bit uint8) (*types.Transaction, error) {
	return _NetworkHandler.Contract.RemoveMethod(&_NetworkHandler.TransactOpts, bit)
}

// RemoveMethod is a paid mutator transaction binding the contract method 0x3e2e5c6e.
//
// Solidity: function removeMethod(uint8 bit) returns()
func (_NetworkHandler *NetworkHandlerTransactorSession) RemoveMethod(bit uint8) (*types.Transaction, error) {
	return _NetworkHandler.Contract.RemoveMethod(&_NetworkHandler.TransactOpts, bit)
}

// RemoveMethod0 is a paid mutator transaction binding the contract method 0xf5f2c1e2.
//
// Solidity: function removeMethod(string name) returns(uint8 bit)
func (_NetworkHandler *NetworkHandlerTransactor) RemoveMethod0(opts *bind.TransactOpts, name string) (*types.Transaction, error) {
	return _NetworkHandler.contract.Transact(opts, "removeMethod0", name)
}

// RemoveMethod0 is a paid mutator transaction binding the contract method 0xf5f2c1e2.
//
// Solidity: function removeMethod(string name) returns(uint8 bit)
func (_NetworkHandler *NetworkHandlerSession) RemoveMethod0(name string) (*types.Transaction, error) {
	return _NetworkHandler.Contract.RemoveMethod0(&_NetworkHandler.TransactOpts, name)
}

// RemoveMethod0 is a paid mutator transaction binding the contract method 0xf5f2c1e2.
//
// Solidity: function removeMethod(string name) returns(uint8 bit)
func (_NetworkHandler *NetworkHandlerTransactorSession) RemoveMethod0(name string) (*types.Transaction, error) {
	return _NetworkHandler.Contract.RemoveMethod0(&_NetworkHandler.TransactOpts, name)
}

// SetNetworkOperationsConfig is a paid mutator transaction binding the contract method 0x29135bfd.
//
// Solidity: function setNetworkOperationsConfig((uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint16,uint32,bool,string) config) returns()
func (_NetworkHandler *NetworkHandlerTransactor) SetNetworkOperationsConfig(opts *bind.TransactOpts, config NetworkOperationsConfig) (*types.Transaction, error) {
	return _NetworkHandler.contract.Transact(opts, "setNetworkOperationsConfig", config)
}

// SetNetworkOperationsConfig is a paid mutator transaction binding the contract method 0x29135bfd.
//
// Solidity: function setNetworkOperationsConfig((uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint16,uint32,bool,string) config) returns()
func (_NetworkHandler *NetworkHandlerSession) SetNetworkOperationsConfig(config NetworkOperationsConfig) (*types.Transaction, error) {
	return _NetworkHandler.Contract.SetNetworkOperationsConfig(&_NetworkHandler.TransactOpts, config)
}

// SetNetworkOperationsConfig is a paid mutator transaction binding the contract method 0x29135bfd.
//
// Solidity: function setNetworkOperationsConfig((uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint16,uint32,bool,string) config) returns()
func (_NetworkHandler *NetworkHandlerTransactorSession) SetNetworkOperationsConfig(config NetworkOperationsConfig) (*types.Transaction, error) {
	return _NetworkHandler.Contract.SetNetworkOperationsConfig(&_NetworkHandler.TransactOpts, config)
}

// SetNetworkStatus is a paid mutator transaction binding the contract method 0x9a20fe4b.
//
// Solidity: function setNetworkStatus(uint8 status) returns()
func (_NetworkHandler *NetworkHandlerTransactor) SetNetworkStatus(opts *bind.TransactOpts, status uint8) (*types.Transaction, error) {
	return _NetworkHandler.contract.Transact(opts, "setNetworkStatus", status)
}

// SetNetworkStatus is a paid mutator transaction binding the contract method 0x9a20fe4b.
//
// Solidity: function setNetworkStatus(uint8 status) returns()
func (_NetworkHandler *NetworkHandlerSession) SetNetworkStatus(status uint8) (*types.Transaction, error) {
	return _NetworkHandler.Contract.SetNetworkStatus(&_NetworkHandler.TransactOpts, status)
}

// SetNetworkStatus is a paid mutator transaction binding the contract method 0x9a20fe4b.
//
// Solidity: function setNetworkStatus(uint8 status) returns()
func (_NetworkHandler *NetworkHandlerTransactorSession) SetNetworkStatus(status uint8) (*types.Transaction, error) {
	return _NetworkHandler.Contract.SetNetworkStatus(&_NetworkHandler.TransactOpts, status)
}

// NetworkHandlerAddMethodToNetworkIterator is returned from FilterAddMethodToNetwork and is used to iterate over the raw logs and unpacked data for AddMethodToNetwork events raised by the NetworkHandler contract.
type NetworkHandlerAddMethodToNetworkIterator struct {
	Event *NetworkHandlerAddMethodToNetwork // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NetworkHandlerAddMethodToNetworkIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NetworkHandlerAddMethodToNetwork)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NetworkHandlerAddMethodToNetwork)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NetworkHandlerAddMethodToNetworkIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NetworkHandlerAddMethodToNetworkIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NetworkHandlerAddMethodToNetwork represents a AddMethodToNetwork event raised by the NetworkHandler contract.
type NetworkHandlerAddMethodToNetwork struct {
	Network common.Address
	Method  string
	Bit     uint8
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterAddMethodToNetwork is a free log retrieval operation binding the contract event 0x84fde3f06caa45a2be0890e0df37e0d2521d0be39087c226554a4f857ab56189.
//
// Solidity: event AddMethodToNetwork(address network, string method, uint8 bit)
func (_NetworkHandler *NetworkHandlerFilterer) FilterAddMethodToNetwork(opts *bind.FilterOpts) (*NetworkHandlerAddMethodToNetworkIterator, error) {

	logs, sub, err := _NetworkHandler.contract.FilterLogs(opts, "AddMethodToNetwork")
	if err != nil {
		return nil, err
	}
	return &NetworkHandlerAddMethodToNetworkIterator{contract: _NetworkHandler.contract, event: "AddMethodToNetwork", logs: logs, sub: sub}, nil
}

// WatchAddMethodToNetwork is a free log subscription operation binding the contract event 0x84fde3f06caa45a2be0890e0df37e0d2521d0be39087c226554a4f857ab56189.
//
// Solidity: event AddMethodToNetwork(address network, string method, uint8 bit)
func (_NetworkHandler *NetworkHandlerFilterer) WatchAddMethodToNetwork(opts *bind.WatchOpts, sink chan<- *NetworkHandlerAddMethodToNetwork) (event.Subscription, error) {

	logs, sub, err := _NetworkHandler.contract.WatchLogs(opts, "AddMethodToNetwork")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NetworkHandlerAddMethodToNetwork)
				if err := _NetworkHandler.contract.UnpackLog(event, "AddMethodToNetwork", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseAddMethodToNetwork is a log parse operation binding the contract event 0x84fde3f06caa45a2be0890e0df37e0d2521d0be39087c226554a4f857ab56189.
//
// Solidity: event AddMethodToNetwork(address network, string method, uint8 bit)
func (_NetworkHandler *NetworkHandlerFilterer) ParseAddMethodToNetwork(log types.Log) (*NetworkHandlerAddMethodToNetwork, error) {
	event := new(NetworkHandlerAddMethodToNetwork)
	if err := _NetworkHandler.contract.UnpackLog(event, "AddMethodToNetwork", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NetworkHandlerNetworkOperationConfigUpdatedIterator is returned from FilterNetworkOperationConfigUpdated and is used to iterate over the raw logs and unpacked data for NetworkOperationConfigUpdated events raised by the NetworkHandler contract.
type NetworkHandlerNetworkOperationConfigUpdatedIterator struct {
	Event *NetworkHandlerNetworkOperationConfigUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NetworkHandlerNetworkOperationConfigUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NetworkHandlerNetworkOperationConfigUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NetworkHandlerNetworkOperationConfigUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NetworkHandlerNetworkOperationConfigUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NetworkHandlerNetworkOperationConfigUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NetworkHandlerNetworkOperationConfigUpdated represents a NetworkOperationConfigUpdated event raised by the NetworkHandler contract.
type NetworkHandlerNetworkOperationConfigUpdated struct {
	Network   common.Address
	NewConfig NetworkOperationsConfig
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterNetworkOperationConfigUpdated is a free log retrieval operation binding the contract event 0x7e4082996b608d409d0f9e0e945d76fb0d995142cd13ab46f7a79f324fba768b.
//
// Solidity: event NetworkOperationConfigUpdated(address indexed network, (uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint16,uint32,bool,string) newConfig)
func (_NetworkHandler *NetworkHandlerFilterer) FilterNetworkOperationConfigUpdated(opts *bind.FilterOpts, network []common.Address) (*NetworkHandlerNetworkOperationConfigUpdatedIterator, error) {

	var networkRule []interface{}
	for _, networkItem := range network {
		networkRule = append(networkRule, networkItem)
	}

	logs, sub, err := _NetworkHandler.contract.FilterLogs(opts, "NetworkOperationConfigUpdated", networkRule)
	if err != nil {
		return nil, err
	}
	return &NetworkHandlerNetworkOperationConfigUpdatedIterator{contract: _NetworkHandler.contract, event: "NetworkOperationConfigUpdated", logs: logs, sub: sub}, nil
}

// WatchNetworkOperationConfigUpdated is a free log subscription operation binding the contract event 0x7e4082996b608d409d0f9e0e945d76fb0d995142cd13ab46f7a79f324fba768b.
//
// Solidity: event NetworkOperationConfigUpdated(address indexed network, (uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint16,uint32,bool,string) newConfig)
func (_NetworkHandler *NetworkHandlerFilterer) WatchNetworkOperationConfigUpdated(opts *bind.WatchOpts, sink chan<- *NetworkHandlerNetworkOperationConfigUpdated, network []common.Address) (event.Subscription, error) {

	var networkRule []interface{}
	for _, networkItem := range network {
		networkRule = append(networkRule, networkItem)
	}

	logs, sub, err := _NetworkHandler.contract.WatchLogs(opts, "NetworkOperationConfigUpdated", networkRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NetworkHandlerNetworkOperationConfigUpdated)
				if err := _NetworkHandler.contract.UnpackLog(event, "NetworkOperationConfigUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseNetworkOperationConfigUpdated is a log parse operation binding the contract event 0x7e4082996b608d409d0f9e0e945d76fb0d995142cd13ab46f7a79f324fba768b.
//
// Solidity: event NetworkOperationConfigUpdated(address indexed network, (uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint8,uint16,uint32,bool,string) newConfig)
func (_NetworkHandler *NetworkHandlerFilterer) ParseNetworkOperationConfigUpdated(log types.Log) (*NetworkHandlerNetworkOperationConfigUpdated, error) {
	event := new(NetworkHandlerNetworkOperationConfigUpdated)
	if err := _NetworkHandler.contract.UnpackLog(event, "NetworkOperationConfigUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NetworkHandlerNetworkStatusUpdatedIterator is returned from FilterNetworkStatusUpdated and is used to iterate over the raw logs and unpacked data for NetworkStatusUpdated events raised by the NetworkHandler contract.
type NetworkHandlerNetworkStatusUpdatedIterator struct {
	Event *NetworkHandlerNetworkStatusUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NetworkHandlerNetworkStatusUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NetworkHandlerNetworkStatusUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NetworkHandlerNetworkStatusUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NetworkHandlerNetworkStatusUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NetworkHandlerNetworkStatusUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NetworkHandlerNetworkStatusUpdated represents a NetworkStatusUpdated event raised by the NetworkHandler contract.
type NetworkHandlerNetworkStatusUpdated struct {
	Network   common.Address
	NewStatus uint8
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterNetworkStatusUpdated is a free log retrieval operation binding the contract event 0x2b4ee568e9f2a1024bda603fa924411e6e4b9e866fc7d3f8451dd5d002c0c8d9.
//
// Solidity: event NetworkStatusUpdated(address indexed network, uint8 newStatus)
func (_NetworkHandler *NetworkHandlerFilterer) FilterNetworkStatusUpdated(opts *bind.FilterOpts, network []common.Address) (*NetworkHandlerNetworkStatusUpdatedIterator, error) {

	var networkRule []interface{}
	for _, networkItem := range network {
		networkRule = append(networkRule, networkItem)
	}

	logs, sub, err := _NetworkHandler.contract.FilterLogs(opts, "NetworkStatusUpdated", networkRule)
	if err != nil {
		return nil, err
	}
	return &NetworkHandlerNetworkStatusUpdatedIterator{contract: _NetworkHandler.contract, event: "NetworkStatusUpdated", logs: logs, sub: sub}, nil
}

// WatchNetworkStatusUpdated is a free log subscription operation binding the contract event 0x2b4ee568e9f2a1024bda603fa924411e6e4b9e866fc7d3f8451dd5d002c0c8d9.
//
// Solidity: event NetworkStatusUpdated(address indexed network, uint8 newStatus)
func (_NetworkHandler *NetworkHandlerFilterer) WatchNetworkStatusUpdated(opts *bind.WatchOpts, sink chan<- *NetworkHandlerNetworkStatusUpdated, network []common.Address) (event.Subscription, error) {

	var networkRule []interface{}
	for _, networkItem := range network {
		networkRule = append(networkRule, networkItem)
	}

	logs, sub, err := _NetworkHandler.contract.WatchLogs(opts, "NetworkStatusUpdated", networkRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NetworkHandlerNetworkStatusUpdated)
				if err := _NetworkHandler.contract.UnpackLog(event, "NetworkStatusUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseNetworkStatusUpdated is a log parse operation binding the contract event 0x2b4ee568e9f2a1024bda603fa924411e6e4b9e866fc7d3f8451dd5d002c0c8d9.
//
// Solidity: event NetworkStatusUpdated(address indexed network, uint8 newStatus)
func (_NetworkHandler *NetworkHandlerFilterer) ParseNetworkStatusUpdated(log types.Log) (*NetworkHandlerNetworkStatusUpdated, error) {
	event := new(NetworkHandlerNetworkStatusUpdated)
	if err := _NetworkHandler.contract.UnpackLog(event, "NetworkStatusUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NetworkHandlerRemoveMethodFromNetworkIterator is returned from FilterRemoveMethodFromNetwork and is used to iterate over the raw logs and unpacked data for RemoveMethodFromNetwork events raised by the NetworkHandler contract.
type NetworkHandlerRemoveMethodFromNetworkIterator struct {
	Event *NetworkHandlerRemoveMethodFromNetwork // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *NetworkHandlerRemoveMethodFromNetworkIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NetworkHandlerRemoveMethodFromNetwork)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(NetworkHandlerRemoveMethodFromNetwork)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *NetworkHandlerRemoveMethodFromNetworkIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NetworkHandlerRemoveMethodFromNetworkIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NetworkHandlerRemoveMethodFromNetwork represents a RemoveMethodFromNetwork event raised by the NetworkHandler contract.
type NetworkHandlerRemoveMethodFromNetwork struct {
	Network common.Address
	Method  string
	Bit     uint8
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterRemoveMethodFromNetwork is a free log retrieval operation binding the contract event 0x2baf4a2a9d62128414c571ea30e4c62c5e6562e0af147add7ad02193d58b9dc8.
//
// Solidity: event RemoveMethodFromNetwork(address network, string method, uint8 bit)
func (_NetworkHandler *NetworkHandlerFilterer) FilterRemoveMethodFromNetwork(opts *bind.FilterOpts) (*NetworkHandlerRemoveMethodFromNetworkIterator, error) {

	logs, sub, err := _NetworkHandler.contract.FilterLogs(opts, "RemoveMethodFromNetwork")
	if err != nil {
		return nil, err
	}
	return &NetworkHandlerRemoveMethodFromNetworkIterator{contract: _NetworkHandler.contract, event: "RemoveMethodFromNetwork", logs: logs, sub: sub}, nil
}

// WatchRemoveMethodFromNetwork is a free log subscription operation binding the contract event 0x2baf4a2a9d62128414c571ea30e4c62c5e6562e0af147add7ad02193d58b9dc8.
//
// Solidity: event RemoveMethodFromNetwork(address network, string method, uint8 bit)
func (_NetworkHandler *NetworkHandlerFilterer) WatchRemoveMethodFromNetwork(opts *bind.WatchOpts, sink chan<- *NetworkHandlerRemoveMethodFromNetwork) (event.Subscription, error) {

	logs, sub, err := _NetworkHandler.contract.WatchLogs(opts, "RemoveMethodFromNetwork")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NetworkHandlerRemoveMethodFromNetwork)
				if err := _NetworkHandler.contract.UnpackLog(event, "RemoveMethodFromNetwork", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseRemoveMethodFromNetwork is a log parse operation binding the contract event 0x2baf4a2a9d62128414c571ea30e4c62c5e6562e0af147add7ad02193d58b9dc8.
//
// Solidity: event RemoveMethodFromNetwork(address network, string method, uint8 bit)
func (_NetworkHandler *NetworkHandlerFilterer) ParseRemoveMethodFromNetwork(log types.Log) (*NetworkHandlerRemoveMethodFromNetwork, error) {
	event := new(NetworkHandlerRemoveMethodFromNetwork)
	if err := _NetworkHandler.contract.UnpackLog(event, "RemoveMethodFromNetwork", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
