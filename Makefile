VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
GCFLAGS :=
BUILD_FLAGS := -trimpath -ldflags="$(LDFLAGS)"

.PHONY: test lint fmt build build-linux build-all clean

test:
	go test ./...

lint:
	golangci-lint run

fmt:
	gofmt -w .

build:
	go build $(BUILD_FLAGS) -o mdflow ./cmd/mdflow/

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o mdflow-linux-amd64 ./cmd/mdflow/

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build $(BUILD_FLAGS) -o mdflow-linux-arm64 ./cmd/mdflow/

build-all: build-linux-amd64 build-linux-arm64
	ls -lh mdflow-linux-*

clean:
	rm -f mdflow mdflow-linux-* mdflow-upx
