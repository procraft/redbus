package repository

import (
	"context"

	"github.com/sergiusd/redbus/internal/app/model"
	"github.com/sergiusd/redbus/internal/pkg/db"
)

type Repository struct{}

func New() *Repository {
	return &Repository{}
}

func (r *Repository) Insert(ctx context.Context, repeat model.Repeat) error {
	conn := db.FromContext(ctx)
	return conn.QueryRow(ctx, `INSERT INTO repeat 
		(topic, "group", consumer_id, message_id, key, data, error)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		repeat.Topic, repeat.Group, repeat.ConsumerId, repeat.MessageId, repeat.Key, repeat.Data, repeat.Error).
		Scan(&repeat.Id)
}

func (r *Repository) FindForRepeat(ctx context.Context, topicGroupList model.TopicGroupList) (model.RepeatList, error) {
	return nil, nil
}
