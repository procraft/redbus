FROM node:24-alpine AS web-builder

ARG apiToken
ENV REDBUS_API_HOST=
ENV REDBUS_API_TOKEN=$apiToken

WORKDIR /app
RUN corepack enable
COPY ./web/admin/package.json ./web/admin/yarn.lock ./web/admin/.yarnrc.yml ./
RUN yarn install --immutable
COPY ./web/admin .
RUN yarn build

FROM golang:1.24-alpine3.20 AS go-builder

WORKDIR /app
COPY ./go.mod ./go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags '-w -extldflags "-static"' -o redbus-admin ./cmd/redbus-admin/main.go


FROM alpine:3.20
RUN adduser -D -H -h /app redbus && \
    mkdir -p /app/web/admin/dist && \
    chown -R redbus:redbus /app
WORKDIR /app
USER redbus

COPY --chown=redbus --from=go-builder /app/redbus-admin /app/redbus-admin
COPY --chown=redbus --from=go-builder /app/config.json /app/config.json
COPY --chown=redbus --from=web-builder /app/dist /app/web/admin/dist

EXPOSE 8080
CMD ["/app/redbus-admin"]
