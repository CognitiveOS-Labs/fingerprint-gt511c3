VERSION ?= 0.1.0
NAME := com.cognitiveos.labs.fingerprint-gt511c3
BIN := tools/gt511c3-mcp
CGP := $(NAME)-$(VERSION)-linux-amd64.cgp

GOFLAGS := -trimpath -ldflags "-s -w"

.PHONY: build test vet fmt lint pack verify clean

build:
	CGO_ENABLED=0 go build $(GOFLAGS) -o $(BIN) ./cmd/gt511c3-mcp

test:
	go test -timeout 60s ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

lint: vet fmt
	@test -z "$$(gofmt -l .)" || (echo "gofmt: files need formatting" && exit 1)

pack: build
	cpm pack

verify: pack
	cpm verify $(CGP)

clean:
	rm -f $(BIN) $(CGP)
