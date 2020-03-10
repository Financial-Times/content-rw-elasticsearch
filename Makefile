PROJECT_NAME=content-rw-elasticsearch
.PHONY: all test clean

all: clean test build-readonly

build:
	@echo ">>> Embedding static resources in binary..."
	go generate ./cmd/${PROJECT_NAME}
	@echo ">>> Building Application..."
	go build -v ./cmd/${PROJECT_NAME}

build-readonly:
	@echo ">>> Embedding static resources in binary..."
	go generate ./cmd/${PROJECT_NAME}
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
