package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/prokraft/redbus/internal/app/model"
	"github.com/prokraft/redbus/internal/pkg/db"
	"github.com/prokraft/redbus/internal/pkg/runtime"
)

const repeatFields = `id, topic, "group", consumer_id, message_id, key, data, headers, attempt, repeat_strategy, error, created_at, started_at, finished_at`

func repeatScanDest(r *model.Repeat) []any {
	return []any{
		&r.Id, &r.Topic, &r.Group, &r.ConsumerId, &r.MessageId, &r.Key, &r.Data,
		&r.Headers, &r.Attempt, &r.Strategy, &r.Error, &r.CreatedAt, &r.StartedAt, &r.FinishedAt,
	}
}

func (r *Repository) Insert(ctx context.Context, repeat model.Repeat) error {
	conn := db.FromContext(ctx)
	return conn.QueryRow(ctx, `INSERT INTO repeat
		(topic, "group", consumer_id, message_id, key, data, headers, error, attempt, repeat_strategy, created_at, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id`,
		repeat.Topic, repeat.Group, repeat.ConsumerId, repeat.MessageId, repeat.Key, repeat.Data, repeat.Headers,
		repeat.Error, repeat.Attempt, repeat.Strategy, repeat.CreatedAt, repeat.StartedAt,
	).Scan(&repeat.Id)
}

func (r *Repository) FindForRepeat(ctx context.Context, topicGroupList model.TopicGroupList) (model.RepeatList, error) {
	conn := db.FromContext(ctx)
	if len(topicGroupList) == 0 {
		return nil, nil
	}
	sql := "SELECT " + repeatFields + " FROM repeat " +
		"WHERE finished_at IS NULL AND started_at <= $2 AND (topic || '|' || \"group\") = any($1)"
	rows, err := conn.Query(ctx, sql, topicGroupList.String("|"), runtime.Now())
	if err != nil {
		return nil, fmt.Errorf("Can't get repeat list from db: %w", err)
	}
	defer rows.Close()

	ret := make(model.RepeatList, 0)
	for rows.Next() {
		r := model.Repeat{}
		err := rows.Scan(repeatScanDest(&r)...)
		if err != nil {
			return nil, fmt.Errorf("Can't scan on get repeat list from db: %w", err)
		}
		ret = append(ret, &r)
	}
	return ret, nil
}

func (r *Repository) Delete(ctx context.Context, repeatId int64) error {
	conn := db.FromContext(ctx)
	if _, err := conn.Exec(ctx, `DELETE FROM repeat WHERE id = $1`, repeatId); err != nil {
		return fmt.Errorf("Can't delete repeat: %w", err)
	}
	return nil
}

func (r *Repository) UpdateAttempt(ctx context.Context, repeat *model.Repeat) error {
	conn := db.FromContext(ctx)
	_, err := conn.Exec(ctx, `UPDATE repeat
		SET started_at = $1, attempt = $2, error = $3, finished_at = $4
		WHERE id = $5`,
		repeat.StartedAt, repeat.Attempt, repeat.Error, repeat.FinishedAt, repeat.Id)
	return err
}

func (r *Repository) GetCount(ctx context.Context) (int, int, error) {
	conn := db.FromContext(ctx)
	allCount, failedCount := 0, 0
	sql := `SELECT
    	COUNT(*) as all_count,
		COALESCE(SUM(CASE WHEN finished_at IS NULL THEN 0 ELSE 1 END), 0) AS failed_count
	FROM repeat`
	err := conn.QueryRow(ctx, sql).Scan(&allCount, &failedCount)
	if err != nil {
		return 0, 0, fmt.Errorf("Can't get all repeat count from db: %w", err)
	}
	return allCount, failedCount, nil
}

func (r *Repository) GetStat(ctx context.Context) (model.RepeatStat, error) {
	conn := db.FromContext(ctx)
	sql := `WITH group_stats AS (
		SELECT
			topic,
			"group",
			COUNT(*) AS all_count,
			COUNT(*) FILTER (WHERE finished_at IS NOT NULL) AS failed_count,
			(ARRAY_AGG(error ORDER BY (finished_at IS NOT NULL) DESC, started_at DESC, id DESC))[1] AS last_error
		FROM repeat
		GROUP BY topic, "group"
	), error_stats AS (
		SELECT
			topic,
			"group",
			error,
			COUNT(*) AS failed_count,
			MIN(finished_at) AS first_failed_at,
			MAX(finished_at) AS last_failed_at
		FROM repeat
		WHERE finished_at IS NOT NULL
		GROUP BY topic, "group", error
	)
	SELECT
		group_stats.topic,
		group_stats."group",
		group_stats.last_error,
		group_stats.all_count,
		group_stats.failed_count,
		error_stats.error,
		COALESCE(error_stats.failed_count, 0),
		error_stats.first_failed_at,
		error_stats.last_failed_at
	FROM group_stats
	LEFT JOIN error_stats USING (topic, "group")
	ORDER BY
		group_stats.topic,
		group_stats."group",
		error_stats.failed_count DESC,
		error_stats.error`
	rows, err := conn.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("Can't get repeat stat from db: %w", err)
	}
	defer rows.Close()

	ret := make(model.RepeatStat, 0)
	for rows.Next() {
		item := model.RepeatStatItem{}
		var errorMessage *string
		var errorFailedCount int
		var firstFailedAt *time.Time
		var lastFailedAt *time.Time
		err := rows.Scan(
			&item.Topic,
			&item.Group,
			&item.LastError,
			&item.AllCount,
			&item.FailedCount,
			&errorMessage,
			&errorFailedCount,
			&firstFailedAt,
			&lastFailedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("Can't scan on get repeat stat from db: %w", err)
		}

		if len(ret) == 0 || ret[len(ret)-1].Topic != item.Topic || ret[len(ret)-1].Group != item.Group {
			item.Errors = make([]model.RepeatErrorStat, 0)
			ret = append(ret, item)
		}
		if errorMessage != nil {
			errorStat := model.RepeatErrorStat{
				Error:       *errorMessage,
				FailedCount: errorFailedCount,
			}
			if firstFailedAt != nil {
				errorStat.FirstFailedAt = *firstFailedAt
			}
			if lastFailedAt != nil {
				errorStat.LastFailedAt = *lastFailedAt
			}
			ret[len(ret)-1].Errors = append(ret[len(ret)-1].Errors, errorStat)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("Can't iterate repeat stat from db: %w", err)
	}
	return ret, nil
}

func (r *Repository) RestartFailed(ctx context.Context, topic, group string) error {
	conn := db.FromContext(ctx)
	_, err := conn.Exec(ctx, `UPDATE repeat
		SET started_at = $1, attempt = 0, error = '', finished_at = null
		WHERE finished_at IS NOT NULL AND topic = $2 AND "group" = $3`, runtime.Now(), topic, group)
	return err
}

func (r *Repository) RestartFailedSince(ctx context.Context, topic, group string, since time.Time) error {
	conn := db.FromContext(ctx)
	_, err := conn.Exec(ctx, `UPDATE repeat
		SET started_at = $1, attempt = 0, error = '', finished_at = null
		WHERE finished_at IS NOT NULL AND topic = $2 AND "group" = $3 AND finished_at >= $4`,
		runtime.Now(), topic, group, since)
	return err
}

func (r *Repository) RestartFailedByError(ctx context.Context, topic, group, errorMessage string, since time.Time) error {
	conn := db.FromContext(ctx)
	_, err := conn.Exec(ctx, `UPDATE repeat
		SET started_at = $1, attempt = 0, error = '', finished_at = null
		WHERE finished_at IS NOT NULL AND topic = $2 AND "group" = $3 AND error = $4 AND finished_at >= $5`,
		runtime.Now(), topic, group, errorMessage, since)
	return err
}

func (r *Repository) DeleteFailedByError(ctx context.Context, topic, group, errorMessage string) error {
	conn := db.FromContext(ctx)
	_, err := conn.Exec(ctx, `DELETE FROM repeat
		WHERE finished_at IS NOT NULL AND topic = $1 AND "group" = $2 AND error = $3`,
		topic, group, errorMessage)
	return err
}
