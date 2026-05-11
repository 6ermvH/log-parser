# Log Parser — Components (C3)

```mermaid
C4Component
    title Log Parser — Components (C3)

    Person(admin, "HPC Administrator", "Загружает логи и читает топологию")
    SystemDb_Ext(db, "PostgreSQL", "logs, nodes, ports, nodes_info, connections")
    System_Ext(fs, "Logs Volume", "/app/data — входные log.zip")

    Container_Boundary(api, "API Service (Go)") {
        Component(router, "HTTP Router", "net/http", "Маршрутизация /api/v1/*, middleware (recover, request log)")
        Component(handlers, "Handlers", "net/http handlers", "Разбор запроса, валидация, JSON-ответ")
        Component(parserSvc, "Parser Service", "Go package", "Открыть zip, скормить файлы агрегатору, передать результат в Repository")

        Component_Boundary(aggBoundary, "Topology Aggregator") {
            Component(aggFacade, "Aggregator (facade)", "Go", "AnalyzeFile(name, reader); Result() → domain.Log; накапливает между файлами")
            Component(sm, "Section State Machine", "Go (internal)", "START_X / Header / Body / END_X; вызывает aggregator.onEvent через callback")
        }

        Component(repo, "Repository", "database/sql + lib/pq", "CRUD + транзакционная запись. Схема — embed-миграции на старте")
    }

    Rel(admin, router, "HTTP", "JSON :8080")
    Rel(router, handlers, "dispatch")
    Rel(handlers, parserSvc, "POST /parse")
    Rel(handlers, repo, "GET /topology, /node, /port, /log")
    Rel(parserSvc, fs, "Открывает log.zip", "file I/O")
    Rel(parserSvc, aggFacade, "AnalyzeFile(name, reader)")
    Rel(aggFacade, sm, "создаёт на каждый файл")
    Rel(sm, aggFacade, "emit(section, columns, row)", "callback")
    Rel(parserSvc, repo, "SaveLog(domain.Log)", "в одной tx")
    Rel(repo, db, "SQL")
```
