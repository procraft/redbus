-- A pending retry the consumer postponed (retryLater): waiting for its turn, not failing.
-- Existing rows keep false; the flag is set on their next deferred delivery.
alter table repeat add column deferred boolean not null default false;
