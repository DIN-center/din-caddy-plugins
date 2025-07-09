SHELL := /bin/bash


## Generate Mock interface
# To download mockgen, run: `go install go.uber.org/mock/mockgen@v0.5.2`
generate-mocks:
	mockgen -source=./lib/auth/interface.go -package=auth -destination=./lib/auth/interface_mock.go
	mockgen -source=./lib/auth/siwe/client.go -package=siwe -destination=./lib/auth/siwe/interface_mock.go

