FROM golang@sha256:445f34008a77b0b98bf1821bf7ef5e37bb63cc42d22ee7c21cc17041070d134f

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
ENTRYPOINT ["/goflake"]