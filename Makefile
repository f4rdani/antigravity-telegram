.PHONY: all build linux clean test

BINARY_NAME=agy-tele

all: build

build:
	go build -ldflags="-s -w" -o bin/$(BINARY_NAME) ./cmd/agy-tele

linux:
	set CGO_ENABLED=0&& set GOOS=linux&& set GOARCH=amd64&& go build -ldflags="-s -w" -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/agy-tele

clean:
	rm -rf bin/ sessions.json
