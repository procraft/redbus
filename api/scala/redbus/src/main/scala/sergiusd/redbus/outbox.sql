# --- !Ups
CREATE TABLE public.redbus_outbox (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    topic VARCHAR NOT NULL,
    message BYTEA NOT NULL,
    options JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION redbus_notify() RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('redbus_outbox', NEW.id::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER redbus_notify_trigger
    AFTER INSERT ON redbus_outbox
    FOR EACH ROW
    EXECUTE FUNCTION redbus_notify();

# --- !Downs
DROP TRIGGER IF EXISTS redbus_notify_trigger ON public.redbus_outbox;
DROP FUNCTION IF EXISTS redbus_notify();
DROP TABLE IF EXISTS public.redbus_outbox;