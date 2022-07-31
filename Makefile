prep:
	@buf generate --path api --template api/protobuf/buf.gen.yaml
	@go mod tidy
	@go fmt ./...
	@go vet ./...

build: prep
	@go build -o goflake ./internal

run: prep
	@LOG_LEVEL=debug go run ./internal