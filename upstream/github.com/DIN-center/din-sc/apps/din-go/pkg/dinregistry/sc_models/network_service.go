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

// NetworkServiceHandlerMetaData contains all meta data concerning the NetworkServiceHandler contract.
var NetworkServiceHandlerMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[{\"name\":\"network\",\"type\":\"address\",\"internalType\":\"contractINetwork\"},{\"name\":\"initialCaps\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"_serviceUrl\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"status\",\"type\":\"uint8\",\"internalType\":\"enumNetworkServiceStatus\"},{\"name\":\"providerOwner\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"locations\",\"type\":\"uint8[]\",\"internalType\":\"enumLocationType[]\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"addLocations\",\"inputs\":[{\"name\":\"_locations\",\"type\":\"uint8[]\",\"internalType\":\"enumLocationType[]\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"capabilities\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getAllMethodNames\",\"inputs\":[],\"outputs\":[{\"name\":\"methods\",\"type\":\"string[]\",\"internalType\":\"string[]\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getLocations\",\"inputs\":[],\"outputs\":[{\"name\":\"locations\",\"type\":\"uint8[]\",\"internalType\":\"enumLocationType[]\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"getStatus\",\"inputs\":[],\"outputs\":[{\"name\":\"status\",\"type\":\"uint8\",\"internalType\":\"enumNetworkServiceStatus\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"inetwork\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractINetwork\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"isMethodSupported\",\"inputs\":[{\"name\":\"bit\",\"type\":\"uint8\",\"internalType\":\"uint8\"}],\"outputs\":[{\"name\":\"supported\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"removeLocations\",\"inputs\":[{\"name\":\"_locations\",\"type\":\"uint8[]\",\"internalType\":\"enumLocationType[]\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"serviceOwner\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"serviceUrl\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"string\",\"internalType\":\"string\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"setCapabilities\",\"inputs\":[{\"name\":\"caps\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"setStatus\",\"inputs\":[{\"name\":\"status\",\"type\":\"uint8\",\"internalType\":\"enumNetworkServiceStatus\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"event\",\"name\":\"CapabilitiesUpdated\",\"inputs\":[{\"name\":\"service\",\"type\":\"address\",\"indexed\":false,\"internalType\":\"contractNetworkService\"},{\"name\":\"capabilities\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"ServiceStatusUpdated\",\"inputs\":[{\"name\":\"service\",\"type\":\"address\",\"indexed\":false,\"internalType\":\"contractNetworkService\"},{\"name\":\"status\",\"type\":\"uint8\",\"indexed\":false,\"internalType\":\"enumNetworkServiceStatus\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"AuthNotServiceOwner\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"CapabilitiesNotSupported\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"NetworkHasNoMethods\",\"inputs\":[]}]",
}

// NetworkServiceHandlerABI is the input ABI used to generate the binding from.
// Deprecated: Use NetworkServiceHandlerMetaData.ABI instead.
var NetworkServiceHandlerABI = NetworkServiceHandlerMetaData.ABI

// NetworkServiceHandler is an auto generated Go binding around an Ethereum contract.
type NetworkServiceHandler struct {
	NetworkServiceHandlerCaller     // Read-only binding to the contract
	NetworkServiceHandlerTransactor // Write-only binding to the contract
	NetworkServiceHandlerFilterer   // Log filterer for contract events
}

// NetworkServiceHandlerCaller is an auto generated read-only Go binding around an Ethereum contract.
type NetworkServiceHandlerCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NetworkServiceHandlerTransactor is an auto generated write-only Go binding around an Ethereum contract.
type NetworkServiceHandlerTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NetworkServiceHandlerFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type NetworkServiceHandlerFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NetworkServiceHandlerSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type NetworkServiceHandlerSession struct {
	Contract     *NetworkServiceHandler // Generic contract binding to set the session for
	CallOpts     bind.CallOpts          // Call options to use throughout this session
	TransactOpts bind.TransactOpts      // Transaction auth options to use throughout this session
}

// NetworkServiceHandlerCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type NetworkServiceHandlerCallerSession struct {
	Contract *NetworkServiceHandlerCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts                // Call options to use throughout this session
}

// NetworkServiceHandlerTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type NetworkServiceHandlerTransactorSession struct {
	Contract     *NetworkServiceHandlerTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts                // Transaction auth options to use throughout this session
}

// NetworkServiceHandlerRaw is an auto generated low-level Go binding around an Ethereum contract.
type NetworkServiceHandlerRaw struct {
	Contract *NetworkServiceHandler // Generic contract binding to access the raw methods on
}

// NetworkServiceHandlerCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type NetworkServiceHandlerCallerRaw struct {
	Contract *NetworkServiceHandlerCaller // Generic read-only contract binding to access the raw methods on
}

// NetworkServiceHandlerTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type NetworkServiceHandlerTransactorRaw struct {
	Contract *NetworkServiceHandlerTransactor // Generic write-only contract binding to access the raw methods on
}

// NewNetworkServiceHandler creates a new instance of NetworkServiceHandler, bound to a specific deployed contract.
func NewNetworkServiceHandler(address common.Address, backend bind.ContractBackend) (*NetworkServiceHandler, error) {
	contract, err := bindNetworkServiceHandler(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &NetworkServiceHandler{NetworkServiceHandlerCaller: NetworkServiceHandlerCaller{contract: contract}, NetworkServiceHandlerTransactor: NetworkServiceHandlerTransactor{contract: contract}, NetworkServiceHandlerFilterer: NetworkServiceHandlerFilterer{contract: contract}}, nil
}

// NewNetworkServiceHandlerCaller creates a new read-only instance of NetworkServiceHandler, bound to a specific deployed contract.
func NewNetworkServiceHandlerCaller(address common.Address, caller bind.ContractCaller) (*NetworkServiceHandlerCaller, error) {
	contract, err := bindNetworkServiceHandler(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &NetworkServiceHandlerCaller{contract: contract}, nil
}

// NewNetworkServiceHandlerTransactor creates a new write-only instance of NetworkServiceHandler, bound to a specific deployed contract.
func NewNetworkServiceHandlerTransactor(address common.Address, transactor bind.ContractTransactor) (*NetworkServiceHandlerTransactor, error) {
	contract, err := bindNetworkServiceHandler(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &NetworkServiceHandlerTransactor{contract: contract}, nil
}

// NewNetworkServiceHandlerFilterer creates a new log filterer instance of NetworkServiceHandler, bound to a specific deployed contract.
func NewNetworkServiceHandlerFilterer(address common.Address, filterer bind.ContractFilterer) (*NetworkServiceHandlerFilterer, error) {
	contract, err := bindNetworkServiceHandler(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &NetworkServiceHandlerFilterer{contract: contract}, nil
}

// bindNetworkServiceHandler binds a generic wrapper to an already deployed contract.
func bindNetworkServiceHandler(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := NetworkServiceHandlerMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_NetworkServiceHandler *NetworkServiceHandlerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _NetworkServiceHandler.Contract.NetworkServiceHandlerCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_NetworkServiceHandler *NetworkServiceHandlerRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NetworkServiceHandler.Contract.NetworkServiceHandlerTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_NetworkServiceHandler *NetworkServiceHandlerRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _NetworkServiceHandler.Contract.NetworkServiceHandlerTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_NetworkServiceHandler *NetworkServiceHandlerCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _NetworkServiceHandler.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_NetworkServiceHandler *NetworkServiceHandlerTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NetworkServiceHandler.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_NetworkServiceHandler *NetworkServiceHandlerTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _NetworkServiceHandler.Contract.contract.Transact(opts, method, params...)
}

// Capabilities is a free data retrieval call binding the contract method 0x34a18fc3.
//
// Solidity: function capabilities() view returns(uint256)
func (_NetworkServiceHandler *NetworkServiceHandlerCaller) Capabilities(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _NetworkServiceHandler.contract.Call(opts, &out, "capabilities")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Capabilities is a free data retrieval call binding the contract method 0x34a18fc3.
//
// Solidity: function capabilities() view returns(uint256)
func (_NetworkServiceHandler *NetworkServiceHandlerSession) Capabilities() (*big.Int, error) {
	return _NetworkServiceHandler.Contract.Capabilities(&_NetworkServiceHandler.CallOpts)
}

// Capabilities is a free data retrieval call binding the contract method 0x34a18fc3.
//
// Solidity: function capabilities() view returns(uint256)
func (_NetworkServiceHandler *NetworkServiceHandlerCallerSession) Capabilities() (*big.Int, error) {
	return _NetworkServiceHandler.Contract.Capabilities(&_NetworkServiceHandler.CallOpts)
}

// GetAllMethodNames is a free data retrieval call binding the contract method 0xd39bed2d.
//
// Solidity: function getAllMethodNames() view returns(string[] methods)
func (_NetworkServiceHandler *NetworkServiceHandlerCaller) GetAllMethodNames(opts *bind.CallOpts) ([]string, error) {
	var out []interface{}
	err := _NetworkServiceHandler.contract.Call(opts, &out, "getAllMethodNames")

	if err != nil {
		return *new([]string), err
	}

	out0 := *abi.ConvertType(out[0], new([]string)).(*[]string)

	return out0, err

}

// GetAllMethodNames is a free data retrieval call binding the contract method 0xd39bed2d.
//
// Solidity: function getAllMethodNames() view returns(string[] methods)
func (_NetworkServiceHandler *NetworkServiceHandlerSession) GetAllMethodNames() ([]string, error) {
	return _NetworkServiceHandler.Contract.GetAllMethodNames(&_NetworkServiceHandler.CallOpts)
}

// GetAllMethodNames is a free data retrieval call binding the contract method 0xd39bed2d.
//
// Solidity: function getAllMethodNames() view returns(string[] methods)
func (_NetworkServiceHandler *NetworkServiceHandlerCallerSession) GetAllMethodNames() ([]string, error) {
	return _NetworkServiceHandler.Contract.GetAllMethodNames(&_NetworkServiceHandler.CallOpts)
}

// GetLocations is a free data retrieval call binding the contract method 0xab63616f.
//
// Solidity: function getLocations() view returns(uint8[] locations)
func (_NetworkServiceHandler *NetworkServiceHandlerCaller) GetLocations(opts *bind.CallOpts) ([]uint8, error) {
	var out []interface{}
	err := _NetworkServiceHandler.contract.Call(opts, &out, "getLocations")

	if err != nil {
		return *new([]uint8), err
	}

	out0 := *abi.ConvertType(out[0], new([]uint8)).(*[]uint8)

	return out0, err

}

// GetLocations is a free data retrieval call binding the contract method 0xab63616f.
//
// Solidity: function getLocations() view returns(uint8[] locations)
func (_NetworkServiceHandler *NetworkServiceHandlerSession) GetLocations() ([]uint8, error) {
	return _NetworkServiceHandler.Contract.GetLocations(&_NetworkServiceHandler.CallOpts)
}

// GetLocations is a free data retrieval call binding the contract method 0xab63616f.
//
// Solidity: function getLocations() view returns(uint8[] locations)
func (_NetworkServiceHandler *NetworkServiceHandlerCallerSession) GetLocations() ([]uint8, error) {
	return _NetworkServiceHandler.Contract.GetLocations(&_NetworkServiceHandler.CallOpts)
}

// GetStatus is a free data retrieval call binding the contract method 0x4e69d560.
//
// Solidity: function getStatus() view returns(uint8 status)
func (_NetworkServiceHandler *NetworkServiceHandlerCaller) GetStatus(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _NetworkServiceHandler.contract.Call(opts, &out, "getStatus")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// GetStatus is a free data retrieval call binding the contract method 0x4e69d560.
//
// Solidity: function getStatus() view returns(uint8 status)
func (_NetworkServiceHandler *NetworkServiceHandlerSession) GetStatus() (uint8, error) {
	return _NetworkServiceHandler.Contract.GetStatus(&_NetworkServiceHandler.CallOpts)
}

// GetStatus is a free data retrieval call binding the contract method 0x4e69d560.
//
// Solidity: function getStatus() view returns(uint8 status)
func (_NetworkServiceHandler *NetworkServiceHandlerCallerSession) GetStatus() (uint8, error) {
	return _NetworkServiceHandler.Contract.GetStatus(&_NetworkServiceHandler.CallOpts)
}

// Inetwork is a free data retrieval call binding the contract method 0x3d8f6ca1.
//
// Solidity: function inetwork() view returns(address)
func (_NetworkServiceHandler *NetworkServiceHandlerCaller) Inetwork(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _NetworkServiceHandler.contract.Call(opts, &out, "inetwork")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Inetwork is a free data retrieval call binding the contract method 0x3d8f6ca1.
//
// Solidity: function inetwork() view returns(address)
func (_NetworkServiceHandler *NetworkServiceHandlerSession) Inetwork() (common.Address, error) {
	return _NetworkServiceHandler.Contract.Inetwork(&_NetworkServiceHandler.CallOpts)
}

// Inetwork is a free data retrieval call binding the contract method 0x3d8f6ca1.
//
// Solidity: function inetwork() view returns(address)
func (_NetworkServiceHandler *NetworkServiceHandlerCallerSession) Inetwork() (common.Address, error) {
	return _NetworkServiceHandler.Contract.Inetwork(&_NetworkServiceHandler.CallOpts)
}

// IsMethodSupported is a free data retrieval call binding the contract method 0xbd822ebf.
//
// Solidity: function isMethodSupported(uint8 bit) view returns(bool supported)
func (_NetworkServiceHandler *NetworkServiceHandlerCaller) IsMethodSupported(opts *bind.CallOpts, bit uint8) (bool, error) {
	var out []interface{}
	err := _NetworkServiceHandler.contract.Call(opts, &out, "isMethodSupported", bit)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsMethodSupported is a free data retrieval call binding the contract method 0xbd822ebf.
//
// Solidity: function isMethodSupported(uint8 bit) view returns(bool supported)
func (_NetworkServiceHandler *NetworkServiceHandlerSession) IsMethodSupported(bit uint8) (bool, error) {
	return _NetworkServiceHandler.Contract.IsMethodSupported(&_NetworkServiceHandler.CallOpts, bit)
}

// IsMethodSupported is a free data retrieval call binding the contract method 0xbd822ebf.
//
// Solidity: function isMethodSupported(uint8 bit) view returns(bool supported)
func (_NetworkServiceHandler *NetworkServiceHandlerCallerSession) IsMethodSupported(bit uint8) (bool, error) {
	return _NetworkServiceHandler.Contract.IsMethodSupported(&_NetworkServiceHandler.CallOpts, bit)
}

// ServiceOwner is a free data retrieval call binding the contract method 0xa4f4d379.
//
// Solidity: function serviceOwner() view returns(address)
func (_NetworkServiceHandler *NetworkServiceHandlerCaller) ServiceOwner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _NetworkServiceHandler.contract.Call(opts, &out, "serviceOwner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// ServiceOwner is a free data retrieval call binding the contract method 0xa4f4d379.
//
// Solidity: function serviceOwner() view returns(address)
func (_NetworkServiceHandler *NetworkServiceHandlerSession) ServiceOwner() (common.Address, error) {
	return _NetworkServiceHandler.Contract.ServiceOwner(&_NetworkServiceHandler.CallOpts)
}

// ServiceOwner is a free data retrieval call binding the contract method 0xa4f4d379.
//
// Solidity: function serviceOwner() view returns(address)
func (_NetworkServiceHandler *NetworkServiceHandlerCallerSession) ServiceOwner() (common.Address, error) {
	return _NetworkServiceHandler.Contract.ServiceOwner(&_NetworkServiceHandler.CallOpts)
}

// ServiceUrl is a free data retrieval call binding the contract method 0x65e55d3f.
//
// Solidity: function serviceUrl() view returns(string)
func (_NetworkServiceHandler *NetworkServiceHandlerCaller) ServiceUrl(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _NetworkServiceHandler.contract.Call(opts, &out, "serviceUrl")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// ServiceUrl is a free data retrieval call binding the contract method 0x65e55d3f.
//
// Solidity: function serviceUrl() view returns(string)
func (_NetworkServiceHandler *NetworkServiceHandlerSession) ServiceUrl() (string, error) {
	return _NetworkServiceHandler.Contract.ServiceUrl(&_NetworkServiceHandler.CallOpts)
}

// ServiceUrl is a free data retrieval call binding the contract method 0x65e55d3f.
//
// Solidity: function serviceUrl() view returns(string)
func (_NetworkServiceHandler *NetworkServiceHandlerCallerSession) ServiceUrl() (string, error) {
	return _NetworkServiceHandler.Contract.ServiceUrl(&_NetworkServiceHandler.CallOpts)
}

// AddLocations is a paid mutator transaction binding the contract method 0xdb01b353.
//
// Solidity: function addLocations(uint8[] _locations) returns()
func (_NetworkServiceHandler *NetworkServiceHandlerTransactor) AddLocations(opts *bind.TransactOpts, _locations []uint8) (*types.Transaction, error) {
	return _NetworkServiceHandler.contract.Transact(opts, "addLocations", _locations)
}

// AddLocations is a paid mutator transaction binding the contract method 0xdb01b353.
//
// Solidity: function addLocations(uint8[] _locations) returns()
func (_NetworkServiceHandler *NetworkServiceHandlerSession) AddLocations(_locations []uint8) (*types.Transaction, error) {
	return _NetworkServiceHandler.Contract.AddLocations(&_NetworkServiceHandler.TransactOpts, _locations)
}

// AddLocations is a paid mutator transaction binding the contract method 0xdb01b353.
//
// Solidity: function addLocations(uint8[] _locations) returns()
func (_NetworkServiceHandler *NetworkServiceHandlerTransactorSession) AddLocations(_locations []uint8) (*types.Transaction, error) {
	return _NetworkServiceHandler.Contract.AddLocations(&_NetworkServiceHandler.TransactOpts, _locations)
}

// RemoveLocations is a paid mutator transaction binding the contract method 0xe6a0e397.
//
// Solidity: function removeLocations(uint8[] _locations) returns()
func (_NetworkServiceHandler *NetworkServiceHandlerTransactor) RemoveLocations(opts *bind.TransactOpts, _locations []uint8) (*types.Transaction, error) {
	return _NetworkServiceHandler.contract.Transact(opts, "removeLocations", _locations)
}

// RemoveLocations is a paid mutator transaction binding the contract method 0xe6a0e397.
//
// Solidity: function removeLocations(uint8[] _locations) returns()
func (_NetworkServiceHandler *NetworkServiceHandlerSession) RemoveLocations(_locations []uint8) (*types.Transaction, error) {
	return _NetworkServiceHandler.Contract.RemoveLocations(&_NetworkServiceHandler.TransactOpts, _locations)
}

// RemoveLocations is a paid mutator transaction binding the contract method 0xe6a0e397.
//
// Solidity: function removeLocations(uint8[] _locations) returns()
func (_NetworkServiceHandler *NetworkServiceHandlerTransactorSession) RemoveLocations(_locations []uint8) (*types.Transaction, error) {
	return _NetworkServiceHandler.Contract.RemoveLocations(&_NetworkServiceHandler.TransactOpts, _locations)
}

// SetCapabilities is a paid mutator transaction binding the contract method 0x38bbddd9.
//
// Solidity: function setCapabilities(uint256 caps) returns()
func (_NetworkServiceHandler *NetworkServiceHandlerTransactor) SetCapabilities(opts *bind.TransactOpts, caps *big.Int) (*types.Transaction, error) {
	return _NetworkServiceHandler.contract.Transact(opts, "setCapabilities", caps)
}

// SetCapabilities is a paid mutator transaction binding the contract method 0x38bbddd9.
//
// Solidity: function setCapabilities(uint256 caps) returns()
func (_NetworkServiceHandler *NetworkServiceHandlerSession) SetCapabilities(caps *big.Int) (*types.Transaction, error) {
	return _NetworkServiceHandler.Contract.SetCapabilities(&_NetworkServiceHandler.TransactOpts, caps)
}

// SetCapabilities is a paid mutator transaction binding the contract method 0x38bbddd9.
//
// Solidity: function setCapabilities(uint256 caps) returns()
func (_NetworkServiceHandler *NetworkServiceHandlerTransactorSession) SetCapabilities(caps *big.Int) (*types.Transaction, error) {
	return _NetworkServiceHandler.Contract.SetCapabilities(&_NetworkServiceHandler.TransactOpts, caps)
}

// SetStatus is a paid mutator transaction binding the contract method 0x2e49d78b.
//
// Solidity: function setStatus(uint8 status) returns()
func (_NetworkServiceHandler *NetworkServiceHandlerTransactor) SetStatus(opts *bind.TransactOpts, status uint8) (*types.Transaction, error) {
	return _NetworkServiceHandler.contract.Transact(opts, "setStatus", status)
}

// SetStatus is a paid mutator transaction binding the contract method 0x2e49d78b.
//
// Solidity: function setStatus(uint8 status) returns()
func (_NetworkServiceHandler *NetworkServiceHandlerSession) SetStatus(status uint8) (*types.Transaction, error) {
	return _NetworkServiceHandler.Contract.SetStatus(&_NetworkServiceHandler.TransactOpts, status)
}

// SetStatus is a paid mutator transaction binding the contract method 0x2e49d78b.
//
// Solidity: function setStatus(uint8 status) returns()
func (_NetworkServiceHandler *NetworkServiceHandlerTransactorSession) SetStatus(status uint8) (*types.Transaction, error) {
	return _NetworkServiceHandler.Contract.SetStatus(&_NetworkServiceHandler.TransactOpts, status)
}

// NetworkServiceHandlerCapabilitiesUpdatedIterator is returned from FilterCapabilitiesUpdated and is used to iterate over the raw logs and unpacked data for CapabilitiesUpdated events raised by the NetworkServiceHandler contract.
type NetworkServiceHandlerCapabilitiesUpdatedIterator struct {
	Event *NetworkServiceHandlerCapabilitiesUpdated // Event containing the contract specifics and raw log

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
func (it *NetworkServiceHandlerCapabilitiesUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NetworkServiceHandlerCapabilitiesUpdated)
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
		it.Event = new(NetworkServiceHandlerCapabilitiesUpdated)
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
func (it *NetworkServiceHandlerCapabilitiesUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NetworkServiceHandlerCapabilitiesUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NetworkServiceHandlerCapabilitiesUpdated represents a CapabilitiesUpdated event raised by the NetworkServiceHandler contract.
type NetworkServiceHandlerCapabilitiesUpdated struct {
	Service      common.Address
	Capabilities *big.Int
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterCapabilitiesUpdated is a free log retrieval operation binding the contract event 0xf84544f6f6a92d98415df5e0314f4f83c95ceae83173cde77b2ddcd370fbb07d.
//
// Solidity: event CapabilitiesUpdated(address service, uint256 capabilities)
func (_NetworkServiceHandler *NetworkServiceHandlerFilterer) FilterCapabilitiesUpdated(opts *bind.FilterOpts) (*NetworkServiceHandlerCapabilitiesUpdatedIterator, error) {

	logs, sub, err := _NetworkServiceHandler.contract.FilterLogs(opts, "CapabilitiesUpdated")
	if err != nil {
		return nil, err
	}
	return &NetworkServiceHandlerCapabilitiesUpdatedIterator{contract: _NetworkServiceHandler.contract, event: "CapabilitiesUpdated", logs: logs, sub: sub}, nil
}

// WatchCapabilitiesUpdated is a free log subscription operation binding the contract event 0xf84544f6f6a92d98415df5e0314f4f83c95ceae83173cde77b2ddcd370fbb07d.
//
// Solidity: event CapabilitiesUpdated(address service, uint256 capabilities)
func (_NetworkServiceHandler *NetworkServiceHandlerFilterer) WatchCapabilitiesUpdated(opts *bind.WatchOpts, sink chan<- *NetworkServiceHandlerCapabilitiesUpdated) (event.Subscription, error) {

	logs, sub, err := _NetworkServiceHandler.contract.WatchLogs(opts, "CapabilitiesUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NetworkServiceHandlerCapabilitiesUpdated)
				if err := _NetworkServiceHandler.contract.UnpackLog(event, "CapabilitiesUpdated", log); err != nil {
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

// ParseCapabilitiesUpdated is a log parse operation binding the contract event 0xf84544f6f6a92d98415df5e0314f4f83c95ceae83173cde77b2ddcd370fbb07d.
//
// Solidity: event CapabilitiesUpdated(address service, uint256 capabilities)
func (_NetworkServiceHandler *NetworkServiceHandlerFilterer) ParseCapabilitiesUpdated(log types.Log) (*NetworkServiceHandlerCapabilitiesUpdated, error) {
	event := new(NetworkServiceHandlerCapabilitiesUpdated)
	if err := _NetworkServiceHandler.contract.UnpackLog(event, "CapabilitiesUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NetworkServiceHandlerServiceStatusUpdatedIterator is returned from FilterServiceStatusUpdated and is used to iterate over the raw logs and unpacked data for ServiceStatusUpdated events raised by the NetworkServiceHandler contract.
type NetworkServiceHandlerServiceStatusUpdatedIterator struct {
	Event *NetworkServiceHandlerServiceStatusUpdated // Event containing the contract specifics and raw log

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
func (it *NetworkServiceHandlerServiceStatusUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NetworkServiceHandlerServiceStatusUpdated)
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
		it.Event = new(NetworkServiceHandlerServiceStatusUpdated)
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
func (it *NetworkServiceHandlerServiceStatusUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NetworkServiceHandlerServiceStatusUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NetworkServiceHandlerServiceStatusUpdated represents a ServiceStatusUpdated event raised by the NetworkServiceHandler contract.
type NetworkServiceHandlerServiceStatusUpdated struct {
	Service common.Address
	Status  uint8
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterServiceStatusUpdated is a free log retrieval operation binding the contract event 0x681417fc59984b5c6394dd5c87322cd54bc617667ad8a7a27e188a99227a2e29.
//
// Solidity: event ServiceStatusUpdated(address service, uint8 status)
func (_NetworkServiceHandler *NetworkServiceHandlerFilterer) FilterServiceStatusUpdated(opts *bind.FilterOpts) (*NetworkServiceHandlerServiceStatusUpdatedIterator, error) {

	logs, sub, err := _NetworkServiceHandler.contract.FilterLogs(opts, "ServiceStatusUpdated")
	if err != nil {
		return nil, err
	}
	return &NetworkServiceHandlerServiceStatusUpdatedIterator{contract: _NetworkServiceHandler.contract, event: "ServiceStatusUpdated", logs: logs, sub: sub}, nil
}

// WatchServiceStatusUpdated is a free log subscription operation binding the contract event 0x681417fc59984b5c6394dd5c87322cd54bc617667ad8a7a27e188a99227a2e29.
//
// Solidity: event ServiceStatusUpdated(address service, uint8 status)
func (_NetworkServiceHandler *NetworkServiceHandlerFilterer) WatchServiceStatusUpdated(opts *bind.WatchOpts, sink chan<- *NetworkServiceHandlerServiceStatusUpdated) (event.Subscription, error) {

	logs, sub, err := _NetworkServiceHandler.contract.WatchLogs(opts, "ServiceStatusUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NetworkServiceHandlerServiceStatusUpdated)
				if err := _NetworkServiceHandler.contract.UnpackLog(event, "ServiceStatusUpdated", log); err != nil {
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

// ParseServiceStatusUpdated is a log parse operation binding the contract event 0x681417fc59984b5c6394dd5c87322cd54bc617667ad8a7a27e188a99227a2e29.
//
// Solidity: event ServiceStatusUpdated(address service, uint8 status)
func (_NetworkServiceHandler *NetworkServiceHandlerFilterer) ParseServiceStatusUpdated(log types.Log) (*NetworkServiceHandlerServiceStatusUpdated, error) {
	event := new(NetworkServiceHandlerServiceStatusUpdated)
	if err := _NetworkServiceHandler.contract.UnpackLog(event, "ServiceStatusUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
