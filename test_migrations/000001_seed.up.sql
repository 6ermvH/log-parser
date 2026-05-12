INSERT INTO logs (id, status) VALUES
    ('11111111-1111-1111-1111-111111111111', 'ok');

INSERT INTO nodes (log_id, node_guid, node_type, node_desc, system_image_guid, port_guid) VALUES
    ('11111111-1111-1111-1111-111111111111', '0xseed_host',   'host',   'SEED_HOST',   '0xseed_host',   '0xseed_host'),
    ('11111111-1111-1111-1111-111111111111', '0xseed_switch', 'switch', 'SEED_SWITCH', '0xseed_switch', '0xseed_switch');

INSERT INTO ports (node_id, port_num, port_guid, port_state, port_phy_state, link_speed_actv, link_width_actv, lid, raw)
SELECT n.id, 1, n.node_guid, 4, 5, 2048, 2, 1, '{}'::jsonb
FROM nodes n
WHERE n.log_id = '11111111-1111-1111-1111-111111111111';

INSERT INTO nodes_info (node_id, switch_info, system_info)
SELECT n.id, '{"LinearFDBCap":"49152"}'::jsonb, '{"SerialNumber":"SEED01","ProductName":"Gorilla"}'::jsonb
FROM nodes n
WHERE n.log_id = '11111111-1111-1111-1111-111111111111' AND n.node_type = 'switch';
