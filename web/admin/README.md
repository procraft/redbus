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

The production admin image deliberately forces `REDBUS_API_HOST` to an empty value and uses the same-origin
`/api` endpoint served by `redbus-admin`. This keeps one image portable between environments and
prevents a build-time host from routing production UI requests to another environment.

## Checks and production build

```shell
yarn check
```

The command runs TypeScript type checking, Oxlint, and the Rspack production build. Build artifacts are written to `dist/`.
