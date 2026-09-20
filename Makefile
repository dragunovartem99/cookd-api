.DEFAULT_GOAL := check

# check is what CI runs.
.PHONY: check
check: fmt-check vet test

.PHONY: fmt
fmt:
	gofmt -w .

.PHONY: fmt-check
fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test:
	go test -race ./...

.PHONY: cover
cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

.PHONY: build
build:
	go build -trimpath -o bin/cookd-api ./cmd/cookd-api

# run serves the API locally with .env.development.
.PHONY: run
run:
	env $$(grep -v '^#' .env.development | xargs) go run ./cmd/cookd-api

# token prints a session token for EMAIL, to call the API from curl without Google.
.PHONY: token
token:
	@env $$(grep -v '^#' .env.development | xargs) go run ./cmd/cookd-api token $(EMAIL)
