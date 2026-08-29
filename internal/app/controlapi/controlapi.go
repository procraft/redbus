package controlapi

import (
	"context"

	"github.com/prokraft/redbus/internal/api/admincontrol"
	"github.com/prokraft/redbus/internal/app/model"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type IDataBusService interface {
	GetStat(ctx context.Context) (model.Stat, error)
	GetTopicList(ctx context.Context) (model.StatTopicList, error)
}

type IRepeater interface {
	GetStat(ctx context.Context) (model.RepeatStat, error)
	RestartFailed(ctx context.Context, topic, group string) error
}

type ControlApi struct {
	dataBus  IDataBusService
	repeater IRepeater
}

func New(dataBus IDataBusService, repeater IRepeater) *ControlApi {
	return &ControlApi{dataBus: dataBus, repeater: repeater}
}

func (a *ControlApi) GetStateSnapshot(ctx context.Context, _ *admincontrol.Empty) (*admincontrol.StateSnapshot, error) {
	stat, err := a.dataBus.GetStat(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get state snapshot: %v", err)
	}
	return &admincontrol.StateSnapshot{
		ConsumeTopicCount: int32(stat.ConsumeTopicCount),
		ConsumerCount:     int32(stat.ConsumerCount),
		RepeatAllCount:    int32(stat.RepeatAllCount),
		RepeatFailedCount: int32(stat.RepeatFailedCount),
	}, nil
}

func (a *ControlApi) GetTopicStats(ctx context.Context, _ *admincontrol.Empty) (*admincontrol.TopicStats, error) {
	list, err := a.dataBus.GetTopicList(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get topic stats: %v", err)
	}

	result := make([]*admincontrol.Topic, 0, len(list))
	for _, topic := range list {
		partitions := make([]*admincontrol.Partition, 0, len(topic.PartitionList))
		for _, partition := range topic.PartitionList {
			partitions = append(partitions, &admincontrol.Partition{
				Number:      int32(partition.N),
				FirstOffset: int64(partition.FirstOffset),
				LastOffset:  int64(partition.LastOffset),
			})
		}

		groups := make([]*admincontrol.Group, 0, len(topic.GroupList))
		for _, group := range topic.GroupList {
			groupPartitions := make([]*admincontrol.GroupPartition, 0, len(group.PartitionList))
			for _, partition := range group.PartitionList {
				groupPartitions = append(groupPartitions, &admincontrol.GroupPartition{
					Number:        int32(partition.N),
					Offset:        int64(partition.Offset),
					ConsumerId:    string(partition.ConsumerId),
					ConsumerState: partition.ConsumerState,
				})
			}
			groups = append(groups, &admincontrol.Group{
				Name:       string(group.Name),
				Partitions: groupPartitions,
			})
		}

		result = append(result, &admincontrol.Topic{
			Name:       string(topic.Name),
			Partitions: partitions,
			Groups:     groups,
		})
	}
	return &admincontrol.TopicStats{List: result}, nil
}

func (a *ControlApi) GetRetryStats(ctx context.Context, _ *admincontrol.Empty) (*admincontrol.RetryStats, error) {
	stat, err := a.repeater.GetStat(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get retry stats: %v", err)
	}

	result := make([]*admincontrol.RetryStat, 0, len(stat))
	for _, item := range stat {
		result = append(result, &admincontrol.RetryStat{
			Topic:       item.Topic,
			Group:       item.Group,
			AllCount:    int32(item.AllCount),
			FailedCount: int32(item.FailedCount),
			LastError:   item.LastError,
		})
	}
	return &admincontrol.RetryStats{List: result}, nil
}

func (a *ControlApi) RestartFailed(ctx context.Context, req *admincontrol.RestartFailedRequest) (*admincontrol.Empty, error) {
	if req.GetTopic() == "" || req.GetGroup() == "" {
		return nil, status.Error(codes.InvalidArgument, "topic and group are required")
	}
	if err := a.repeater.RestartFailed(ctx, req.GetTopic(), req.GetGroup()); err != nil {
		return nil, status.Errorf(codes.Internal, "restart failed retries: %v", err)
	}
	return &admincontrol.Empty{}, nil
}
