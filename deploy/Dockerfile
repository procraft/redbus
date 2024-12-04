FROM golang:1.23-alpine3.20 as builder

WORKDIR /app

# for cache go mod depends
COPY ../go.mod .
COPY ../go.sum .
RUN go mod download

COPY .. .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags '-w -extldflags "-static"' -o redbus ./cmd/redbus/main.go


FROM alpine:3.20
EXPOSE 8080
RUN adduser -D -H -h /app redbus && \
    mkdir -p /app  && \
    chown -R redbus:redbus /app
WORKDIR /app
USER redbus

COPY --chown=redbus --from=builder /app/redbus /app
COPY --chown=redbus --from=builder /app/config.json /app

CMD ["/app/redbus"]
