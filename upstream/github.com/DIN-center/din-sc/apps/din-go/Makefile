SHELL := /bin/bash

# Get the current working directory
# assume it is MONO_ROOT_DIR/apps/din-go
PWD := $(CURDIR)

# Derive MONO_ROOT_DIR by trimming the path up twice using dirname
MONO_ROOT_DIR := $(shell dirname $(shell dirname $(PWD)))
include $(MONO_ROOT_DIR)/common.mk


## Generate Mock interface
# To download mockgen, run: `go install github.com/golang/mock/mockgen@latest`
generate:
	mockgen -source=./pkg/dinregistry/interface.go -package=dinregistry -destination=./pkg/dinregistry/interface_mock.go
## Run integration tests against foundry contract data
test-contracts:
	go test -v -count=1 ./lib/din

