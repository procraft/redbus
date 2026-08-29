CREATE INDEX repeat_pending_topic_group_started_at_idx
    ON repeat ((topic || '|' || "group"), started_at)
    WHERE finished_at IS NULL;

CREATE INDEX repeat_failed_topic_group_idx
    ON repeat (topic, "group")
    WHERE finished_at IS NOT NULL;

DROP INDEX repeat_started_at_idx;
DROP INDEX repeat_finished_at_idx;
