create table account
(
    id          serial constraint account_pk primary key,
    org_id      integer,
    api_key     varchar(255),
    hook_key    varchar(255),
    channel_id  varchar(255) NOT NULL DEFAULT '',
    plain_id    varchar(255) NOT NULL DEFAULT '',
    status      varchar(255) NOT NULL DEFAULT '',
    is_disabled boolean
);
create index account_org_id_idx on account (org_id);
create index account_hook_key_idx on account (hook_key);
create index account_is_disabled_idx on account (is_disabled) where (NOT is_disabled);

create table message
(
    id          varchar(255) primary key,
    org_id      integer      NOT NULL,
    external_id varchar(255) NOT NULL DEFAULT '',
    created_at  timestamptz  NOT NULL,
    error       text         NOT NULL DEFAULT '',
    events      jsonb
);
create index message_external_id_idx on message (external_id);