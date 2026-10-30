.PHONY: build build-pcap run test lint clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/rayavriti/netmonitor-backend/internal/config.BuildVersion=$(VERSION)

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/netmonitor ./cmd/server

build-pcap:  # With packet capture support (requires libpcap-dev)
	CGO_ENABLED=1 go build -ldflags="$(LDFLAGS)" -tags pcap -o bin/netmonitor ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./... -v -count=1

lint:
	$(shell go env GOPATH)/bin/golangci-lint run ./...

clean:
	rm -rf bin/
