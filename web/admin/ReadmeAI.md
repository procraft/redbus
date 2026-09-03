# Admin UI — контекст модуля

## Назначение

`web/admin` — статическая React-админка для просмотра состояния consumers, Kafka topics и retry-очереди,
а также ручного перезапуска окончательно упавших сообщений. Она использует отдельные HTTP/SSE-контракты
из `internal/app/adminapi` и собирается в `web/admin/dist`.

## Стек и структура

- React + TypeScript, сборка через Rspack и встроенный SWC loader; точные версии зафиксированы в
  [`package.json`](package.json) и [`yarn.lock`](yarn.lock).
- UI строится на Mantine, иконки — только из `lucide-react`. Не возвращай Bootstrap, jQuery или
  локальные SVG для стандартных действий без отдельной причины.
- [`src/App.tsx`](src/App.tsx) содержит AppShell и маршруты; страницы находятся в `src/pages`, таблицы
  и live-карточки — в `src/components`, HTTP-клиент и DTO — в `src/api`.
- Используется `HashRouter`: это позволяет одинаково работать через Go `http.FileServer` и nginx без
  отдельного SPA fallback для `/topics`, `/consumers` и `/failed-repeats`.
- Страницы загружаются через route-level `lazy`/`Suspense`, чтобы таблицы статистики не попадали целиком
  в начальный JavaScript bundle dashboard.

Маршруты:

- `#/` — dashboard;
- `#/topics` — полный список Kafka topics, offsets и состояния consumer groups;
- `#/consumers` — подключённые Redbus consumers, назначения партиций, lag и runtime-состояние;
- `#/failed-repeats` — статистика retry и ручной restart.

## HTTP и SSE

Все HTTP-методы админки — `POST` относительно `/api`:

- `/dashboard/stat`;
- `/topic/stat`;
- `/consumer/stat`;
- `/repeat/stat`;
- `/repeat/repeatTopicGroup`;
- `/repeat/repeatTopicGroupSince`;
- `/repeat/repeatError`.

`/repeat/stat` возвращает для каждой пары topic/group вложенный триаж окончательно упавших сообщений,
сгруппированный по точному значению сохранённого `error`, с количеством и первым/последним `finished_at`
в каждом классе. `/repeat/repeatError` перезапускает только сообщения выбранного класса за заданный период,
а `/repeat/repeatTopicGroupSince` — все окончательно упавшие сообщения topic/group за период. Период передаётся
как положительный `lookbackSeconds` и вычисляется от серверного времени. Существующий
`/repeat/repeatTopicGroup` сохраняет прежнее поведение без временного фильтра для обратной совместимости.

`/topic/stat` возвращает все пользовательские Kafka topics, включая те, для которых нет подключённых
Redbus consumers. Для каждой партиции Kafka `lastOffset` означает high watermark, то есть offset следующего
сообщения. Для подключённых topic/group backend дополнительно получает состояние Kafka consumer group,
committed offsets и назначения партиций. Lag считается как `lastOffset - committedOffset`; если offset ещё
не был committed, начальной точкой считается `firstOffset`, а `committed` остаётся `false`.

`/consumer/stat` — плоский consumer-разрез того же снимка. Он объединяет назначения и offsets из Kafka с
runtime-метриками Redbus: состоянием соединения, временем подключения и последнего сообщения, количеством
обработанных сообщений, reconnects, retry strategy и последней ошибкой. Runtime-метрики живут в памяти
процесса Redbus и сбрасываются после его перезапуска. Kafka `ClientID` consumer задаётся равным Redbus
consumer id, чтобы назначение партиции можно было связать с конкретным подключением.

HTTP-клиент передаёт `Authorization: Token <token>`. UI и API должны находиться на разных origins: внешний
ingress UI использует тот же стандартный header для управляемого браузером HTTP Basic auth, поэтому
same-origin API-запрос заменит Basic credentials API-токеном и снова получит Basic challenge.

Go-сервер читает `REDBUS_API_HOST` из окружения контейнера и отдаёт его как JSON-экранированный
`/runtime-config.js` с `Cache-Control: no-store`; HTML загружает этот script до React bundle. При пустом host
admin завершается на старте, поэтому один и тот же образ можно безопасно продвигать между контурами. Rspack
значение из `.env` остаётся только fallback для локального dev server.

`REDBUS_API_TOKEN` подставляется Rspack во время сборки, поэтому токен оказывается в browser bundle и не
является секретом. Для настоящей защиты нужна серверная аутентификация или reverse proxy, а не попытка
скрыть build argument.

Live-статистика приходит из `/api/events` через native `EventSource`:

- событие `consumers`: `consumerCount`, `consumeTopicCount`;
- событие `repeater`: `allCount`, `failedCount`.

Исторически Vue-клиент слушал ошибочное имя `customers`, а сервер отправлял поле `failedount`. Серверный
контракт исправлен в `internal/app/model/event.go`; React-клиент пока принимает и `failedount` для плавного
обновления старых инсталляций. Не удаляй этот fallback без решения о прекращении обратной совместимости.

Обычный `EventSource` не умеет задавать `Authorization` header, поэтому SSE route сейчас публичный. Если
на `/api/events` будет добавлена авторизация, одновременно потребуется cookie/session, signed URL или другой
совместимый с EventSource механизм.

API списка отдельных failed messages отсутствует. React UI показывает агрегированный по точному тексту
ошибки триаж и поддерживает restart как всего topic/group, так и одного класса ошибки.

## Локальный запуск и проверки

Проект использует Yarn 4 через Corepack и требует Node.js `>=22.22`; для разработки принят Node.js 24:

```shell
nvm install 24       # один раз
nvm use 24           # в новой shell-сессии
corepack enable      # один раз для установленного Node
yarn install --immutable
yarn dev
```

Если запускается глобальный Yarn 1.22 из Homebrew, значит он раньше Corepack shim в `PATH`. После
`nvm use 24` команды `which node`, `which yarn` должны указывать в один каталог NVM, а `yarn --version` —
показывать версию из поля `packageManager`.

В `.yarnrc.yml` точечно разрешены версии, которые на момент фиксации были новее стандартного суточного
Yarn age gate и были проверены по registry. Не отключай age gate глобально и не расширяй исключения без
необходимости.

Основная проверка:

```shell
yarn check
yarn npm audit --all --recursive
```

Rspack может предупреждать о raw-размере общего Mantine bundle; это не ошибка сборки. Перед добавлением
крупных UI-зависимостей оцени gzip-размер и необходимость route-level code splitting.

## Сборка и деплой

- [`rspack.config.mjs`](rspack.config.mjs) читает `.env`, копирует favicon и генерирует production assets
  без публичных source maps.
- Go-сервер `redbus-admin` отдаёт `./web/admin/dist` и `/api` на одном HTTP listener;
  отдельный образ [`deploy/admin/Dockerfile`](../../deploy/admin/Dockerfile) собирает dist через
  Node 24 + `yarn install --immutable` и запускает этот Go-сервер.
- DTO статистики проходят через внутренний protobuf-контракт `internal/api/admincontrol/admincontrol.proto`.
  При изменении полей Topics/Consumers/Retry пересобирай и выкатывай Redbus и admin backend согласованно;
  обновление только одного из процессов может потерять новые поля на внутренней gRPC-границе.
- Production-образ получает `REDBUS_API_HOST` только при запуске контейнера; host не вшивается в assets.
  `docker-run-admin` передаёт runtime env из обязательного `ADMIN_API_HOST`. Для production значение —
  `https://redbus-api.sohoup.ru`; Kubernetes/CI/CD должен задать `REDBUS_API_HOST` контейнеру, а DNS, TLS и
  CORS для этого origin должны быть настроены инфраструктурой.
  Отдельный origin обязателен, чтобы браузерный HTTP Basic на UI не конфликтовал с API
  `Authorization: Token`. `apiToken` остаётся build argument и для make-задачи задаётся через
  `ADMIN_API_TOKEN`.

## Связанные визуальные assets

Основной Modern Badge хранится в `doc/logo.png` и сейчас используется только корневым `README.md`.
Админка продолжает использовать отдельный `public/favicon.ico`; не считай его автоматически синхронизированным
с основным логотипом.
