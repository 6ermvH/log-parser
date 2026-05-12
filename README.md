# Log Parser

[![CI](https://github.com/6ermvH/log-parser/actions/workflows/check-correctness.yml/badge.svg?branch=main)](https://github.com/6ermvH/log-parser/actions/workflows/check-correctness.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/6ermvH/log-parser)](https://github.com/6ermvH/log-parser/blob/main/go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/6ermvH/log-parser)](https://goreportcard.com/report/github.com/6ermvH/log-parser)

Тестовое задание на стажировку в **YADRO**, команда прикладной разработки.

Микросервис на Go, который принимает архивы диагностики `ibdiagnet2`, разбирает секционный CSV, агрегирует топологию InfiniBand (узлы / порты / связи) и сохраняет результат в PostgreSQL. Наружу доступен REST API.

## Запуск

```bash
cp .env.example .env
docker compose up --build
```

Сервис стартует на `http://localhost:8080`. При первом запуске накатываются миграции на свежую БД.

Положите архив в `data/` (папка смонтирована в контейнер как `/app/data`) и запустите парсинг:

```bash
curl -X POST http://localhost:8080/api/v1/parse \
  -H 'Content-Type: application/json' \
  -d '{"path":"log.zip"}'
```

В ответ придёт `log_id` (202 Accepted), статус разбора смотрите через `GET /api/v1/log/{log_id}`.

## Конфигурация

| Источник | Параметр         | Назначение                                                 |
| -------- | ---------------- | ---------------------------------------------------------- |
| ENV      | `DATABASE_URL`   | строка подключения к Postgres (обязательно)                |
| ENV      | `PORT`           | порт HTTP-сервера (default `8080`)                         |
| ENV      | `LOG_LEVEL`      | `debug` / `info` / `warn` / `error`                        |
| ENV      | `DATA_DIR`       | путь к папке с архивами (default `./data`)                 |
| ENV      | `CONFIG_PATH`    | путь к YAML (default `./configs/config.yaml`)              |
| YAML     | `reaper.tick`    | период проверки зависших `processing` (default `30s`)      |
| YAML     | `reaper.timeout` | максимально допустимое время в `processing` (default `5m`) |

ENV, (DSN, секреты). YAML — для тюнинга в коде.

## Архитектура

Слоистая, с разделением transport / business / data:

- `cmd/server` — entry point, lifecycle (миграции на старте, graceful shutdown, фоновый reaper).
- `internal/api/v1/http` — HTTP transport: router, handlers, middleware, response/error JSON.
- `internal/service` — бизнес-логика: `ParseService` оркестрирует POST `/parse`, `QueryService` собирает ответы для GET-эндпоинтов.
- `internal/parser` — парсер логов: stream через state machine + aggregator.
- `internal/storage/postgres` — репозиторий поверх `pgxpool`, транзакционная запись `domain.Log`.
- `internal/storage/migrate` — runner поверх `golang-migrate` с `embed.FS`.
- `internal/reaper` — фоновая горутина-ETL, следит за зависшими `processing`-логами.
- `internal/domain` — доменные типы.
- `internal/config`, `internal/logger` — ENV + YAML конфиг, структурный slog в stdout.

Подробная компонентная схема (C4 level 3) — [docs/c3-components.md](docs/c3-components.md).

## API

| Метод | Путь | Назначение |
|---|---|---|
| `POST` | `/api/v1/parse` | Принимает архив на парсинг асинхронно: возвращает `log_id` сразу, парсинг идёт в фоне |
| `GET` | `/api/v1/log/{log_id}` | Текущий `status` (`processing` / `ok` / `failed`), счётчики, текст ошибки |
| `GET` | `/api/v1/topology/{log_id}` | Узлы, порты и связи лога |
| `GET` | `/api/v1/node/{node_id}` | Детали узла + расширенная информация (switch/system/sharp) |
| `GET` | `/api/v1/port/{node_id}` | Список портов узла |
| `GET` | `/health` | Liveness, включая ping БД |

**Async flow**: `POST /parse` сразу отвечает `202 Accepted` с `log_id` — фактический парсинг живёт в горутине. Клиент поллит `GET /log/{log_id}` до перехода `status` из `processing` в `ok` / `failed`. Если приложение упало посреди парсинга, [reaper](#допущения-и-ограничения) переводит зависшие записи в `failed` по таймауту.

Поэтапные потоки каждого эндпоинта (включая фоновую горутину парсера и reaper) — в [docs/sequences.md](docs/sequences.md).

**Postman-коллекция** автоматически генерируется из OpenAPI-спеки и лежит в [docs/postman_collection.json](docs/postman_collection.json) - импортируйте в Postman одним кликом, переменная `{{baseUrl}}` уже выставлена на `http://localhost:8080`. OpenAPI 3.1 спека: [docs/openapi/openapi.json](docs/openapi/openapi.json).

## База данных

Шесть таблиц с FK и `ON DELETE CASCADE` от корневой `logs`:

- `logs` — снапшот загрузки
- `failed_logs` — детали ошибок парсинга (1:1 с `logs`)
- `nodes` — узлы фабрики (host / switch / router / unknown)
- `ports` — порты узлов
- `nodes_info` — расширенные блоки (`switch_info`, `system_info`, `sharp_info`) как JSONB
- `connections` — рёбра графа (`port_a_id`, `port_b_id`) с нормализацией направления и `UNIQUE`

Полная диаграмма связей с кардинальностями — [docs/erd.md](docs/erd.md).

Схема создаётся и эволюционирует через **golang-migrate** с `//go:embed migrations/*.sql`. Файлы миграций живут в `migrations/`, применяются автоматически при старте приложения (`internal/storage/migrate`), CI не требует отдельного шага накатки.

## Парсер

Парсер собран из двух независимых слоёв, чтобы изменять формат отдельно от доменной логики:

- **State machine** (`internal/parser/statemachine.go`) — стрим построчно, состояния `Outside / Header / Body`. Знает только про секционный CSV-формат `START_X / header / data / END_X`, эмитит события `(section, columns, row)` через callback. Не знает про InfiniBand.
- **Aggregator** (`internal/parser/aggregator.go`) — принимает события, маппит CSV-строки в доменную модель `Log → Node → Port` + `NodeInfo` + `Connection`. Не знает про zip и стрим.

Парсер строгий: отказывает в файле целиком при любом из двух нарушений (см. раздел про допущения).

## Тестирование

Покрытие по слоям, замеряется `go test -cover`
Все логические пакеты держат покрытие **≥ 70%** (правило проекта)

```bash
go test ./...                                    # unit
go test -tags integration -timeout 5m ./...      # integration (нужен Docker)
go test -tags e2e -timeout 5m ./tests/e2e/...    # e2e (нужен Docker)
go generate ./...                                # перегенерация моков (gomock)
golangci-lint run ./...                          # линт
```

Build-tag разделение: unit `//go:build !integration`, integration `//go:build integration`, e2e `//go:build e2e`. В CI три отдельных job-а.

## Граф топологии (LINKS)

В нашей модели:
- **Узлы графа** — записи в `nodes`.
- **Рёбра графа** — записи в `connections (port_a_id, port_b_id)`, обе ссылки на `ports`. Каждый кабель = ровно одна строка, направление нормализовано (`port_a_id < port_b_id`).

В приложенном к заданию `testdata/log.zip` есть четыре секции (`NODES`, `PORTS`, `SWITCHES`, `SYSTEM_GENERAL_INFORMATION`), но **нет секции с физическими связями** между портами. Поэтому `GET /api/v1/topology` для него возвращает `edges: []`.

> **Не верифицировано:** реальный формат секции с информацией о соединениях в выводе `ibdiagnet2` я не сверял с документацией NVIDIA . По здравому смыслу такие данные могут лежать либо отдельной секцией в `db_csv`

Под **предположение**, что в `db_csv` найдётся секция `LINKS` со столбцами «два конца кабеля», парсер уже расширен:

```
START_LINKS
NodeGuid1,PortNum1,NodeGuid2,PortNum2,LinkSpeed,LinkWidth
0xhost1,1,0xswitch1,1,EDR,4x
0xswitch1,33,0xswitch2,33,EDR,4x
...
END_LINKS
```

`testdata/log_with_links.zip` — синтетика по этой же гипотезе, используется в e2e-тесте. **На реальных данных** имя секции или её формат, скорее всего, будут другими — придётся свериться с настоящим выводом `ibdiagnet2` и поправить `internal/parser/aggregator.go` (ветка `case sectionLinks`) под фактический формат. Сама схема `connections` от этого не меняется — это просто другой источник тех же данных.
## Допущения и ограничения

- **Аутентификации нет.** Это тестовое задание, выставлять API наружу не предполагается. В реальном сервисе сюда добавили бы middleware с JWT / API-ключом.
- **Reaper работает в одном инстансе.** Фоновый ETL, помечающий зависшие `processing`-логи как `failed`, не координируется между несколькими копиями приложения. Для multi-instance потребовался бы `pg_try_advisory_lock` или внешний шедулер.
- **Ошибки парсинга нигде не теряются.** На каждой ошибке:
  1. Запись логируется в **stdout** структурным slog (`level=WARN`, `err=…`, `parse_duration_ms=…`).
  2. Возвращается клиенту в теле ответа POST `/parse` вместе с `log_id`.
  3. Сохраняется в БД в `failed_logs(log_id, error_message)` со статусом лога `failed` — для последующего анализа.
- **Reaper (ETL для зависших логов).** Если приложение упало посреди парсинга или клиент оборвал запрос — запись в `logs` остаётся в состоянии `processing` навсегда. Reaper раз в `reaper.tick` (default `30s`) ищет такие записи и переводит в `failed`, если они «висят» дольше `reaper.timeout` (default `5m`).
- **Жёсткая валидация формата CSV — две проверки в state machine:**
  1. Кол-во полей строки должно совпадать с кол-вом колонок заголовка секции.
  2. Каждая открытая `START_X` секция должна быть закрыта парным `END_X`; иначе ошибка на EOF.

  На практике второе условие почти всегда падает через первое (битый или обрезанный `END_X` парсится как 1-токеновая «строка» и валится column count mismatch). EOF-проверка ловит только редкий кейс — все данные валидные, но файл просто обрывается без `END_X`.
- **Хранение полей `ports`.** В колонках лежат только наиболее используемые признаки (`port_state`, `port_phy_state`, `link_speed_actv`, `link_width_actv`, `lid`, `guid`). Остальные ~47 полей секции PORTS сохраняются как `raw JSONB` — это даёт гибкость без потери данных. Перенести что-то из JSONB в типизированную колонку — миграция в одну строку.
- **`sharp_an_info` не валидируется.** Файл с key=value блоками просто парсится в `map[string]string` и кладётся в `nodes_info.sharp_info`. Никаких проверок на значения (`endianness`, флаги SHARP) не делается — это конфиг, и его семантика выходит за рамки задания.
- **Postman-коллекция генерируется частично автоматически.** Источник правды — аннотации `swag` в коде. По команде `make openapi` поднимается OpenAPI 3.1 спека, по `make postman` — Postman v2.1 collection из неё. `make docs` запускает обе цели подряд. Для Postman нужен только Node.js (через `npx`), Docker не требуется.
- **Интеграционные и e2e-тесты используют `testcontainers-go`.** Поднимают реальный Postgres в Docker, накатывают миграции, прогоняют тесты на живой БД. В e2e дополнительно поднимается HTTP-сервер через `httptest` поверх настоящего router'а и реальных зависимостей.
- **State machine вынесена в отдельный модуль.** Чистая FSM с состояниями `Outside / Header / Body` живёт независимо от агрегатора и не знает про InfiniBand. Если завтра формат входа поменяется (JSON-lines, проприетарный бинарь) — заменяется только парсер, доменный слой не трогается.

## Структура проекта

```text
cmd/server/                  # entry point
internal/
  api/v1/http/               # HTTP handlers, router, middleware, моки
  service/                   # ParseService + QueryService
  parser/                    # state machine + aggregator
  storage/postgres/          # репозиторий поверх pgx
  storage/migrate/           # golang-migrate runner
  reaper/                    # фоновый ETL
  domain/                    # доменные типы
  config/                    # ENV + YAML
  logger/                    # slog JSON
migrations/                  # SQL миграции (embed)
test_migrations/             # сиды для integration-тестов
configs/                     # YAML конфиг (reaper)
tests/e2e/                   # e2e-тесты
testdata/                    # фикстуры (log.zip, log_with_links.zip, битые логи)
docs/                        # диаграммы + OpenAPI + Postman collection
```
