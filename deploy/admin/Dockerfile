FROM node:fermium AS builder

ARG apiHost
ARG apiToken
ARG CACHE_BUST=1
ENV VUE_APP_REDBUS_API_HOST=$apiHost
ENV VUE_APP_REDBUS_API_TOKEN=$apiToken
WORKDIR /app
COPY ./web/admin .
RUN --mount=type=cache,id=cache-bust,target=/yarn echo $CACHE_BUST && yarn && yarn build


FROM nginx
EXPOSE 80
WORKDIR /usr/share/nginx/html

COPY --from=builder /app/dist .