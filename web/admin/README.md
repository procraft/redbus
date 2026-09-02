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

The production admin image requires an explicit API host at build time. The UI and API must use separate
origins because the UI ingress uses browser-managed HTTP Basic authentication while the admin API uses
`Authorization: Token`:

```shell
ADMIN_API_HOST=https://redbus-api.sohoup.ru \
ADMIN_API_TOKEN=changeme \
make docker-build-admin
```

The Docker build fails when `ADMIN_API_HOST` is missing, preventing an accidental same-origin bundle. CI/CD
must set the correct host for each environment; DNS, TLS, and CORS for that API origin are infrastructure
prerequisites.

## Checks and production build

```shell
yarn check
```

The command runs TypeScript type checking, Oxlint, and the Rspack production build. Build artifacts are written to `dist/`.
