# RED Bus admin panel

React + TypeScript admin panel built with Rspack and Mantine.

## Requirements

- Node.js 22.22 or newer (Node.js 24 is used in Docker)
- Corepack with Yarn 4.18.0

When using NVM, install and activate Node.js 24, then enable Corepack:

```shell
nvm install 24
nvm use 24
corepack enable
```

`nvm install 24` and `corepack enable` only need to be run once. Use `nvm use 24` for each new shell.

## Development

Install the exact dependency tree from `yarn.lock`:

```shell
yarn install --immutable
```

Start the development server on <http://localhost:8081>:

```shell
yarn dev
```

The API connection is configured in `.env`:

```dotenv
REDBUS_API_HOST=http://localhost:50006
REDBUS_API_TOKEN=changeme
```

The production admin image reads the API host from the container environment at runtime. The same image can
therefore be promoted between environments:

```shell
docker run -e REDBUS_API_HOST=https://redbus-api.sohoup.ru lms-redbus-admin:latest
```

The Go server exposes the value as a non-cacheable `/runtime-config.js` before the React bundle starts. Admin
startup fails when `REDBUS_API_HOST` is missing, preventing an accidental same-origin fallback. The UI and API
must use separate origins because the UI ingress uses browser-managed HTTP Basic authentication while the admin
API uses `Authorization: Token`. DNS, TLS, and CORS for the configured API origin are infrastructure prerequisites.

## Checks and production build

```shell
yarn check
```

The command runs TypeScript type checking, Oxlint, and the Rspack production build. Build artifacts are written to `dist/`.
