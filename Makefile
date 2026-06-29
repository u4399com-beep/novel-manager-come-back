.PHONY: build run test lint clean docker-build docker-up docker-down

APP_NAME := novel-server
GO := go
GOFLAGS := -ldflags="-s -w"

build:
	$(GO) build $(GOFLAGS) -o $(APP_NAME) ./cmd/server/

run:
	$(GO) run ./cmd/server/

test:
	$(GO) test ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(APP_NAME)
	rm -rf data/

docker-build:
	docker build -t novel-come-back .

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

tidy:
	$(GO) mod tidy

fmt:
	$(GO) fmt ./...

all: fmt tidy lint build
