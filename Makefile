# Tên binary của dự án
BINARY_NAME=engramd
BUILD_DIR=./build

# Lấy phiên bản version dựa trên git tag hoặc mặc định
VERSION=$(shell git describe --tags --always)

.PHONY: all build build-linux clean test lint proto-gen docker-build help

all: build

# 1. Biên dịch dự án
build:
	@echo "--> Building engramd..."
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/engramd

# 2. Biên dịch cho Docker (Linux)
build-linux:
	@echo "--> Building engramd for Linux..."
	GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux ./cmd/engramd

# 3. Chạy tất cả các test (bao gồm keeper tests, abci tests, x/da, x/vigilante, tests/e2e)
# -race: lớp phòng thủ cuối cho các struct proto-generated (vd. PeripheralMetrics)
# nhúng sync.Mutex marker qua protoimpl.MessageState -- bắt các copy-theo-giá-trị
# ẩn sau unsafe/generic mà go vet's copylocks check có thể bỏ sót (xem
# docs/DEVELOPMENT.md's pointer-bug note).
test:
	@echo "--> Running tests..."
	go test -v -race ./...

# 4. Kiểm tra code (Linting)
lint:
	@echo "--> Running golangci-lint..."
	golangci-lint run

# 5. Sinh mã Protobuf (Cần cài đặt buf + protoc-gen-go + protoc-gen-go-grpc)
# buf.gen.yaml generates with paths=source_relative into .tmp-proto-gen/
# (mirroring the proto package path engram/sovereignty/v1/), since the proto
# package (engram.sovereignty.v1) and the Go package (x/sovereignty/types)
# intentionally differ -- this recipe copies the generated files into place
# and removes the staging directory.
proto-gen:
	@echo "--> Generating protobuf code..."
	rm -rf .tmp-proto-gen
	buf generate
	cp .tmp-proto-gen/engram/sovereignty/v1/*.go x/sovereignty/types/
	rm -rf .tmp-proto-gen

# 6. Biên dịch ZK Circuit (Noir)
zk-compile:
	@echo "--> Compiling ZK circuits..."
	cd circuit/reanchoring && nargo compile

# 7. Xây dựng Docker Image
docker-build:
	@echo "--> Building docker image..."
	docker build -t engram/node:$(VERSION) .

# Dọn dẹp build
clean:
	rm -rf $(BUILD_DIR)

# Trợ giúp
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build         - Biên dịch node (engramd)"
	@echo "  test          - Chạy các test case"
	@echo "  proto-gen     - Sinh mã proto"
	@echo "  zk-compile    - Biên dịch các mạch Noir"
	@echo "  lint          - Chạy kiểm tra code"
	@echo "  docker-build  - Xây dựng image docker"