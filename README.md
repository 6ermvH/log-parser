# Log Parser

[![CI](https://github.com/6ermvH/log-parser/actions/workflows/check-correctness.yml/badge.svg?branch=main)](https://github.com/6ermvH/log-parser/actions/workflows/check-correctness.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/6ermvH/log-parser)](https://github.com/6ermvH/log-parser/blob/main/go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/6ermvH/log-parser)](https://goreportcard.com/report/github.com/6ermvH/log-parser)

Тестовое задание на стажировку в YADRO, команда прикладной разработки.

Микросервис на Go: принимает архивы диагностики `ibdiagnet2`, разбирает секционный CSV, собирает из него топологию InfiniBand (узлы и порты) и кладёт всё это в PostgreSQL. Наружу торчит REST API.

## Запуск

```bash
cp .env.example .env
docker compose up --build
```

Сервис поднимется на `http://localhost:8080`. На первом запуске сам накатит миграции на пустую БД.

Дальше кладёте архив в `data/` (эта папка монтируется в контейнер как `/app/data`) и отправляете на парсинг:

```bash
curl -X POST http://localhost:8080/api/v1/parse \
  -H 'Content-Type: application/json' \
  -d '{"path":"log.zip"}'
```

В ответ прилетит `log_id` со статусом 202 Accepted. Сам разбор идёт в фоне, статус можно тянуть через `GET /api/v1/log/{log_id}`. Если файла нет или это не zip, сервис ответит 400 сразу, без `log_id` и без записи в БД.

## Конфигурация

Через ENV задаются вещи, которые меняются от окружения к окружению (DSN, секреты, порт). YAML это уже про тюнинг внутренней логики.

| Источник | Параметр         | Назначение                                                 |
| -------- | ---------------- | ---------------------------------------------------------- |
| ENV      | `DATABASE_URL`   | строка подключения к Postgres, обязательно                 |
| ENV      | `PORT`           | порт HTTP-сервера, по умолчанию `8080`                     |
| ENV      | `LOG_LEVEL`      | `debug`, `info`, `warn` или `error`                        |
| ENV      | `DATA_DIR`       | путь к папке с архивами, по умолчанию `./data`             |
| ENV      | `CONFIG_PATH`    | путь к YAML, по умолчанию `./configs/config.yaml`          |
| YAML     | `reaper.tick`    | период проверки зависших `processing`, по умолчанию `30s`  |
| YAML     | `reaper.timeout` | сколько лог может висеть в `processing`, по умолчанию `5m` |

## Архитектура

Стандартная слоёная: transport, business, data.

- `cmd/server` это entry point. На старте: миграции, регистрация хендлеров, запуск reaper в горутине, graceful shutdown по сигналу.
- `internal/api/v1/http` это HTTP-слой: роутер, хендлеры, middleware, JSON-ответы.
- `internal/service` это бизнес-логика. `ParseService` оркестрирует `POST /parse`, `QueryService` собирает ответы для всех GET-эндпоинтов.
- `internal/parser` это сам парсер логов: стейт-машина для разбора секционного CSV плюс агрегатор поверх неё.
- `internal/storage/postgres` это репозиторий поверх `pgxpool`, транзакционная запись доменной модели.
- `internal/storage/migrate` это runner для `golang-migrate` с миграциями через `embed.FS`.
- `internal/reaper` это фоновая горутина, которая чистит зависшие `processing`-логи.
- `internal/domain` это доменные типы.
- `internal/config` и `internal/logger` это ENV+YAML конфиг и структурный slog в stdout.

Компонентная схема (C4 level 3) лежит в [docs/c3-components.md](docs/c3-components.md).

## API

| Метод  | Путь                        | Что делает                                                                       |
| ------ | --------------------------- | -------------------------------------------------------------------------------- |
| `POST` | `/api/v1/parse`             | ставит архив в очередь на парсинг, возвращает `log_id` (202) или 400, если файла нет или это не zip |
| `GET`  | `/api/v1/log/{log_id}`      | текущий статус (`processing`, `ok`, `failed`), счётчики, текст ошибки            |
| `GET`  | `/api/v1/topology/{log_id}` | узлы и порты лога                                                                |
| `GET`  | `/api/v1/node/{node_id}`    | детали узла плюс расширенные блоки (`switch_info`, `system_info`, `sharp_info`)  |
| `GET`  | `/api/v1/port/{node_id}`    | список портов узла                                                               |
| `GET`  | `/health`                   | liveness, ходит в БД                                                             |

`POST /parse` сначала проверяет вход: что путь корректный, файл существует и открывается как zip. Если что-то не так, возвращает 400 и на этом всё, ничего не пишется в БД. Если проверка прошла, в `logs` создаётся запись со статусом `processing`, клиент получает её `log_id` и 202 Accepted, а сам разбор уходит в горутину. Дальше клиент поллит `GET /log/{log_id}` пока статус не съедет с `processing` на `ok` или `failed`. Если приложение упало посреди парсинга, reaper потом переведёт зависшую запись в `failed` по таймауту (см. раздел про допущения).

Sequence-диаграммы по каждому эндпоинту, включая фоновую горутину парсера и reaper, лежат в [docs/sequences.md](docs/sequences.md).

Postman-коллекция автоматически генерируется из OpenAPI и лежит в [docs/postman_collection.json](docs/postman_collection.json). Импортируется в Postman одним кликом, переменная `{{baseUrl}}` уже выставлена на `http://localhost:8080`. Сама OpenAPI 3.1 спека: [docs/openapi/openapi.json](docs/openapi/openapi.json).

## База данных

Пять таблиц, всё привязано к корневой `logs` через FK с `ON DELETE CASCADE`:

- `logs`: снапшот загрузки.
- `failed_logs`: детали ошибок парсинга, связаны 1:1 с `logs`.
- `nodes`: узлы фабрики (host, switch, router, unknown).
- `ports`: порты узлов.
- `nodes_info`: расширенные блоки (`switch_info`, `system_info`, `sharp_info`) в JSONB.

Полная ERD с кардинальностями: [docs/erd.md](docs/erd.md).

Схему ведёт `golang-migrate` с `//go:embed migrations/*.sql`. Файлы миграций лежат в `migrations/` и применяются автоматически на старте приложения, отдельного шага накатки в CI нет.

## Парсер

Внутри два независимых слоя, чтобы можно было трогать формат отдельно от доменной модели:

- Стейт-машина (`internal/parser/statemachine.go`) идёт построчно. Состояния: `Outside`, `Header`, `Body`. Знает только про секционный CSV-формат `START_X`, заголовок, данные, `END_X`. По каждой строке данных вызывает колбэк с именем секции, заголовком и значениями. Про InfiniBand ничего не знает.
- Агрегатор (`internal/parser/aggregator.go`) берёт эти события и складывает CSV-строки в доменную модель: `Log`, внутри `Node`, у `Node` есть `Port` и `NodeInfo`. Про zip и про сам поток ничего не знает.

Парсер строгий: на любом нарушении формата валит весь файл целиком (про сами проверки в разделе допущений).

## Тестирование

Покрытие меряется через `go test -cover`. Все пакеты с логикой держат покрытие не меньше 70%, это правило проекта.

```bash
go test ./...                                    # unit
go test -tags integration -timeout 5m ./...      # integration, нужен Docker
go test -tags e2e -timeout 5m ./tests/e2e/...    # e2e, нужен Docker
go generate ./...                                # перегенерация моков (gomock)
golangci-lint run ./...                          # линт
```

Build-tag-и: unit под `//go:build !integration`, integration под `//go:build integration`, e2e под `//go:build e2e`. В CI это три отдельных job-а.

## Про физическую топологию (LINKS)

Физических связей между портами в `ibdiagnet2.db_csv` нет, секция с кабельной топологией в этом файле не выводится. По документации NVIDIA информация о соединениях лежит в других артефактах `ibdiagnet2`: `ibdiagnet2.net_dump`, `ibdiagnet2.lst`, `ibdiagnet2.ibnetdiscover`. Формат там не CSV, а другой табличный текст со своими правилами разбора.

Из-за этого `GET /api/v1/topology` сейчас возвращает только `nodes` и `ports`: узлы и их порты с метаданными (`state`, `phy_state`, `link_speed_actv`, `link_width_actv`, `lid`, `port_guid`). Этого хватает для «списка железа», но не хватает на кабельный граф.

Чтобы добавить рёбра, надо подключить к парсеру один из `net_dump`, `lst` или `ibnetdiscover`, написать под него отдельный парсер (нынешний разборщик секционного CSV из `internal/parser` тут не подойдёт, формат другой), завести таблицу `connections (port_a_id, port_b_id)` и отдавать рёбра в ответ `/topology`. В рамках текущего задания этого делать не стал, на вход приходит только `db_csv`.

## Покрытие ТЗ

| ID  | Требование                                                              | Реализация                                                                                                          |
| --- | ----------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| F-1 | Чтение файлов из `data/` через монтированную папку                      | volume `./data:/app/data` в `docker-compose.yml`, защита от path traversal в `internal/api/v1/http/parse_handler.go` |
| F-2 | Разбиение на секции, сборка по идентификаторам, отказ при любой ошибке  | `internal/parser/statemachine.go` плюс `internal/parser/aggregator.go`, валидации описаны ниже                      |
| F-3 | Граф `nodes` и `ports` плюс заметка о возможных связях                  | `/api/v1/topology/{log_id}` отдаёт узлы и порты, обсуждение реальных рёбер в разделе [LINKS](#про-физическую-топологию-links) |
| F-4 | Таблицы `logs`, `nodes`, `ports`, `nodes_info` через FK, автомиграции   | `migrations/` плюс `golang-migrate` через `embed.FS`, накатываются на старте `cmd/server`                           |
| F-5 | Все пять эндпоинтов                                                     | смотри таблицу API выше                                                                                             |
| F-6 | docker-compose с `app` и `db`, порт через `PORT`                        | `docker-compose.yml`                                                                                                |
| NFR | gofmt и lint                                                            | `.golangci.yaml`, отдельный job в `.github/workflows/check-correctness.yml`                                         |
| NFR | Структурные логи в stdout                                               | `internal/logger`, `slog` JSON                                                                                      |
| NFR | ENV: `DATABASE_URL`, `PORT`, `LOG_LEVEL`                                | `internal/config`, пример в `.env.example`                                                                          |
| NFR | README с инструкциями и `curl`                                          | разделы [Запуск](#запуск) и [API](#api)                                                                             |
| NFR | Обработка битых логов                                                   | статус `failed` в `logs` плюс текст ошибки в `failed_logs`, см. ниже                                                |

## Допущения и ограничения

Аутентификации нет. Это тестовое задание, выставлять API наружу не предполагается. В реальном сервисе тут стояло бы middleware с JWT или API-ключом.

Reaper работает в одном инстансе. Фоновая горутина, которая чистит зависшие `processing`-логи, не координируется между копиями приложения. Для нескольких инстансов понадобился бы `pg_try_advisory_lock` или внешний шедулер.

Парсинг сделан асинхронным сознательно. Архив `ibdiagnet2` на реальной фабрике это десятки мегабайт секционного CSV, и его разбор занимает секунды или десятки секунд. Если бы `POST /parse` держал HTTP-соединение до конца разбора, промежуточные прокси и балансеры рвали бы запрос по своим таймаутам, а клиент не понимал бы, успел ли парсер записать данные или нет. Плюс один тяжёлый запрос блокировал бы воркер на всё время разбора и снижал пропускную способность сервиса. С `202 Accepted` и поллингом `GET /log/{log_id}` клиент получает `log_id` сразу, опрашивает статус когда удобно, а retry на его стороне сводится к повторному POST с тем же файлом. Цена решения: в БД появляется состояние `processing`, и если процесс упал посреди парсинга, такая запись висит навсегда. Эту проблему закрывает reaper.

Ошибки разделены на два канала. Если файл просто не нашёлся или это не zip, проверка на входе ловит это и возвращает 400 синхронно, без `log_id` и без записи в БД (клиент видит ошибку сразу, типичный кейс с опечаткой в пути). Если же файл был открыт и парсинг начался, но в процессе всё сломалось (битый CSV, незакрытая секция, ошибка записи в БД), это уже асинхронный канал: запись лога переводится в `failed`, в stdout уходит структурная запись slog (`level=WARN`, `err=...`, `parse_duration_ms=...`), а в `failed_logs(log_id, error_message)` сохраняется текст ошибки для последующего разбора.

Reaper нужен на случай, если приложение упало посреди парсинга или клиент оборвал запрос. Запись в `logs` остаётся в `processing` навсегда, и reaper раз в `reaper.tick` (по умолчанию `30s`) ищет такие и переводит в `failed`, если они висят дольше `reaper.timeout` (по умолчанию `1m`).

Валидация формата CSV в парсере жёсткая, всего две проверки:
1. Количество полей в строке должно совпадать с количеством колонок в заголовке секции.
2. Каждая открытая `START_X` секция должна быть закрыта парным `END_X`, иначе ошибка на EOF.

На практике вторая проверка почти всегда падает через первую: битый или обрезанный `END_X` парсится как однотокеновая строка и валится на column count mismatch. EOF-проверка ловит только редкий случай, когда все данные валидные, а файл просто обрывается без `END_X`.

Поля портов сохраняются избирательно. В колонках лежат только самые ходовые признаки: `port_state`, `port_phy_state`, `link_speed_actv`, `link_width_actv`, `lid`, `guid`. Остальные примерно 47 полей секции PORTS пишутся в `raw JSONB`. Это даёт гибкость без потери данных: если завтра что-то из JSONB понадобится в типизированном виде, перенос делается миграцией в одну строку.

`sharp_an_info` не валидируется. Файл с key=value блоками просто парсится в `map[string]string` и кладётся в `nodes_info.sharp_info`. Никаких проверок на значения (`endianness`, флаги SHARP) не делается, это конфиг, и его семантика выходит за рамки задания.

Postman-коллекция собирается полуавтоматически. Источник правды это аннотации `swag` в коде. `make openapi` собирает OpenAPI 3.1 спеку, `make postman` собирает Postman v2.1 collection из неё, `make docs` запускает обе цели подряд. Для Postman нужен только Node.js (через `npx`), Docker для этого не нужен.

Integration и e2e тесты крутятся через `testcontainers-go`. Поднимается реальный Postgres в Docker, накатываются миграции, тесты гоняются на живой БД. В e2e дополнительно поднимается HTTP-сервер через `httptest` поверх настоящего роутера и реальных зависимостей.

Разбор формата вынесен в отдельный модуль. Стейт-машина с состояниями `Outside`, `Header`, `Body` живёт независимо от агрегатора и про InfiniBand не знает. Если завтра формат входа поменяется (JSON-lines, проприетарный бинарь), заменится только парсер, доменный слой не поедет.

## Структура проекта

```text
cmd/server/                  # entry point
internal/
  api/v1/http/               # HTTP handlers, router, middleware, моки
  service/                   # ParseService + QueryService
  parser/                    # стейт-машина + агрегатор
  storage/postgres/          # репозиторий поверх pgx
  storage/migrate/           # golang-migrate runner
  reaper/                    # фоновая чистка зависших логов
  domain/                    # доменные типы
  config/                    # ENV + YAML
  logger/                    # slog JSON
migrations/                  # SQL миграции (embed)
test_migrations/             # сиды для integration-тестов
configs/                     # YAML конфиг (reaper)
tests/e2e/                   # e2e-тесты
testdata/                    # фикстуры (log.zip, битые логи)
docs/                        # диаграммы + OpenAPI + Postman collection
```
