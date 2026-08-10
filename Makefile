BINARY_NAME=engramd
BUILD_DIR=./build

# Version from git tag, falling back to a commit hash if none exists.
VERSION=$(shell git describe --tags --always)

.PHONY: all build build-linux clean test lint proto-gen docker-build help

all: build

build:
	@echo "--> Building engramd..."
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/engramd

build-linux:
	@echo "--> Building engramd for Linux..."
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux ./cmd/engramd

# -race catches proto-generated structs (e.g. PeripheralMetrics, embedding
# sync.Mutex via protoimpl.MessageState) copied by value.
test:
	@echo "--> Running tests..."
	go test -v -race ./...

lint:
	@echo "--> Running golangci-lint..."
	golangci-lint run

# buf.gen.yaml generates into .tmp-proto-gen/ (mirroring the proto package
# path) since the proto package (engram.sovereignty.v1) and the Go package
# (x/sovereignty/types) intentionally differ -- this recipe copies the
# generated files into place and removes the staging directory.
proto-gen:
	@echo "--> Generating protobuf code..."
	rm -rf .tmp-proto-gen
	buf generate
	cp .tmp-proto-gen/engram/sovereignty/v1/*.go x/sovereignty/types/
	rm -rf .tmp-proto-gen

zk-compile:
	@echo "--> Compiling ZK circuits..."
	cd circuit/reanchoring && nargo compile

docker-build:
	@echo "--> Building docker image..."
	docker build -t engram/node:$(VERSION) .

clean:
	rm -rf $(BUILD_DIR)

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build         - Build the node binary (engramd)"
	@echo "  test          - Run all tests"
	@echo "  proto-gen     - Generate protobuf code"
	@echo "  zk-compile    - Compile the Noir circuits"
	@echo "  lint          - Run golangci-lint"
	@echo "  docker-build  - Build the docker image"
