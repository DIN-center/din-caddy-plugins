SHELL := /bin/bash

# Get the current working directory
# assume it is MONO_ROOT_DIR/apps/din-go
PWD := $(CURDIR)

# Derive MONO_ROOT_DIR by trimming the path up twice using dirname
MONO_ROOT_DIR := $(shell dirname $(shell dirname $(PWD)))
include $(MONO_ROOT_DIR)/common.mk


## Generate Mock interface
generate:
	mockgen -source=./pkg/dinregistry/interface.go -package=dinregistry -destination=./pkg/dinregistry/interface_mock.go

