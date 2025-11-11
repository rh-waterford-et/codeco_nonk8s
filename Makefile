.PHONY: lint fmt test build clean

lint:
	golangci-lint run ./...

fmt:
	gofmt -s -w .

test:
	go test -v -race -coverprofile=coverage.out ./...

test-contract:
	go test -v -race ./tests/unit/*_contract_test.go

test-integration:
	go test -v -race ./tests/integration/...

build:
	go build -o bin/vk-flightctl-provider ./cmd/vk-flightctl-provider

clean:
	rm -rf bin/ coverage.out

.DEFAULT_GOAL := build
