prep:
	@buf generate --path api --template api/protobuf/buf.gen.yaml
	@go mod tidy
	@go fmt ./...
	@go vet ./...

build: prep
	@go build -o goflake .

run: prep
	@LOG_LEVEL=debug go run .

release:
	goreleaser release --skip-publish --rm-dist

docker:
	docker build -t ghcr.io/avielyo10/goflake:latest .
