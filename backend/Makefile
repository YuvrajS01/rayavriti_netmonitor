BINARY := bin/netmonitor
CMD     := ./cmd/server

.PHONY: build run test lint clean docker-build tidy

build:
	CGO_ENABLED=0 go build -tags netgo -o $(BINARY) $(CMD)

build-pcap:
	go build -o $(BINARY) $(CMD)

run: build
	./$(BINARY)

test:
	CGO_ENABLED=0 go test -tags netgo ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/

docker-build:
	docker build -t netmonitor-backend .

tidy:
	go mod tidy
