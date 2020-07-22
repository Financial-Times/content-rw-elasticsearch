PROJECT_NAME=content-rw-elasticsearch
STATIK_VERSION=$(shell go list -m all | grep statik | cut -d ' ' -f2)
.PHONY: all test clean

all: clean build-readonly test


build:
	@echo ">>> Building Application..."
	go build -v ./cmd/${PROJECT_NAME}

build-readonly:
	@echo ">>> Building Application with -mod=readonly..."
	go build -mod=readonly -v ./cmd/${PROJECT_NAME}

test:
	@echo ">>> Running Tests..."
	go test -race -v ./...

cover-test:
	@echo ">>> Running Tests with Coverage..."
	go test -race ./... -coverprofile=coverage.out -covermode=atomic

clean:
	@echo ">>> Removing binaries..."
	@rm -rf ./${PROJECT_NAME}
	@echo ">>> Cleaning modules cache..."
	go clean -modcache
