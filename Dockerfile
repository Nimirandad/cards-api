FROM golang:1.27.1-alpine3.24 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/cards-api \
    ./cmd/api

FROM alpine:3.24

RUN addgroup -S app && adduser -S app -G app
COPY --from=builder /out/cards-api /usr/local/bin/cards-api

USER app
EXPOSE 8080
ENV HTTP_ADDR=:8080

ENTRYPOINT ["/usr/local/bin/cards-api"]
