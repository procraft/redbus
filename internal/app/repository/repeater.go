package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sergiusd/redbus/internal/app/model"
	"github.com/sergiusd/redbus/internal/pkg/db"
	"github.com/sergiusd/redbus/internal/pkg/runtime"
)

const repeatFields = `id, topic, "group", consumer_id, message_id, key, data, attempt, repeat_strategy, error, created_at, started_at, finished_at`

func (r *Repository) Insert(ctx context.Context, repeat model.Repeat) error {
	b, _ := json.Marshal(repeat.Strategy)
	fmt.Printf("%v\n", b)
	conn := db.FromContext(ctx)
	return conn.QueryRow(ctx, `INSERT INTO repeat 
		(topic, "group", consumer_id, message_id, key, data, error, attempt, repeat_strategy, created_at, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id`,
		repeat.Topic, repeat.Group, repeat.ConsumerId, repeat.MessageId, repeat.Key, repeat.Data, repeat.Error,
		repeat.Attempt, repeat.Strategy, repeat.CreatedAt, repeat.StartedAt,
	).Scan(&repeat.Id)
}

func (r *Repository) FindForRepeat(ctx context.Context, topicGroupList model.TopicGroupList) (model.RepeatList, error) {
	conn := db.FromContext(ctx)
	if len(topicGroupList) == 0 {
		return nil, nil
	}
	sql := "SELECT " + repeatFields + " FROM repeat " +
		"WHERE finished_at IS NULL AND started_at <= $2 AND (topic || '|' || \"group\") = any($1)"
	rows, err := conn.Query(ctx, sql, topicGroupList.GetStrList("|"), runtime.Now())
	if err != nil {
		return nil, fmt.Errorf("Can't get repeat list from db: %w", err)
	}
	defer rows.Close()

	ret := make(model.RepeatList, 0)
	for rows.Next() {
		r := model.Repeat{}
		err := rows.Scan(
			&r.Id, &r.Topic, &r.Group, &r.ConsumerId, &r.MessageId, &r.Key, &r.Data,
			&r.Attempt, &r.Strategy, &r.Error, &r.CreatedAt, &r.StartedAt, &r.FinishedAt,
		)
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
		SUM(CASE WHEN finished_at IS NULL THEN 0 ELSE 1 END) AS failed_count
	FROM repeat`
	err := conn.QueryRow(ctx, sql).Scan(&allCount, &failedCount)
	if err != nil {
		return 0, 0, fmt.Errorf("Can't get all repeat count from db: %w", err)
	}
	return allCount, failedCount, nil
}

func (r *Repository) GetStat(ctx context.Context) (model.RepeatStat, error) {
	conn := db.FromContext(ctx)
	sql := `WITH groups AS (
		SELECT
			id, topic, "group", finished_at,
			(first_value(error) OVER (PARTITION BY topic, "group" ORDER BY started_at DESC))::text AS last_error
		FROM repeat
	)
	SELECT 
		topic, "group", last_error,
		COUNT(id) AS all_count,
		SUM(CASE WHEN finished_at IS NULL THEN 0 ELSE 1 END) AS failed_count
	FROM groups
	GROUP BY topic, "group", last_error`
	rows, err := conn.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("Can't get repeat stat from db: %w", err)
	}
	defer rows.Close()

	ret := make(model.RepeatStat, 0)
	for rows.Next() {
		r := model.RepeatStatItem{}
		err := rows.Scan(&r.Topic, &r.Group, &r.LastError, &r.AllCount, &r.FailedCount)
		if err != nil {
			return nil, fmt.Errorf("Can't scan on get repeat stat from db: %w", err)
		}
		ret = append(ret, r)
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
