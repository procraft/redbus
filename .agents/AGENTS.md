# Redbus — рабочие инструкции для агентов

<!-- shared-guidance:start -->
## Shared baseline guidance

Before substantive work, read and follow:

- `.agents/guidance/readme-ai.md` — module-local context discovery and documentation maintenance.
- `ReadmeAI.index.md` — repository-wide map for finding relevant context outside the nearest
  directory walk-up path.

Store committed durable AI-facing reference docs in `.agents/docs/`; keep temporary contracts,
plans, test plans/reports, and drafts in the git-ignored `.agents/proposals/`; keep topic directories
directly under `.agents/<topic>/`. Keep AI-only helper scripts in `.agents/scripts/`;
`docs/ai/` and `dev-tools/ai/` are legacy locations. Preserve hard design decisions as ADRs under
the coordination workspace's `docs/adr/`.

### ReadmeAI index maintenance

After creating, deleting, or moving a `ReadmeAI.md`, or changing its H1 or first summary paragraph,
run `make readme-index`. Validate the committed result with `make readme-index-check`.

A pre-commit hook may run `make readme-index-hook` and stage only the generated index. Pre-push hooks
and CI must never regenerate committed files; they should run `make readme-index-check` and fail on
drift.
<!-- shared-guidance:end -->

## Единая агентская конфигурация

`.agents/` — единственный источник общих инструкций, правил и проектных скилов:

- `.agents/AGENTS.md` — этот файл, общие инструкции для всех агентов.
- `.agents/guidance/` — обязательные правила с frontmatter `description` и `alwaysApply`.
- `.agents/skills/` — проектные скилы в каталогах `<name>/SKILL.md`.
- `.agents/scripts/` — вспомогательные скрипты, не относящиеся к отдельным скилам.
- `.agents/docs/` — подробный контекст, который читается по необходимости.
- `.agents/proposals/` — планы, черновики и временные файлы. Клади их сюда, а не в корень проекта
  и не в `/tmp`; содержимое каталога не коммитится.

`AGENTS.md` и `CLAUDE.md` в корне, а также файлы в `.claude/skills/`, `.opencode/skills/` и
`.cursor/rules/` — генерируемые симлинки. Не редактируй их напрямую. Правь источник в `.agents/`
и затем запускай `make agent-adapters`. Для проверки без изменений используй
`make agent-adapters-check`.

## Назначение проекта

Redbus — сервис над Kafka для гарантированной обработки сообщений. Продюсеры публикуют сообщения
через gRPC, consumers возвращают результат обработки по двунаправленному stream. Неуспешные
сообщения сохраняются в PostgreSQL и повторяются по настраиваемой стратегии; HTTP admin API и
React-интерфейс показывают статистику и позволяют перезапускать окончательно упавшие сообщения.

## Команды

Основной модуль использует версию Go из `go.mod`.

- `go test ./...` — все Go-пакеты вместе с проверками `go test`/vet.
- `go test ./internal/app/model -run '^TestRepeatStrategy$' -count=1` — пример запуска одного теста;
  меняй пакет и точное имя теста по области изменений.
- `make build` — собрать сервер в `bin/redbus`; `make build-all` дополнительно собирает Go-примеры.
- `make fmt` — применить `gofmt` и `goimports` ко всем Go-пакетам, кроме `api/`; цель при
  необходимости скачивает `goimports` в `bin/`.
- `make gen` — заново сгенерировать Go protobuf/gRPC-код из `api/api.proto` в `api/golang/pb/`.
- `make hooks` — один раз подключить repository-managed Git hooks для текущего checkout.
- `docker compose -f example/docker-compose.yml up` — поднять Kafka и PostgreSQL для локального
  запуска; сервер читает `config.json`, затем необязательный `config.local.json`, затем env.
- В `web/admin/`: `yarn install --immutable`, `yarn dev`, `yarn build`, `yarn lint`, `yarn check`.
- В `api/scala/redbus/`: `sbt test`; ScalaPB генерируется средствами sbt.

## Архитектура

- `cmd/redbus` загружает конфигурацию и передаёт управление `internal/pkg/app`. Там собираются
  PostgreSQL, Kafka, сервисы, gRPC, admin HTTP и фоновые задачи; все долгоживущие процессы связаны
  общим `context.Context` и `errgroup`.
- `internal/app/grpcapi` и `internal/app/adminapi` — транспортные границы. Они конвертируют запросы
  в доменные типы и вызывают интерфейсы сервисов, но не должны содержать SQL или Kafka-детали.
- `internal/app/service/databus` координирует publish/consume; `connstore` хранит активные
  producers/consumers; `repeater` сохраняет неуспешную обработку и фоново отправляет сообщения
  подходящему подключённому consumer.
- `internal/app/repository` содержит PostgreSQL-запросы к retry-очереди. Соединение или транзакция
  передаются через context interceptors из `internal/pkg/db`; repository получает их через
  `db.FromContext`.
- `internal/app/model` — доменные типы, retry-стратегии и интерфейсы Kafka-клиентов. Реализации
  инфраструктуры находятся в `internal/pkg/`.
- `api/api.proto` — источник gRPC-контракта. Go-клиент лежит в `api/golang`, Scala-клиент — в
  `api/scala/redbus`; React admin в `web/admin` работает с отдельным HTTP/SSE API.
- Серверные изменения схемы PostgreSQL идут в `internal/migrations/`. `api/inbox.sql` и
  `api/outbox.sql` — схемы интеграции клиентских приложений, а не миграции серверной retry-таблицы.

## Локальные соглашения

- После изменения `api/api.proto` запускай `make gen`; не редактируй `api/golang/pb/*.pb.go`
  вручную и проверяй совместимость Go- и Scala-клиентов.
- Интерфейсы зависимостей объявляются рядом с потребителем (`grpcapi`, `databus`, `repeater`), а
  зависимости передаются через `New(...)`. Сохраняй это разделение при добавлении поведения.
- Repository-методы рассчитывают на DB в `context.Context`; при новых HTTP/gRPC/background путях
  не обходи DB middleware/interceptors и не передавай глобальное соединение напрямую.
- Конфигурационные значения должны иметь JSON-поле в `config.json`/`internal/config` и, если нужна
  операция через окружение, тег `REDBUS_*`. Локальные секреты и overrides держи в игнорируемом
  `config.local.json`.
- Для изменений retry-модели синхронно проверяй сериализацию стратегии, расчёт `started_at`,
  соответствие SQL-полей `repeatFields` порядку `repeatScanDest` и миграции таблицы `repeat`.
