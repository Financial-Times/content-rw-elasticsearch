PROJECT_NAME=content-rw-elasticsearch
STATIK_VERSION=$(shell go list -m all | grep statik | cut -d ' ' -f2)
.PHONY: all test clean

all: clean build-readonly test

install:
	GO111MODULE="off" go get -u github.com/myitcv/gobin
	gobin github.com/rakyll/statik@${STATIK_VERSION}

generate:
	@echo ">>> Embedding static resources in binary..."
	go generate ./cmd/${PROJECT_NAME}

build: generate
	@echo ">>> Building Application..."
	go build -v ./cmd/${PROJECT_NAME}

build-readonly: generate
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
