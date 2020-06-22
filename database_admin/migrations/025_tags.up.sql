CREATE TABLE tag
(
    id        INT GENERATED BY DEFAULT AS IDENTITY,
    namespace TEXT NOT NULL CHECK (NOT EMPTY(namespace)),
    name      TEXT NOT NULL CHECK ( NOT EMPTY(name)),
    PRIMARY KEY (id)
);

CREATE INDEX tags_idx on tag (namespace, name);

GRANT SELECT, INSERT, UPDATE, DELETE ON tag to listener;
GRANT SELECT ON tag to evaluator;
GRANT SELECT ON tag to vmaas_sync;
GRANT SELECT ON tag to manager;

GRANT SELECT,USAGE ON SEQUENCE public.tag_id_seq TO evaluator;
GRANT SELECT,USAGE ON SEQUENCE public.tag_id_seq TO listener;
GRANT SELECT,USAGE ON SEQUENCE public.tag_id_seq TO vmaas_sync;


CREATE TABLE system_tag
(
    tag_id    INT  NOT NULL REFERENCES tag,
    system_id INT  NOT NULL REFERENCES system_platform,
    tag_value TEXT NOT NULL,
    PRIMARY KEY (tag_id, system_id)
);

CREATE INDEX system_tags_idx on system_tag (system_id, tag_id, tag_value);

GRANT SELECT, INSERT, UPDATE, DELETE ON system_tag to listener;
GRANT SELECT ON system_tag to evaluator;
GRANT SELECT ON system_tag to vmaas_sync;
GRANT SELECT ON system_tag to manager;