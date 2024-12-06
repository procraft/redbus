FROM golang:1.23-alpine3.20 AS builder

WORKDIR /app

# for cache go mod depends
COPY ./go.mod .
COPY ./go.sum .
RUN go mod download

COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags '-w -extldflags "-static"' -o redbus ./cmd/redbus/main.go


FROM alpine:3.20
RUN adduser -D -H -h /app redbus && \
    mkdir -p /app/internal/migrations && \
    chown -R redbus:redbus /app
WORKDIR /app
USER redbus

COPY --chown=redbus --from=builder /app/redbus /app
COPY --chown=redbus --from=builder /app/config.json /app
COPY --chown=redbus --from=builder /app/internal/migrations /app/internal/migrations
COPY --chown=redbus --from=builder /app/deploy/YandexCA.crt /usr/local/share/ca-certificates/YandexCA.crt

CMD ["/app/redbus"]
