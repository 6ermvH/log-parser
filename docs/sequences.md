# Log Parser — Sequence Diagrams

## POST /api/v1/parse

```mermaid
sequenceDiagram
    autonumber
    participant C as Client
    participant H as Handler
    participant P as ParserService
    participant FS as Logs Volume
    participant A as Aggregator
    participant R as Repository
    participant DB as PostgreSQL

    C->>H: POST /api/v1/parse {path}
    H->>P: ParseLog(ctx, path)

    P->>R: InsertLog(id, status processing)
    R->>DB: INSERT into logs (commit)
    DB-->>R: ok
    R-->>P: log_id

    P->>FS: open zip
    FS-->>P: zip entries

    loop for each file in archive
        P->>A: AnalyzeFile(name, reader)
        Note over A: internal state machine<br/>START_X / Header / Body / END_X<br/>emits events via callback
        A-->>P: nil or error
    end

    alt parse succeeded
        P->>A: Result()
        A-->>P: domain.Log
        P->>R: SaveDomain + UpdateStatus ok (single tx)
        R->>DB: tx — insert nodes/ports/nodes_info, update logs
        DB-->>R: ok
        R-->>P: ok
        P-->>H: log_id
        H-->>C: 201 {log_id}
    else parse failed
        P->>R: MarkFailed(log_id, error) (single tx)
        R->>DB: tx — update logs status failed, insert failed_logs
        DB-->>R: ok
        R-->>P: ok
        P-->>H: error
        H-->>C: 400 {log_id, error}
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

        H->>R: ListConnections(log_id)
        R->>DB: select connections for log
        DB-->>R: edges[]
        R-->>H: edges[]

        H-->>C: 200 {nodes, ports, edges}
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
