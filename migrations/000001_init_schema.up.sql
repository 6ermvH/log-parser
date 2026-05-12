CREATE TABLE logs (
    id          uuid        PRIMARY KEY,
    status      text        NOT NULL CHECK (status IN ('processing', 'ok', 'failed')),
    uploaded_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE failed_logs (
    log_id        uuid PRIMARY KEY REFERENCES logs(id) ON DELETE CASCADE,
    error_message text NOT NULL
);

CREATE TABLE nodes (
    id                bigserial PRIMARY KEY,
    log_id            uuid      NOT NULL REFERENCES logs(id) ON DELETE CASCADE,
    node_guid         text      NOT NULL,
    node_type         text      NOT NULL,
    node_desc         text,
    system_image_guid text,
    port_guid         text,
    UNIQUE (log_id, node_guid)
);

CREATE TABLE ports (
    id              bigserial PRIMARY KEY,
    node_id         bigint    NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    port_num        int       NOT NULL,
    port_guid       text,
    port_state      int,
    port_phy_state  int,
    link_speed_actv int,
    link_width_actv int,
    lid             int,
    raw             jsonb     NOT NULL,
    UNIQUE (node_id, port_num)
);

CREATE TABLE nodes_info (
    node_id     bigint PRIMARY KEY REFERENCES nodes(id) ON DELETE CASCADE,
    switch_info jsonb,
    system_info jsonb,
    sharp_info  jsonb
);
