package repository

import (
	"context"
	"fmt"

	"github.com/procraft/redbus/internal/app/model"
	"github.com/procraft/redbus/internal/pkg/db"
)

// This replaces the existing metrics COUNT scan, not an additional query per
// group. Only scalar columns are read; payloads and retry error text are not.
const retryQueueStatsSQL = `SELECT topic, "group", COUNT(*),
 COUNT(*) FILTER (WHERE finished_at IS NOT NULL),
 COUNT(*) FILTER (WHERE finished_at IS NULL AND deferred),
 MIN(created_at) FILTER (WHERE finished_at IS NULL)
 FROM repeat GROUP BY topic, "group"`

func (r *Repository) GetQueueStats(ctx context.Context) ([]model.RetryQueueStat, error) {
	rows, err := db.FromContext(ctx).Query(ctx, retryQueueStatsSQL)
	if err != nil {
		return nil, fmt.Errorf("get retry queue stats: %w", err)
	}
	defer rows.Close()
	var result []model.RetryQueueStat
	for rows.Next() {
		var stat model.RetryQueueStat
		if err := rows.Scan(&stat.Topic, &stat.Group, &stat.All, &stat.Failed, &stat.Deferred, &stat.Oldest); err != nil {
			return nil, err
		}
		result = append(result, stat)
	}
	return result, rows.Err()
}
