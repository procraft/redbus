create table repeat
(
    id          bigserial constraint repeat_pk primary key,
    topic       varchar(255) NOT NULL,
    "group"       varchar(255) NOT NULL,
    consumer_id varchar(255) NOT NULL,
    message_id  varchar(255) NOT NULL,
    key         bytea NOT NULL,
    data        bytea NOT NULL,
    attempt     int NOT NULL DEFAULT 0,
    repeat_strategy jsonb,
    error       text NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at  TIMESTAMPTZ NOT NULL,
    finished_at  TIMESTAMPTZ
);
create index repeat_started_at_idx on repeat (started_at);
create index repeat_finished_at_idx on repeat (finished_at) WHERE finished_at IS NOT NULL;