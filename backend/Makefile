.PHONY: build run test lint clean

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/netmonitor ./cmd/server

build-pcap:  # With packet capture support (requires libpcap-dev)
	CGO_ENABLED=1 go build -ldflags="-s -w" -tags pcap -o bin/netmonitor ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./... -v -count=1

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/
