FROM golang:1.25-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/pg-client ./cmd/pg-client

FROM alpine:3.20

RUN apk add --no-cache ca-certificates && \
    addgroup -S app && \
    adduser -S app -G app

USER app
WORKDIR /app

COPY --from=builder /out/pg-client /app/pg-client

EXPOSE 8090

ENTRYPOINT ["/app/pg-client"]
