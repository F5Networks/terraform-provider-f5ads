default: fmt lint build

build:
	go build -v ./...

deps:
	go mod tidy

install: build
	go install -v ./...

lint:
	golangci-lint run

fmt:
	gofmt -s -w -e .

generate:
	go generate ./...

test:
	go test -v -cover -timeout=120s -parallel=10 ./...

testacc:
	TF_ACC=1 go test -v -cover -timeout 60m ./...

.PHONY: fmt lint test testacc build install generate deps
