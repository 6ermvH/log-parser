CREATE TABLE connections (
    id        bigserial PRIMARY KEY,
    log_id    uuid   NOT NULL REFERENCES logs(id) ON DELETE CASCADE,
    port_a_id bigint NOT NULL REFERENCES ports(id) ON DELETE CASCADE,
    port_b_id bigint NOT NULL REFERENCES ports(id) ON DELETE CASCADE,
    UNIQUE (port_a_id, port_b_id),
    CHECK  (port_a_id < port_b_id)
);
