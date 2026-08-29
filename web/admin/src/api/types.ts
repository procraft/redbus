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
  firstOffset: number;
  lastOffset: number;
  lag: number;
  committed: boolean;
  consumerId: string;
  consumerState: ConsumerState;
};

export type ConsumerPartition = {
  n: number;
  groupOffset: number;
  lastOffset: number;
  lag: number;
  committed: boolean;
};

export type ConsumerStat = {
  id: string;
  topic: string;
  group: string;
  state: ConsumerState;
  connectedAt: string;
  stateSince: string;
  lastMessageAt: string | null;
  messagesProcessed: number;
  reconnectCount: number;
  lastError: string;
  repeatStrategy: string;
  kafkaMemberId: string;
  clientHost: string;
  partitions: ConsumerPartition[] | null;
};

export type TopicGroup = {
  name: string;
  kafkaGroupId: string;
  state: string;
  error: string;
  partitions: TopicGroupPartition[] | null;
  consumers: ConsumerStat[] | null;
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
