# Redbus — рабочие инструкции для агентов

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

## Правила

Отдельные guidance-файлы автоматически не подгружаются, поэтому перед работой открой и соблюдай
каждое применимое правило:

- [`guidance/readme-ai.md`](guidance/readme-ai.md) — перед содержательными изменениями прочитай
  ближайший `ReadmeAI.md` вверх по дереву и после изменений бизнес-логики оцени обновление заметки.

## Назначение проекта

Redbus — сервис над Kafka для гарантированной обработки сообщений. Продюсеры публикуют сообщения
через gRPC, consumers возвращают результат обработки по двунаправленному stream. Неуспешные
сообщения сохраняются в PostgreSQL и повторяются по настраиваемой стратегии; HTTP admin API и
Vue-интерфейс показывают статистику и позволяют перезапускать окончательно упавшие сообщения.

## Команды

Основной модуль использует версию Go из `go.mod`.

- `go test ./...` — все Go-пакеты вместе с проверками `go test`/vet.
- `go test ./internal/app/model -run '^TestRepeatStrategy$' -count=1` — пример запуска одного теста;
  меняй пакет и точное имя теста по области изменений.
- `make build` — собрать сервер в `bin/redbus`; `make build-all` дополнительно собирает Go-примеры.
- `make fmt` — применить `gofmt` и `goimports` ко всем Go-пакетам, кроме `api/`; цель при
  необходимости скачивает `goimports` в `bin/`.
- `make gen` — заново сгенерировать Go protobuf/gRPC-код из `api/api.proto` в `api/golang/pb/`.
- `docker compose -f example/docker-compose.yml up` — поднять Kafka и PostgreSQL для локального
  запуска; сервер читает `config.json`, затем необязательный `config.local.json`, затем env.
- В `web/admin/`: `npm ci`, `npm run serve`, `npm run build`, `npm run lint`.
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
  `api/scala/redbus`; Vue admin в `web/admin` работает с отдельным HTTP/SSE API.
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
