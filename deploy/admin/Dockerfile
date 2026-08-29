FROM node:24-alpine AS builder

ARG apiHost
ARG apiToken
ENV REDBUS_API_HOST=$apiHost
ENV REDBUS_API_TOKEN=$apiToken

WORKDIR /app
RUN corepack enable
COPY ./web/admin/package.json ./web/admin/yarn.lock ./web/admin/.yarnrc.yml ./
RUN yarn install --immutable
COPY ./web/admin .
RUN yarn build


FROM nginx:1.31.4-alpine
EXPOSE 80
WORKDIR /usr/share/nginx/html

COPY --from=builder /app/dist .
