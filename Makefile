.PHONY: all run up armv7 test

LISTEN ?= :18080

all: amd64 armv7

run:
	mkdir -p data
	go run ./cmd/armada -listen $(LISTEN) -db-path ./data

up:
	mkdir -p docker-data
	docker compose up --build

amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/armada-amd64 ./cmd/armada

armv7:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -ldflags="-s -w" -o bin/armada-armv7 ./cmd/armada

test:
	go test ./...
