SHELL := /bin/bash

# Get the current working directory
# assume it is MONO_ROOT_DIR/apps/din-go
PWD := $(CURDIR)

# Derive MONO_ROOT_DIR by trimming the path up twice using dirname
MONO_ROOT_DIR := $(shell dirname $(shell dirname $(PWD)))
include $(MONO_ROOT_DIR)/common.mk


## Generate Mock interface
# To download mockgen, run: `go install go.uber.org/mock/mockgen@v0.5.2`
generate-mocks:
	mockgen -source=./cli/cmd/interface.go -package=dincli -destination=./cli/cmd/interface_mock.go
	mockgen -source=./lib/din/interface.go -package=din -destination=./lib/din/interface_mock.go

