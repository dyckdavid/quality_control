.PHONY: build build-linux build-all test vet check run secret

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X quality_control/internal/buildinfo.Version=$(VERSION)

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/quality-control ./cmd/quality-control

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/quality-control-linux-amd64 ./cmd/quality-control
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/quality-control-linux-arm64 ./cmd/quality-control

build-all: build-linux
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/quality-control-darwin-amd64 ./cmd/quality-control
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/quality-control-darwin-arm64 ./cmd/quality-control
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/quality-control-windows-amd64.exe ./cmd/quality-control

test:
	go test ./...

vet:
	go vet ./...

check: vet test

run:
	go run ./cmd/quality-control

secret:
	openssl rand -base64 48
