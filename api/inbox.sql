# --- !Ups
create table public.redbus_inbox
(
    "key"             VARCHAR NOT NULL PRIMARY KEY,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX redbus_inbox__created_at_idx ON public.redbus_inbox(created_at);

# --- !Downs
DROP TABLE public.redbus_inbox;