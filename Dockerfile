# golang:1.27-alpine (go 1.27.4), keep it on a supported go release: the
# previous pin (go 1.21.0) fetched the unpatched go 1.25.0 named in go.mod
FROM golang:1.27-alpine@sha256:eb61c3873c97416fff0116553fff65d67a3549f72e11d9758849a689cdd8b523

RUN apk update && apk --no-cache add upx ca-certificates

WORKDIR /go/src/github.com/Avielyo10/goflake
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o goflake
RUN upx --best --lzma goflake

FROM scratch

LABEL org.opencontainers.image.source=github.com/Avielyo10/goflake
LABEL org.opencontainers.image.description="GoFlake is a distributed unique ID generator inspired by Twitter's Snowflake."

COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=0 /go/src/github.com/Avielyo10/goflake/goflake /goflake
# don't run as root, config and tls files mounted into the container must be readable by this user
USER 65534:65534
ENTRYPOINT ["/goflake"]
