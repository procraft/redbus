export type DashboardStat = {
  consumerCount: number;
  consumeTopicCount: number;
  repeatAllCount: number;
  repeatFailedCount: number;
};

export type ConsumerState = 'connecting' | 'connected' | 'reconnecting' | string;

export type TopicPartition = {
  n: number;
  firstOffset: number;
  lastOffset: number;
};

export type TopicGroupPartition = {
  n: number;
  offset: number;
  consumerId: string;
  consumerState: ConsumerState;
};

export type TopicGroup = {
  name: string;
  partitions: TopicGroupPartition[] | null;
};

export type TopicStat = {
  name: string;
  partitions: TopicPartition[] | null;
  groups: TopicGroup[] | null;
};

export type RepeatStat = {
  topic: string;
  group: string;
  allCount: number;
  failedCount: number;
  lastError: string;
};

export type ConsumersEvent = Pick<DashboardStat, 'consumerCount' | 'consumeTopicCount'>;
export type RepeaterEvent = {
  allCount: number;
  failedCount: number;
};
