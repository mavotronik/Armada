.PHONY: run up build-armv7 test

LISTEN ?= :18080

run:
	go run ./cmd/armada -listen $(LISTEN)

up:
	docker compose up --build

build-armv7:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -ldflags="-s -w" -o bin/armada-armv7 ./cmd/armada

test:
	go test ./...
