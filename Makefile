.PHONY: build clean generate build-server run test test-race test-system test-all lint fmt tidy vulncheck release-check docker-build ci

build: clean generate build-server

run: generate
	@echo "Starting server..."
	@go run ./server/cmd/server start

clean:
	@echo "Cleaning output directories..."
	@npx --yes shx rm -rf ./bin
	@npx --yes shx rm -rf "./server/pkg/modules/web/dist/*"

generate:
	@echo "Generating embedded web assets..."
	@go generate ./...

build-server:
	@echo "Building Go Server..."
	@npx --yes cross-env GOOS=linux GOARCH=amd64 go build -o ./bin/server.bin ./server/cmd/server
	@npx --yes cross-env GOOS=windows GOARCH=amd64 go build -o ./bin/server.exe ./server/cmd/server

test:
	@echo "Running unit tests..."
	@go test ./server/...

test-race:
	@echo "Running unit tests with the race detector..."
	@go test -race -shuffle=on ./server/...

test-system: generate
	@echo "Running system tests..."
	@go test -tags system -count=1 -v -timeout 15m ./server/systemtest/...

test-all: test-race test-system

lint:
	@echo "Linting Go..."
	@go vet ./server/...
	@echo "Linting the web UI..."
	@npm --prefix web run lint

fmt:
	@echo "Formatting..."
	@gofmt -w ./server
	@npm --prefix web run format

tidy:
	@go mod tidy

vulncheck:
	@echo "Scanning for known vulnerabilities..."
	@go run golang.org/x/vuln/cmd/govulncheck@latest ./server/...

release-check:
	@echo "Building a snapshot release..."
	@go run github.com/goreleaser/goreleaser/v2@latest release --clean --snapshot --skip=publish

docker-build:
	@echo "Building the container image..."
	@docker build -f docker/Dockerfile -t cubby:local .

ci: lint test-race test-system
