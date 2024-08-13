// Code generated by MockGen. DO NOT EDIT.
// Source: interface.go

// Package auth is a generated GoMock package.
package auth

import (
	http "net/http"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	zap "go.uber.org/zap"
)

// MockIAuthClient is a mock of IAuthClient interface.
type MockIAuthClient struct {
	ctrl     *gomock.Controller
	recorder *MockIAuthClientMockRecorder
}

// MockIAuthClientMockRecorder is the mock recorder for MockIAuthClient.
type MockIAuthClientMockRecorder struct {
	mock *MockIAuthClient
}

// NewMockIAuthClient creates a new mock instance.
func NewMockIAuthClient(ctrl *gomock.Controller) *MockIAuthClient {
	mock := &MockIAuthClient{ctrl: ctrl}
	mock.recorder = &MockIAuthClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockIAuthClient) EXPECT() *MockIAuthClientMockRecorder {
	return m.recorder
}

// Error mocks base method.
func (m *MockIAuthClient) Error() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Error")
	ret0, _ := ret[0].(error)
	return ret0
}

// Error indicates an expected call of Error.
func (mr *MockIAuthClientMockRecorder) Error() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Error", reflect.TypeOf((*MockIAuthClient)(nil).Error))
}

// GetToken mocks base method.
func (m *MockIAuthClient) GetToken(arg0 map[string]interface{}) (AuthToken, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetToken", arg0)
	ret0, _ := ret[0].(AuthToken)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetToken indicates an expected call of GetToken.
func (mr *MockIAuthClientMockRecorder) GetToken(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetToken", reflect.TypeOf((*MockIAuthClient)(nil).GetToken), arg0)
}

// Sign mocks base method.
func (m *MockIAuthClient) Sign(arg0 *http.Request) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Sign", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Sign indicates an expected call of Sign.
func (mr *MockIAuthClientMockRecorder) Sign(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Sign", reflect.TypeOf((*MockIAuthClient)(nil).Sign), arg0)
}

// Start mocks base method.
func (m *MockIAuthClient) Start(arg0 *zap.Logger) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Start indicates an expected call of Start.
func (mr *MockIAuthClientMockRecorder) Start(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockIAuthClient)(nil).Start), arg0)
}

// Stop mocks base method.
func (m *MockIAuthClient) Stop() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Stop")
}

// Stop indicates an expected call of Stop.
func (mr *MockIAuthClientMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockIAuthClient)(nil).Stop))
}
