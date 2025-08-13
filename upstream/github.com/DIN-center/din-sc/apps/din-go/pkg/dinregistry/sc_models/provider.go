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

// ProviderAuthConfig is an auto generated low-level Go binding around an user-defined struct.

// ProviderHandlerMetaData contains all meta data concerning the ProviderHandler contract.
var ProviderHandlerMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[{\"name\":\"_owner\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"_name\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"_authConfig\",\"type\":\"tuple\",\"internalType\":\"structProviderAuthConfig\",\"components\":[{\"name\":\"auth\",\"type\":\"uint8\",\"internalType\":\"enumProviderAuthType\"},{\"name\":\"url\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"apiKeyPlaceholder\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"useHeader\",\"type\":\"bool\",\"internalType\":\"bool\"}]},{\"name\":\"_providerStatus\",\"type\":\"uint8\",\"internalType\":\"enumProviderStatus\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"addNetworkService\",\"inputs\":[{\"name\":\"network\",\"type\":\"address\",\"internalType\":\"contractINetwork\"},{\"name\":\"initialCaps\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"serviceUrl\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"status\",\"type\":\"uint8\",\"internalType\":\"enumNetworkServiceStatus\"},{\"name\":\"locations\",\"type\":\"uint8[]\",\"internalType\":\"enumLocationType[]\"}],\"outputs\":[{\"name\":\"service\",\"type\":\"address\",\"internalType\":\"contractNetworkService\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"authConfig\",\"inputs\":[],\"outputs\":[{\"name\":\"auth\",\"type\":\"uint8\",\"internalType\":\"enumProviderAuthType\"},{\"name\":\"url\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"apiKeyPlaceholder\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"useHeader\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"dinAddress\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getAllNetworkServices\",\"inputs\":[],\"outputs\":[{\"name\":\"allServices\",\"type\":\"address[]\",\"internalType\":\"contractNetworkService[]\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"name\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"providerOwner\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"providerStatus\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint8\",\"internalType\":\"enumProviderStatus\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"removeNetworkService\",\"inputs\":[{\"name\":\"network\",\"type\":\"address\",\"internalType\":\"contractINetwork\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"serviceCount\",\"inputs\":[],\"outputs\":[{\"name\":\"count\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"serviceMap\",\"inputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractINetwork\"}],\"outputs\":[{\"name\":\"pos\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"managed\",\"type\":\"address\",\"internalType\":\"contractNetworkService\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"services\",\"inputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractNetworkService\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"setProviderStatus\",\"inputs\":[{\"name\":\"newStatus\",\"type\":\"uint8\",\"internalType\":\"enumProviderStatus\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"event\",\"name\":\"AddNetworkServiceToProvider\",\"inputs\":[{\"name\":\"network\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"contractINetwork\"},{\"name\":\"service\",\"type\":\"address\",\"indexed\":false,\"internalType\":\"contractNetworkService\"},{\"name\":\"initialCaps\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"status\",\"type\":\"uint8\",\"indexed\":false,\"internalType\":\"enumNetworkServiceStatus\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"ProviderStatusUpdated\",\"inputs\":[{\"name\":\"provider\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"contractProvider\"},{\"name\":\"newStatus\",\"type\":\"uint8\",\"indexed\":false,\"internalType\":\"enumProviderStatus\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RemoveNetworkServiceFromProvider\",\"inputs\":[{\"name\":\"network\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"contractINetwork\"},{\"name\":\"service\",\"type\":\"address\",\"indexed\":false,\"internalType\":\"contractNetworkService\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"AuthNotProviderOwner\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"NoProviderNetworkServices\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"ProviderServiceAlreadyExists\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"UnknownProviderService\",\"inputs\":[]}]",
}

// ProviderHandlerABI is the input ABI used to generate the binding from.
// Deprecated: Use ProviderHandlerMetaData.ABI instead.
var ProviderHandlerABI = ProviderHandlerMetaData.ABI

// ProviderHandler is an auto generated Go binding around an Ethereum contract.
type ProviderHandler struct {
	ProviderHandlerCaller     // Read-only binding to the contract
	ProviderHandlerTransactor // Write-only binding to the contract
	ProviderHandlerFilterer   // Log filterer for contract events
}

// ProviderHandlerCaller is an auto generated read-only Go binding around an Ethereum contract.
type ProviderHandlerCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ProviderHandlerTransactor is an auto generated write-only Go binding around an Ethereum contract.
type ProviderHandlerTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ProviderHandlerFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type ProviderHandlerFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ProviderHandlerSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type ProviderHandlerSession struct {
	Contract     *ProviderHandler  // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// ProviderHandlerCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type ProviderHandlerCallerSession struct {
	Contract *ProviderHandlerCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts          // Call options to use throughout this session
}

// ProviderHandlerTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type ProviderHandlerTransactorSession struct {
	Contract     *ProviderHandlerTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts          // Transaction auth options to use throughout this session
}

// ProviderHandlerRaw is an auto generated low-level Go binding around an Ethereum contract.
type ProviderHandlerRaw struct {
	Contract *ProviderHandler // Generic contract binding to access the raw methods on
}

// ProviderHandlerCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type ProviderHandlerCallerRaw struct {
	Contract *ProviderHandlerCaller // Generic read-only contract binding to access the raw methods on
}

// ProviderHandlerTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type ProviderHandlerTransactorRaw struct {
	Contract *ProviderHandlerTransactor // Generic write-only contract binding to access the raw methods on
}

// NewProviderHandler creates a new instance of ProviderHandler, bound to a specific deployed contract.
func NewProviderHandler(address common.Address, backend bind.ContractBackend) (*ProviderHandler, error) {
	contract, err := bindProviderHandler(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &ProviderHandler{ProviderHandlerCaller: ProviderHandlerCaller{contract: contract}, ProviderHandlerTransactor: ProviderHandlerTransactor{contract: contract}, ProviderHandlerFilterer: ProviderHandlerFilterer{contract: contract}}, nil
}

// NewProviderHandlerCaller creates a new read-only instance of ProviderHandler, bound to a specific deployed contract.
func NewProviderHandlerCaller(address common.Address, caller bind.ContractCaller) (*ProviderHandlerCaller, error) {
	contract, err := bindProviderHandler(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &ProviderHandlerCaller{contract: contract}, nil
}

// NewProviderHandlerTransactor creates a new write-only instance of ProviderHandler, bound to a specific deployed contract.
func NewProviderHandlerTransactor(address common.Address, transactor bind.ContractTransactor) (*ProviderHandlerTransactor, error) {
	contract, err := bindProviderHandler(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &ProviderHandlerTransactor{contract: contract}, nil
}

// NewProviderHandlerFilterer creates a new log filterer instance of ProviderHandler, bound to a specific deployed contract.
func NewProviderHandlerFilterer(address common.Address, filterer bind.ContractFilterer) (*ProviderHandlerFilterer, error) {
	contract, err := bindProviderHandler(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &ProviderHandlerFilterer{contract: contract}, nil
}

// bindProviderHandler binds a generic wrapper to an already deployed contract.
func bindProviderHandler(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := ProviderHandlerMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ProviderHandler *ProviderHandlerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ProviderHandler.Contract.ProviderHandlerCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ProviderHandler *ProviderHandlerRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ProviderHandler.Contract.ProviderHandlerTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ProviderHandler *ProviderHandlerRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ProviderHandler.Contract.ProviderHandlerTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_ProviderHandler *ProviderHandlerCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _ProviderHandler.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_ProviderHandler *ProviderHandlerTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _ProviderHandler.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_ProviderHandler *ProviderHandlerTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _ProviderHandler.Contract.contract.Transact(opts, method, params...)
}

// AuthConfig is a free data retrieval call binding the contract method 0xe68d84f2.
//
// Solidity: function authConfig() view returns(uint8 auth, string url, string apiKeyPlaceholder, bool useHeader)
func (_ProviderHandler *ProviderHandlerCaller) AuthConfig(opts *bind.CallOpts) (struct {
	Auth              uint8
	Url               string
	ApiKeyPlaceholder string
	UseHeader         bool
}, error) {
	var out []interface{}
	err := _ProviderHandler.contract.Call(opts, &out, "authConfig")

	outstruct := new(struct {
		Auth              uint8
		Url               string
		ApiKeyPlaceholder string
		UseHeader         bool
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Auth = *abi.ConvertType(out[0], new(uint8)).(*uint8)
	outstruct.Url = *abi.ConvertType(out[1], new(string)).(*string)
	outstruct.ApiKeyPlaceholder = *abi.ConvertType(out[2], new(string)).(*string)
	outstruct.UseHeader = *abi.ConvertType(out[3], new(bool)).(*bool)

	return *outstruct, err

}

// AuthConfig is a free data retrieval call binding the contract method 0xe68d84f2.
//
// Solidity: function authConfig() view returns(uint8 auth, string url, string apiKeyPlaceholder, bool useHeader)
func (_ProviderHandler *ProviderHandlerSession) AuthConfig() (struct {
	Auth              uint8
	Url               string
	ApiKeyPlaceholder string
	UseHeader         bool
}, error) {
	return _ProviderHandler.Contract.AuthConfig(&_ProviderHandler.CallOpts)
}

// AuthConfig is a free data retrieval call binding the contract method 0xe68d84f2.
//
// Solidity: function authConfig() view returns(uint8 auth, string url, string apiKeyPlaceholder, bool useHeader)
func (_ProviderHandler *ProviderHandlerCallerSession) AuthConfig() (struct {
	Auth              uint8
	Url               string
	ApiKeyPlaceholder string
	UseHeader         bool
}, error) {
	return _ProviderHandler.Contract.AuthConfig(&_ProviderHandler.CallOpts)
}

// DinAddress is a free data retrieval call binding the contract method 0xb18ed7aa.
//
// Solidity: function dinAddress() view returns(address)
func (_ProviderHandler *ProviderHandlerCaller) DinAddress(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _ProviderHandler.contract.Call(opts, &out, "dinAddress")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// DinAddress is a free data retrieval call binding the contract method 0xb18ed7aa.
//
// Solidity: function dinAddress() view returns(address)
func (_ProviderHandler *ProviderHandlerSession) DinAddress() (common.Address, error) {
	return _ProviderHandler.Contract.DinAddress(&_ProviderHandler.CallOpts)
}

// DinAddress is a free data retrieval call binding the contract method 0xb18ed7aa.
//
// Solidity: function dinAddress() view returns(address)
func (_ProviderHandler *ProviderHandlerCallerSession) DinAddress() (common.Address, error) {
	return _ProviderHandler.Contract.DinAddress(&_ProviderHandler.CallOpts)
}

// GetAllNetworkServices is a free data retrieval call binding the contract method 0x8f7db24e.
//
// Solidity: function getAllNetworkServices() view returns(address[] allServices)
func (_ProviderHandler *ProviderHandlerCaller) GetAllNetworkServices(opts *bind.CallOpts) ([]common.Address, error) {
	var out []interface{}
	err := _ProviderHandler.contract.Call(opts, &out, "getAllNetworkServices")

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// GetAllNetworkServices is a free data retrieval call binding the contract method 0x8f7db24e.
//
// Solidity: function getAllNetworkServices() view returns(address[] allServices)
func (_ProviderHandler *ProviderHandlerSession) GetAllNetworkServices() ([]common.Address, error) {
	return _ProviderHandler.Contract.GetAllNetworkServices(&_ProviderHandler.CallOpts)
}

// GetAllNetworkServices is a free data retrieval call binding the contract method 0x8f7db24e.
//
// Solidity: function getAllNetworkServices() view returns(address[] allServices)
func (_ProviderHandler *ProviderHandlerCallerSession) GetAllNetworkServices() ([]common.Address, error) {
	return _ProviderHandler.Contract.GetAllNetworkServices(&_ProviderHandler.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_ProviderHandler *ProviderHandlerCaller) Name(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _ProviderHandler.contract.Call(opts, &out, "name")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_ProviderHandler *ProviderHandlerSession) Name() (string, error) {
	return _ProviderHandler.Contract.Name(&_ProviderHandler.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_ProviderHandler *ProviderHandlerCallerSession) Name() (string, error) {
	return _ProviderHandler.Contract.Name(&_ProviderHandler.CallOpts)
}

// ProviderOwner is a free data retrieval call binding the contract method 0xece8027a.
//
// Solidity: function providerOwner() view returns(address)
func (_ProviderHandler *ProviderHandlerCaller) ProviderOwner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _ProviderHandler.contract.Call(opts, &out, "providerOwner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// ProviderOwner is a free data retrieval call binding the contract method 0xece8027a.
//
// Solidity: function providerOwner() view returns(address)
func (_ProviderHandler *ProviderHandlerSession) ProviderOwner() (common.Address, error) {
	return _ProviderHandler.Contract.ProviderOwner(&_ProviderHandler.CallOpts)
}

// ProviderOwner is a free data retrieval call binding the contract method 0xece8027a.
//
// Solidity: function providerOwner() view returns(address)
func (_ProviderHandler *ProviderHandlerCallerSession) ProviderOwner() (common.Address, error) {
	return _ProviderHandler.Contract.ProviderOwner(&_ProviderHandler.CallOpts)
}

// ProviderStatus is a free data retrieval call binding the contract method 0x6926236a.
//
// Solidity: function providerStatus() view returns(uint8)
func (_ProviderHandler *ProviderHandlerCaller) ProviderStatus(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _ProviderHandler.contract.Call(opts, &out, "providerStatus")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// ProviderStatus is a free data retrieval call binding the contract method 0x6926236a.
//
// Solidity: function providerStatus() view returns(uint8)
func (_ProviderHandler *ProviderHandlerSession) ProviderStatus() (uint8, error) {
	return _ProviderHandler.Contract.ProviderStatus(&_ProviderHandler.CallOpts)
}

// ProviderStatus is a free data retrieval call binding the contract method 0x6926236a.
//
// Solidity: function providerStatus() view returns(uint8)
func (_ProviderHandler *ProviderHandlerCallerSession) ProviderStatus() (uint8, error) {
	return _ProviderHandler.Contract.ProviderStatus(&_ProviderHandler.CallOpts)
}

// ServiceCount is a free data retrieval call binding the contract method 0x06237526.
//
// Solidity: function serviceCount() view returns(uint256 count)
func (_ProviderHandler *ProviderHandlerCaller) ServiceCount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _ProviderHandler.contract.Call(opts, &out, "serviceCount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ServiceCount is a free data retrieval call binding the contract method 0x06237526.
//
// Solidity: function serviceCount() view returns(uint256 count)
func (_ProviderHandler *ProviderHandlerSession) ServiceCount() (*big.Int, error) {
	return _ProviderHandler.Contract.ServiceCount(&_ProviderHandler.CallOpts)
}

// ServiceCount is a free data retrieval call binding the contract method 0x06237526.
//
// Solidity: function serviceCount() view returns(uint256 count)
func (_ProviderHandler *ProviderHandlerCallerSession) ServiceCount() (*big.Int, error) {
	return _ProviderHandler.Contract.ServiceCount(&_ProviderHandler.CallOpts)
}

// ServiceMap is a free data retrieval call binding the contract method 0x892c4a39.
//
// Solidity: function serviceMap(address ) view returns(uint256 pos, address managed)
func (_ProviderHandler *ProviderHandlerCaller) ServiceMap(opts *bind.CallOpts, arg0 common.Address) (struct {
	Pos     *big.Int
	Managed common.Address
}, error) {
	var out []interface{}
	err := _ProviderHandler.contract.Call(opts, &out, "serviceMap", arg0)

	outstruct := new(struct {
		Pos     *big.Int
		Managed common.Address
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Pos = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.Managed = *abi.ConvertType(out[1], new(common.Address)).(*common.Address)

	return *outstruct, err

}

// ServiceMap is a free data retrieval call binding the contract method 0x892c4a39.
//
// Solidity: function serviceMap(address ) view returns(uint256 pos, address managed)
func (_ProviderHandler *ProviderHandlerSession) ServiceMap(arg0 common.Address) (struct {
	Pos     *big.Int
	Managed common.Address
}, error) {
	return _ProviderHandler.Contract.ServiceMap(&_ProviderHandler.CallOpts, arg0)
}

// ServiceMap is a free data retrieval call binding the contract method 0x892c4a39.
//
// Solidity: function serviceMap(address ) view returns(uint256 pos, address managed)
func (_ProviderHandler *ProviderHandlerCallerSession) ServiceMap(arg0 common.Address) (struct {
	Pos     *big.Int
	Managed common.Address
}, error) {
	return _ProviderHandler.Contract.ServiceMap(&_ProviderHandler.CallOpts, arg0)
}

// Services is a free data retrieval call binding the contract method 0xc22c4f43.
//
// Solidity: function services(uint256 ) view returns(address)
func (_ProviderHandler *ProviderHandlerCaller) Services(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var out []interface{}
	err := _ProviderHandler.contract.Call(opts, &out, "services", arg0)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Services is a free data retrieval call binding the contract method 0xc22c4f43.
//
// Solidity: function services(uint256 ) view returns(address)
func (_ProviderHandler *ProviderHandlerSession) Services(arg0 *big.Int) (common.Address, error) {
	return _ProviderHandler.Contract.Services(&_ProviderHandler.CallOpts, arg0)
}

// Services is a free data retrieval call binding the contract method 0xc22c4f43.
//
// Solidity: function services(uint256 ) view returns(address)
func (_ProviderHandler *ProviderHandlerCallerSession) Services(arg0 *big.Int) (common.Address, error) {
	return _ProviderHandler.Contract.Services(&_ProviderHandler.CallOpts, arg0)
}

// AddNetworkService is a paid mutator transaction binding the contract method 0x150699b7.
//
// Solidity: function addNetworkService(address network, uint256 initialCaps, string serviceUrl, uint8 status, uint8[] locations) returns(address service)
func (_ProviderHandler *ProviderHandlerTransactor) AddNetworkService(opts *bind.TransactOpts, network common.Address, initialCaps *big.Int, serviceUrl string, status uint8, locations []uint8) (*types.Transaction, error) {
	return _ProviderHandler.contract.Transact(opts, "addNetworkService", network, initialCaps, serviceUrl, status, locations)
}

// AddNetworkService is a paid mutator transaction binding the contract method 0x150699b7.
//
// Solidity: function addNetworkService(address network, uint256 initialCaps, string serviceUrl, uint8 status, uint8[] locations) returns(address service)
func (_ProviderHandler *ProviderHandlerSession) AddNetworkService(network common.Address, initialCaps *big.Int, serviceUrl string, status uint8, locations []uint8) (*types.Transaction, error) {
	return _ProviderHandler.Contract.AddNetworkService(&_ProviderHandler.TransactOpts, network, initialCaps, serviceUrl, status, locations)
}

// AddNetworkService is a paid mutator transaction binding the contract method 0x150699b7.
//
// Solidity: function addNetworkService(address network, uint256 initialCaps, string serviceUrl, uint8 status, uint8[] locations) returns(address service)
func (_ProviderHandler *ProviderHandlerTransactorSession) AddNetworkService(network common.Address, initialCaps *big.Int, serviceUrl string, status uint8, locations []uint8) (*types.Transaction, error) {
	return _ProviderHandler.Contract.AddNetworkService(&_ProviderHandler.TransactOpts, network, initialCaps, serviceUrl, status, locations)
}

// RemoveNetworkService is a paid mutator transaction binding the contract method 0x6fb5ec40.
//
// Solidity: function removeNetworkService(address network) returns()
func (_ProviderHandler *ProviderHandlerTransactor) RemoveNetworkService(opts *bind.TransactOpts, network common.Address) (*types.Transaction, error) {
	return _ProviderHandler.contract.Transact(opts, "removeNetworkService", network)
}

// RemoveNetworkService is a paid mutator transaction binding the contract method 0x6fb5ec40.
//
// Solidity: function removeNetworkService(address network) returns()
func (_ProviderHandler *ProviderHandlerSession) RemoveNetworkService(network common.Address) (*types.Transaction, error) {
	return _ProviderHandler.Contract.RemoveNetworkService(&_ProviderHandler.TransactOpts, network)
}

// RemoveNetworkService is a paid mutator transaction binding the contract method 0x6fb5ec40.
//
// Solidity: function removeNetworkService(address network) returns()
func (_ProviderHandler *ProviderHandlerTransactorSession) RemoveNetworkService(network common.Address) (*types.Transaction, error) {
	return _ProviderHandler.Contract.RemoveNetworkService(&_ProviderHandler.TransactOpts, network)
}

// SetProviderStatus is a paid mutator transaction binding the contract method 0x398eed52.
//
// Solidity: function setProviderStatus(uint8 newStatus) returns()
func (_ProviderHandler *ProviderHandlerTransactor) SetProviderStatus(opts *bind.TransactOpts, newStatus uint8) (*types.Transaction, error) {
	return _ProviderHandler.contract.Transact(opts, "setProviderStatus", newStatus)
}

// SetProviderStatus is a paid mutator transaction binding the contract method 0x398eed52.
//
// Solidity: function setProviderStatus(uint8 newStatus) returns()
func (_ProviderHandler *ProviderHandlerSession) SetProviderStatus(newStatus uint8) (*types.Transaction, error) {
	return _ProviderHandler.Contract.SetProviderStatus(&_ProviderHandler.TransactOpts, newStatus)
}

// SetProviderStatus is a paid mutator transaction binding the contract method 0x398eed52.
//
// Solidity: function setProviderStatus(uint8 newStatus) returns()
func (_ProviderHandler *ProviderHandlerTransactorSession) SetProviderStatus(newStatus uint8) (*types.Transaction, error) {
	return _ProviderHandler.Contract.SetProviderStatus(&_ProviderHandler.TransactOpts, newStatus)
}

// ProviderHandlerAddNetworkServiceToProviderIterator is returned from FilterAddNetworkServiceToProvider and is used to iterate over the raw logs and unpacked data for AddNetworkServiceToProvider events raised by the ProviderHandler contract.
type ProviderHandlerAddNetworkServiceToProviderIterator struct {
	Event *ProviderHandlerAddNetworkServiceToProvider // Event containing the contract specifics and raw log

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
func (it *ProviderHandlerAddNetworkServiceToProviderIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ProviderHandlerAddNetworkServiceToProvider)
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
		it.Event = new(ProviderHandlerAddNetworkServiceToProvider)
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
func (it *ProviderHandlerAddNetworkServiceToProviderIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ProviderHandlerAddNetworkServiceToProviderIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ProviderHandlerAddNetworkServiceToProvider represents a AddNetworkServiceToProvider event raised by the ProviderHandler contract.
type ProviderHandlerAddNetworkServiceToProvider struct {
	Network     common.Address
	Service     common.Address
	InitialCaps *big.Int
	Status      uint8
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterAddNetworkServiceToProvider is a free log retrieval operation binding the contract event 0xfb668c8a621451a41a75d764548a5a689cfbddccaacf1c2e2ee5fa47ac897710.
//
// Solidity: event AddNetworkServiceToProvider(address indexed network, address service, uint256 initialCaps, uint8 status)
func (_ProviderHandler *ProviderHandlerFilterer) FilterAddNetworkServiceToProvider(opts *bind.FilterOpts, network []common.Address) (*ProviderHandlerAddNetworkServiceToProviderIterator, error) {

	var networkRule []interface{}
	for _, networkItem := range network {
		networkRule = append(networkRule, networkItem)
	}

	logs, sub, err := _ProviderHandler.contract.FilterLogs(opts, "AddNetworkServiceToProvider", networkRule)
	if err != nil {
		return nil, err
	}
	return &ProviderHandlerAddNetworkServiceToProviderIterator{contract: _ProviderHandler.contract, event: "AddNetworkServiceToProvider", logs: logs, sub: sub}, nil
}

// WatchAddNetworkServiceToProvider is a free log subscription operation binding the contract event 0xfb668c8a621451a41a75d764548a5a689cfbddccaacf1c2e2ee5fa47ac897710.
//
// Solidity: event AddNetworkServiceToProvider(address indexed network, address service, uint256 initialCaps, uint8 status)
func (_ProviderHandler *ProviderHandlerFilterer) WatchAddNetworkServiceToProvider(opts *bind.WatchOpts, sink chan<- *ProviderHandlerAddNetworkServiceToProvider, network []common.Address) (event.Subscription, error) {

	var networkRule []interface{}
	for _, networkItem := range network {
		networkRule = append(networkRule, networkItem)
	}

	logs, sub, err := _ProviderHandler.contract.WatchLogs(opts, "AddNetworkServiceToProvider", networkRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ProviderHandlerAddNetworkServiceToProvider)
				if err := _ProviderHandler.contract.UnpackLog(event, "AddNetworkServiceToProvider", log); err != nil {
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

// ParseAddNetworkServiceToProvider is a log parse operation binding the contract event 0xfb668c8a621451a41a75d764548a5a689cfbddccaacf1c2e2ee5fa47ac897710.
//
// Solidity: event AddNetworkServiceToProvider(address indexed network, address service, uint256 initialCaps, uint8 status)
func (_ProviderHandler *ProviderHandlerFilterer) ParseAddNetworkServiceToProvider(log types.Log) (*ProviderHandlerAddNetworkServiceToProvider, error) {
	event := new(ProviderHandlerAddNetworkServiceToProvider)
	if err := _ProviderHandler.contract.UnpackLog(event, "AddNetworkServiceToProvider", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ProviderHandlerProviderStatusUpdatedIterator is returned from FilterProviderStatusUpdated and is used to iterate over the raw logs and unpacked data for ProviderStatusUpdated events raised by the ProviderHandler contract.
type ProviderHandlerProviderStatusUpdatedIterator struct {
	Event *ProviderHandlerProviderStatusUpdated // Event containing the contract specifics and raw log

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
func (it *ProviderHandlerProviderStatusUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ProviderHandlerProviderStatusUpdated)
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
		it.Event = new(ProviderHandlerProviderStatusUpdated)
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
func (it *ProviderHandlerProviderStatusUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ProviderHandlerProviderStatusUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ProviderHandlerProviderStatusUpdated represents a ProviderStatusUpdated event raised by the ProviderHandler contract.
type ProviderHandlerProviderStatusUpdated struct {
	Provider  common.Address
	NewStatus uint8
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterProviderStatusUpdated is a free log retrieval operation binding the contract event 0x635339c02445b5014f35ed00c10952e919b10b2548782e33b6e8016143f14c70.
//
// Solidity: event ProviderStatusUpdated(address indexed provider, uint8 newStatus)
func (_ProviderHandler *ProviderHandlerFilterer) FilterProviderStatusUpdated(opts *bind.FilterOpts, provider []common.Address) (*ProviderHandlerProviderStatusUpdatedIterator, error) {

	var providerRule []interface{}
	for _, providerItem := range provider {
		providerRule = append(providerRule, providerItem)
	}

	logs, sub, err := _ProviderHandler.contract.FilterLogs(opts, "ProviderStatusUpdated", providerRule)
	if err != nil {
		return nil, err
	}
	return &ProviderHandlerProviderStatusUpdatedIterator{contract: _ProviderHandler.contract, event: "ProviderStatusUpdated", logs: logs, sub: sub}, nil
}

// WatchProviderStatusUpdated is a free log subscription operation binding the contract event 0x635339c02445b5014f35ed00c10952e919b10b2548782e33b6e8016143f14c70.
//
// Solidity: event ProviderStatusUpdated(address indexed provider, uint8 newStatus)
func (_ProviderHandler *ProviderHandlerFilterer) WatchProviderStatusUpdated(opts *bind.WatchOpts, sink chan<- *ProviderHandlerProviderStatusUpdated, provider []common.Address) (event.Subscription, error) {

	var providerRule []interface{}
	for _, providerItem := range provider {
		providerRule = append(providerRule, providerItem)
	}

	logs, sub, err := _ProviderHandler.contract.WatchLogs(opts, "ProviderStatusUpdated", providerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ProviderHandlerProviderStatusUpdated)
				if err := _ProviderHandler.contract.UnpackLog(event, "ProviderStatusUpdated", log); err != nil {
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

// ParseProviderStatusUpdated is a log parse operation binding the contract event 0x635339c02445b5014f35ed00c10952e919b10b2548782e33b6e8016143f14c70.
//
// Solidity: event ProviderStatusUpdated(address indexed provider, uint8 newStatus)
func (_ProviderHandler *ProviderHandlerFilterer) ParseProviderStatusUpdated(log types.Log) (*ProviderHandlerProviderStatusUpdated, error) {
	event := new(ProviderHandlerProviderStatusUpdated)
	if err := _ProviderHandler.contract.UnpackLog(event, "ProviderStatusUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// ProviderHandlerRemoveNetworkServiceFromProviderIterator is returned from FilterRemoveNetworkServiceFromProvider and is used to iterate over the raw logs and unpacked data for RemoveNetworkServiceFromProvider events raised by the ProviderHandler contract.
type ProviderHandlerRemoveNetworkServiceFromProviderIterator struct {
	Event *ProviderHandlerRemoveNetworkServiceFromProvider // Event containing the contract specifics and raw log

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
func (it *ProviderHandlerRemoveNetworkServiceFromProviderIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ProviderHandlerRemoveNetworkServiceFromProvider)
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
		it.Event = new(ProviderHandlerRemoveNetworkServiceFromProvider)
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
func (it *ProviderHandlerRemoveNetworkServiceFromProviderIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ProviderHandlerRemoveNetworkServiceFromProviderIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ProviderHandlerRemoveNetworkServiceFromProvider represents a RemoveNetworkServiceFromProvider event raised by the ProviderHandler contract.
type ProviderHandlerRemoveNetworkServiceFromProvider struct {
	Network common.Address
	Service common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterRemoveNetworkServiceFromProvider is a free log retrieval operation binding the contract event 0xb7c560d5528eeac59248728cdc3475f88d065e525e7e9704576adb35ebd8c057.
//
// Solidity: event RemoveNetworkServiceFromProvider(address indexed network, address service)
func (_ProviderHandler *ProviderHandlerFilterer) FilterRemoveNetworkServiceFromProvider(opts *bind.FilterOpts, network []common.Address) (*ProviderHandlerRemoveNetworkServiceFromProviderIterator, error) {

	var networkRule []interface{}
	for _, networkItem := range network {
		networkRule = append(networkRule, networkItem)
	}

	logs, sub, err := _ProviderHandler.contract.FilterLogs(opts, "RemoveNetworkServiceFromProvider", networkRule)
	if err != nil {
		return nil, err
	}
	return &ProviderHandlerRemoveNetworkServiceFromProviderIterator{contract: _ProviderHandler.contract, event: "RemoveNetworkServiceFromProvider", logs: logs, sub: sub}, nil
}

// WatchRemoveNetworkServiceFromProvider is a free log subscription operation binding the contract event 0xb7c560d5528eeac59248728cdc3475f88d065e525e7e9704576adb35ebd8c057.
//
// Solidity: event RemoveNetworkServiceFromProvider(address indexed network, address service)
func (_ProviderHandler *ProviderHandlerFilterer) WatchRemoveNetworkServiceFromProvider(opts *bind.WatchOpts, sink chan<- *ProviderHandlerRemoveNetworkServiceFromProvider, network []common.Address) (event.Subscription, error) {

	var networkRule []interface{}
	for _, networkItem := range network {
		networkRule = append(networkRule, networkItem)
	}

	logs, sub, err := _ProviderHandler.contract.WatchLogs(opts, "RemoveNetworkServiceFromProvider", networkRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ProviderHandlerRemoveNetworkServiceFromProvider)
				if err := _ProviderHandler.contract.UnpackLog(event, "RemoveNetworkServiceFromProvider", log); err != nil {
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

// ParseRemoveNetworkServiceFromProvider is a log parse operation binding the contract event 0xb7c560d5528eeac59248728cdc3475f88d065e525e7e9704576adb35ebd8c057.
//
// Solidity: event RemoveNetworkServiceFromProvider(address indexed network, address service)
func (_ProviderHandler *ProviderHandlerFilterer) ParseRemoveNetworkServiceFromProvider(log types.Log) (*ProviderHandlerRemoveNetworkServiceFromProvider, error) {
	event := new(ProviderHandlerRemoveNetworkServiceFromProvider)
	if err := _ProviderHandler.contract.UnpackLog(event, "RemoveNetworkServiceFromProvider", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
