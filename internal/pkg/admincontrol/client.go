package admincontrol

import (
	"context"
	"time"

	controlpb "github.com/prokraft/redbus/internal/api/admincontrol"
	"github.com/prokraft/redbus/internal/app/model"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type Client struct {
	connection *grpc.ClientConn
	control    controlpb.AdminControlServiceClient
	health     grpc_health_v1.HealthClient
	timeout    time.Duration
}

func New(address string, timeout time.Duration) (*Client, error) {
	connection, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{
		connection: connection,
		control:    controlpb.NewAdminControlServiceClient(connection),
		health:     grpc_health_v1.NewHealthClient(connection),
		timeout:    timeout,
	}, nil
}

func (c *Client) Close() error {
	return c.connection.Close()
}

func (c *Client) Health(ctx context.Context) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	_, err := c.health.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	return err
}

func (c *Client) GetStateSnapshot(ctx context.Context) (model.Stat, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	response, err := c.control.GetStateSnapshot(ctx, &controlpb.Empty{})
	if err != nil {
		return model.Stat{}, err
	}
	return model.Stat{
		ConsumeTopicCount: int(response.GetConsumeTopicCount()),
		ConsumerCount:     int(response.GetConsumerCount()),
		RepeatAllCount:    int(response.GetRepeatAllCount()),
		RepeatFailedCount: int(response.GetRepeatFailedCount()),
	}, nil
}

func (c *Client) GetTopicStats(ctx context.Context) (model.StatTopicList, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	response, err := c.control.GetTopicStats(ctx, &controlpb.Empty{})
	if err != nil {
		return nil, err
	}

	result := make(model.StatTopicList, 0, len(response.GetList()))
	for _, topic := range response.GetList() {
		partitions := make([]model.StatPartition, 0, len(topic.GetPartitions()))
		for _, partition := range topic.GetPartitions() {
			partitions = append(partitions, model.StatPartition{
				N:           model.PartitionN(partition.GetNumber()),
				FirstOffset: model.Offset(partition.GetFirstOffset()),
				LastOffset:  model.Offset(partition.GetLastOffset()),
			})
		}

		groups := make([]model.StatGroup, 0, len(topic.GetGroups()))
		for _, group := range topic.GetGroups() {
			consumers := make([]model.StatConsumer, 0, len(group.GetConsumers()))
			for _, consumer := range group.GetConsumers() {
				consumers = append(consumers, consumerFromProto(consumer))
			}
			groupPartitions := make([]model.StatGroupPartition, 0, len(group.GetPartitions()))
			for _, partition := range group.GetPartitions() {
				groupPartitions = append(groupPartitions, model.StatGroupPartition{
					N:             model.PartitionN(partition.GetNumber()),
					Offset:        model.Offset(partition.GetOffset()),
					ConsumerId:    model.ConsumerId(partition.GetConsumerId()),
					ConsumerState: partition.GetConsumerState(),
					FirstOffset:   model.Offset(partition.GetFirstOffset()),
					LastOffset:    model.Offset(partition.GetLastOffset()),
					Lag:           model.Offset(partition.GetLag()),
					Committed:     partition.GetCommitted(),
				})
			}
			groups = append(groups, model.StatGroup{
				Name:          model.GroupName(group.GetName()),
				KafkaGroupId:  group.GetKafkaGroupId(),
				State:         group.GetState(),
				Error:         group.GetError(),
				PartitionList: groupPartitions,
				ConsumerList:  consumers,
			})
		}

		result = append(result, model.StatTopic{
			Name:          model.TopicName(topic.GetName()),
			PartitionList: partitions,
			GroupList:     groups,
		})
	}
	return result, nil
}

func (c *Client) GetConsumerStats(ctx context.Context) (model.StatConsumerList, error) {
	topics, err := c.GetTopicStats(ctx)
	if err != nil {
		return nil, err
	}
	consumers := make(model.StatConsumerList, 0)
	for _, topic := range topics {
		for _, group := range topic.GroupList {
			consumers = append(consumers, group.ConsumerList...)
		}
	}
	return consumers, nil
}

func consumerFromProto(consumer *controlpb.Consumer) model.StatConsumer {
	partitions := make([]model.StatConsumerPartition, 0, len(consumer.GetPartitions()))
	for _, partition := range consumer.GetPartitions() {
		partitions = append(partitions, model.StatConsumerPartition{
			N:           model.PartitionN(partition.GetNumber()),
			GroupOffset: model.Offset(partition.GetGroupOffset()),
			LastOffset:  model.Offset(partition.GetLastOffset()),
			Lag:         model.Offset(partition.GetLag()),
			Committed:   partition.GetCommitted(),
		})
	}
	var lastMessageAt *time.Time
	if consumer.GetLastMessageAtUnixMs() > 0 {
		value := time.UnixMilli(consumer.GetLastMessageAtUnixMs())
		lastMessageAt = &value
	}
	return model.StatConsumer{
		Id:                model.ConsumerId(consumer.GetId()),
		Topic:             model.TopicName(consumer.GetTopic()),
		Group:             model.GroupName(consumer.GetGroup()),
		State:             consumer.GetState(),
		ConnectedAt:       timeFromUnixMilli(consumer.GetConnectedAtUnixMs()),
		StateSince:        timeFromUnixMilli(consumer.GetStateSinceUnixMs()),
		LastMessageAt:     lastMessageAt,
		MessagesProcessed: consumer.GetMessagesProcessed(),
		ReconnectCount:    consumer.GetReconnectCount(),
		LastError:         consumer.GetLastError(),
		RepeatStrategy:    consumer.GetRepeatStrategy(),
		KafkaMemberId:     consumer.GetKafkaMemberId(),
		ClientHost:        consumer.GetClientHost(),
		PartitionList:     partitions,
	}
}

func timeFromUnixMilli(value int64) time.Time {
	if value == 0 {
		return time.Time{}
	}
	return time.UnixMilli(value)
}

func (c *Client) GetRetryStats(ctx context.Context) (model.RepeatStat, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	response, err := c.control.GetRetryStats(ctx, &controlpb.Empty{})
	if err != nil {
		return nil, err
	}

	result := make(model.RepeatStat, 0, len(response.GetList()))
	for _, item := range response.GetList() {
		errors := make([]model.RepeatErrorStat, 0, len(item.GetErrors()))
		for _, errorStat := range item.GetErrors() {
			errors = append(errors, model.RepeatErrorStat{
				Error:         errorStat.GetError(),
				FailedCount:   int(errorStat.GetFailedCount()),
				FirstFailedAt: timeFromUnixMilli(errorStat.GetFirstFailedAtUnixMs()),
				LastFailedAt:  timeFromUnixMilli(errorStat.GetLastFailedAtUnixMs()),
			})
		}
		result = append(result, model.RepeatStatItem{
			Topic:       item.GetTopic(),
			Group:       item.GetGroup(),
			AllCount:    int(item.GetAllCount()),
			FailedCount: int(item.GetFailedCount()),
			LastError:   item.GetLastError(),
			Errors:      errors,
		})
	}
	return result, nil
}

func (c *Client) RestartFailed(ctx context.Context, topic, group string) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	_, err := c.control.RestartFailed(ctx, &controlpb.RestartFailedRequest{Topic: topic, Group: group})
	return err
}

func (c *Client) RestartFailedSince(ctx context.Context, topic, group string, lookbackSeconds int64) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	_, err := c.control.RestartFailedSince(ctx, &controlpb.RestartFailedSinceRequest{
		Topic:           topic,
		Group:           group,
		LookbackSeconds: lookbackSeconds,
	})
	return err
}

func (c *Client) RestartFailedByError(ctx context.Context, topic, group, errorMessage string, lookbackSeconds int64) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	_, err := c.control.RestartFailedByError(ctx, &controlpb.RestartFailedByErrorRequest{
		Topic:           topic,
		Group:           group,
		Error:           errorMessage,
		LookbackSeconds: lookbackSeconds,
	})
	return err
}

func (c *Client) DeleteFailedByError(ctx context.Context, topic, group, errorMessage string) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	_, err := c.control.DeleteFailedByError(ctx, &controlpb.DeleteFailedByErrorRequest{
		Topic: topic,
		Group: group,
		Error: errorMessage,
	})
	return err
}

func (c *Client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, c.timeout)
}
