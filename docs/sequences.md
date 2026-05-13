# Log Parser — Sequence Diagrams

## POST /api/v1/parse (async)

Парсинг асинхронный — клиент сразу получает `log_id`, обработка идёт в фоновой горутине. Статус узнавать через `GET /api/v1/log/{log_id}` (polling).

### Синхронная часть (HTTP-запрос)

```mermaid
sequenceDiagram
    autonumber
    participant C as Client
    participant H as Handler
    participant P as ParserService
    participant Pf as Parser.Preflight
    participant R as Repository
    participant DB as PostgreSQL
    participant G as background goroutine

    C->>H: POST /api/v1/parse {path}
    H->>P: Submit(ctx, path)

    P->>Pf: Preflight(path)
    Note over Pf: os.Stat + zip.OpenReader<br/>проверка «файл есть и это zip»
    Pf-->>P: nil or ErrInputNotFound / ErrInputNotZip

    alt preflight failed
        P-->>H: sentinel error
        H-->>C: 400 Bad Request {error}
    else preflight ok
        P->>R: InsertProcessingLog(uuid)
        R->>DB: INSERT logs (status processing) — commit
        DB-->>R: ok
        R-->>P: ok

        P->>G: spawn(process)
        P-->>H: log_id
        H-->>C: 202 Accepted {log_id}
    end
```

### Фоновая горутина

```mermaid
sequenceDiagram
    autonumber
    participant G as background goroutine
    participant FS as Logs Volume
    participant A as Aggregator
    participant R as Repository
    participant DB as PostgreSQL

    G->>FS: open zip
    FS-->>G: zip entries

    loop for each file in archive
        G->>A: AnalyzeFile(name, reader)
        Note over A: internal state machine<br/>START_X / Header / Body / END_X<br/>emits events via callback
        A-->>G: nil or error
    end

    alt parse succeeded
        G->>A: Result()
        A-->>G: domain.Log
        G->>R: SaveDomain + UpdateStatus ok (single tx)
        R->>DB: tx — insert nodes/ports/nodes_info, update logs
        DB-->>R: ok
    else parse failed
        G->>R: MarkFailed(log_id, error) (single tx)
        R->>DB: tx — update logs status failed, insert failed_logs
        DB-->>R: ok
    end
```

### Клиентский polling

```mermaid
sequenceDiagram
    autonumber
    participant C as Client
    participant H as Handler
    participant R as Repository
    participant DB as PostgreSQL

    loop until status != processing
        C->>H: GET /api/v1/log/{log_id}
        H->>R: GetLog(id)
        R->>DB: SELECT logs LEFT JOIN failed_logs
        DB-->>R: row
        R-->>H: status, error?
        H-->>C: 200 {status, error?, counts}
    end
```

---

## GET /api/v1/topology/{log_id}

```mermaid
sequenceDiagram
    autonumber
    participant C as Client
    participant H as Handler
    participant R as Repository
    participant DB as PostgreSQL

    C->>H: GET /api/v1/topology/{log_id}
    H->>R: FindLog(log_id)
    R->>DB: select log by id
    DB-->>R: row or empty

    alt log not found
        R-->>H: ErrNotFound
        H-->>C: 404 {error log not found}
    else log found
        R-->>H: log

        H->>R: ListNodes(log_id)
        R->>DB: select nodes for log
        DB-->>R: nodes[]
        R-->>H: nodes[]

        H->>R: ListPortsByLog(log_id)
        R->>DB: select ports joined with nodes for log
        DB-->>R: ports[]
        R-->>H: ports[]

        H-->>C: 200 {nodes, ports}
    end
```

---

## GET /api/v1/node/{node_id}

```mermaid
sequenceDiagram
    autonumber
    participant C as Client
    participant H as Handler
    participant R as Repository
    participant DB as PostgreSQL

    C->>H: GET /api/v1/node/{node_id}
    H->>R: FindNodeWithInfo(node_id)
    R->>DB: select node left join nodes_info
    DB-->>R: row or empty

    alt node not found
        R-->>H: ErrNotFound
        H-->>C: 404 {error node not found}
    else node found
        R-->>H: node + info blocks
        H-->>C: 200 {id, guid, type, desc, switch_info?, system_info?, sharp_info?}
    end
```

---

## GET /api/v1/port/{node_id}

```mermaid
sequenceDiagram
    autonumber
    participant C as Client
    participant H as Handler
    participant R as Repository
    participant DB as PostgreSQL

    C->>H: GET /api/v1/port/{node_id}
    H->>R: NodeExists(node_id)
    R->>DB: check node existence
    DB-->>R: bool

    alt node does not exist
        R-->>H: false
        H-->>C: 404 {error node not found}
    else node exists
        H->>R: ListPortsByNode(node_id)
        R->>DB: select ports for node ordered by port_num
        DB-->>R: ports[]
        R-->>H: ports[]
        H-->>C: 200 {ports}
    end
```

---

## GET /api/v1/log/{log_id}

```mermaid
sequenceDiagram
    autonumber
    participant C as Client
    participant H as Handler
    participant R as Repository
    participant DB as PostgreSQL

    C->>H: GET /api/v1/log/{log_id}
    H->>R: FindLogMeta(log_id)
    R->>DB: select log left join failed_logs
    DB-->>R: row or empty

    alt log not found
        R-->>H: ErrNotFound
        H-->>C: 404 {error log not found}
    else log found
        R-->>H: log + optional error

        H->>R: CountByLog(log_id)
        R->>DB: count nodes and ports for log
        DB-->>R: nodes_count, ports_count
        R-->>H: counts

        H-->>C: 200 {id, status, uploaded_at, nodes_count, ports_count, error?}
    end
```

---

## Reaper (background)

```mermaid
sequenceDiagram
    autonumber
    participant T as time.Ticker (30s)
    participant Rp as Reaper goroutine
    participant R as Repository
    participant DB as PostgreSQL

    loop while app is alive
        T->>Rp: tick
        Rp->>R: ReapStaleProcessing(ctx, 5m)
        R->>DB: tx — CTE update stale processing to failed, insert into failed_logs on conflict do nothing
        DB-->>R: reaped N rows
        R-->>Rp: N
        Note right of Rp: structured log<br/>reaped=N
    end
```
