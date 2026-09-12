import {
  ActionIcon,
  Badge,
  Center,
  Group,
  Loader,
  Paper,
  Stack,
  Text,
  TextInput,
  Title,
  Tooltip,
  UnstyledButton,
} from '@mantine/core';
import { useElementSize, useViewportSize } from '@mantine/hooks';
import { RefreshCw, Search } from 'lucide-react';
import type { CSSProperties } from 'react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router';

import dataBus from '@/api/dataBus';
import type { RepeatStat, TopicStat } from '@/api/types';
import { useRequest } from '@/hooks/useRequest';
import { averageRate, compactNumber, formatAge, formatDate, numberFormatter, validDate } from '@/utils/format';

/** Above this many failed repeats a topic is critical, below it is only a warning. */
const ERROR_LIMIT = 100;

const TILE_HEIGHT = 84;
const TILE_GAP = 8;
const MIN_TILE_WIDTH = 148;
const MAX_TILE_WIDTH = 260;
/** Page chrome above the grid: header, title, stat cards and the overview toolbar. */
const RESERVED_HEIGHT = 420;

type TopicStatus = 'critical' | 'warning' | 'active' | 'idle';

type TopicOverviewItem = {
  name: string;
  groupCount: number;
  consumerCount: number;
  totalLag: number;
  rate: number;
  lastMessageAt: string | null;
  errorCount: number;
  status: TopicStatus;
};

const statusColor: Record<TopicStatus, string> = {
  critical: 'red',
  warning: 'yellow',
  active: 'teal',
  idle: 'gray',
};

const statusLabel: Record<TopicStatus, string> = {
  critical: `More than ${ERROR_LIMIT} failed repeats`,
  warning: `Up to ${ERROR_LIMIT} failed repeats`,
  active: 'Consumers connected',
  idle: 'Nobody listens',
};

function buildOverview(topics: TopicStat[], repeats: RepeatStat[]): TopicOverviewItem[] {
  const errorsByTopic = new Map<string, number>();
  for (const repeat of repeats) {
    errorsByTopic.set(repeat.topic, (errorsByTopic.get(repeat.topic) ?? 0) + repeat.failedCount);
  }

  return topics
    .map((topic) => {
      const groups = topic.groups ?? [];
      const consumers = groups.flatMap((group) => group.consumers ?? []);
      const totalLag = groups.reduce(
        (total, group) =>
          total + (group.partitions ?? []).reduce((groupTotal, partition) => groupTotal + partition.lag, 0),
        0,
      );
      const rate = consumers.reduce(
        (total, consumer) => total + averageRate(consumer.messagesProcessed, consumer.connectedAt),
        0,
      );
      const lastMessageAt = consumers.reduce<string | null>((latest, consumer) => {
        const current = validDate(consumer.lastMessageAt);
        if (!current) return latest;
        const known = validDate(latest);
        return !known || current > known ? consumer.lastMessageAt : latest;
      }, null);
      const errorCount = errorsByTopic.get(topic.name) ?? 0;

      return {
        name: topic.name,
        groupCount: groups.length,
        consumerCount: consumers.length,
        totalLag,
        rate,
        lastMessageAt,
        errorCount,
        status:
          errorCount > ERROR_LIMIT
            ? 'critical'
            : errorCount > 0
              ? 'warning'
              : consumers.length > 0
                ? 'active'
                : 'idle',
      } satisfies TopicOverviewItem;
    })
    .sort(
      (left, right) =>
        right.errorCount - left.errorCount ||
        right.totalLag - left.totalLag ||
        left.name.localeCompare(right.name),
    );
}

/**
 * Picks a column count so that every tile fits on screen when possible, while keeping
 * tile width inside the readable range.
 */
function gridColumns(containerWidth: number, viewportHeight: number, topicCount: number): number {
  if (containerWidth <= 0 || topicCount === 0) return 1;
  const widest = Math.max(1, Math.floor((containerWidth + TILE_GAP) / (MIN_TILE_WIDTH + TILE_GAP)));
  const narrowest = Math.max(1, Math.floor((containerWidth + TILE_GAP) / (MAX_TILE_WIDTH + TILE_GAP)));
  const availableHeight = Math.max(240, viewportHeight - RESERVED_HEIGHT);
  const rows = Math.max(1, Math.floor((availableHeight + TILE_GAP) / (TILE_HEIGHT + TILE_GAP)));
  const needed = Math.ceil(topicCount / rows);
  return Math.min(widest, Math.max(narrowest, needed));
}

function TopicTile({ topic, onOpen }: { topic: TopicOverviewItem; onOpen: () => void }) {
  const color = statusColor[topic.status];
  const tooltip = (
    <Stack gap={2}>
      <Text size="sm" fw={700}>
        {topic.name}
      </Text>
      <Text size="xs">{statusLabel[topic.status]}</Text>
      <Text size="xs">
        {topic.groupCount} group(s), {topic.consumerCount} consumer(s)
      </Text>
      <Text size="xs">Total lag: {numberFormatter.format(topic.totalLag)}</Text>
      <Text size="xs">Failed repeats: {numberFormatter.format(topic.errorCount)}</Text>
      <Text size="xs">Last message: {formatDate(topic.lastMessageAt)}</Text>
    </Stack>
  );

  return (
    <Tooltip label={tooltip} withArrow openDelay={250}>
      <UnstyledButton
        aria-label={`Topic ${topic.name}`}
        className="topic-tile"
        onClick={onOpen}
        style={
          {
            '--topic-tile-accent': `var(--mantine-color-${color}-filled)`,
            '--topic-tile-bg': `var(--mantine-color-${color}-light)`,
            '--topic-tile-text': `var(--mantine-color-${color}-light-color)`,
          } as CSSProperties
        }
      >
        <Group gap={4} justify="space-between" wrap="nowrap">
          <Text fw={700} size="sm" truncate>
            {topic.name}
          </Text>
          <Badge color={color} size="xs" variant="filled" radius="sm">
            {topic.consumerCount}
          </Badge>
        </Group>
        <Group gap={6} justify="space-between" wrap="nowrap">
          <Text size="xs" fw={600} c={topic.totalLag > 0 ? 'orange' : undefined} className="topic-tile-metric">
            Lag {compactNumber(topic.totalLag)}
          </Text>
          {topic.errorCount > 0 && (
            <Text size="xs" fw={700} className="topic-tile-metric topic-tile-errors">
              {compactNumber(topic.errorCount)} err
            </Text>
          )}
        </Group>
        <Text size="xs" className="topic-tile-metric topic-tile-muted" truncate>
          {topic.rate.toFixed(2)} msg/s avg
        </Text>
        <Text size="xs" className="topic-tile-metric topic-tile-muted" truncate>
          Last: {formatAge(topic.lastMessageAt)}
        </Text>
      </UnstyledButton>
    </Tooltip>
  );
}

export function TopicOverview() {
  const navigate = useNavigate();
  const [topics, setTopics] = useState<TopicStat[]>([]);
  const [repeats, setRepeats] = useState<RepeatStat[]>([]);
  const [search, setSearch] = useState('');
  const { execute, isLoading } = useRequest();
  const { ref, width } = useElementSize();
  const { height } = useViewportSize();

  const refresh = useCallback(
    (notify = true) => {
      void execute(
        async () => {
          const [topicStat, repeatStat] = await Promise.all([
            dataBus.getTopicStat(),
            dataBus.getRepeatStat(),
          ]);
          return { topicStat, repeatStat };
        },
        ({ topicStat, repeatStat }) => {
          setTopics(topicStat);
          setRepeats(repeatStat);
        },
        notify ? 'Topic overview refreshed' : undefined,
      );
    },
    [execute],
  );

  useEffect(() => {
    refresh(false);
  }, [refresh]);

  const overview = useMemo(() => buildOverview(topics, repeats), [topics, repeats]);

  const visibleTopics = useMemo(() => {
    const query = search.trim().toLowerCase();
    return query ? overview.filter((topic) => topic.name.toLowerCase().includes(query)) : overview;
  }, [overview, search]);

  const columns = gridColumns(width, height, visibleTopics.length);

  const counters = useMemo(() => {
    const byStatus = { critical: 0, warning: 0, active: 0, idle: 0 };
    for (const topic of overview) byStatus[topic.status] += 1;
    return byStatus;
  }, [overview]);

  return (
    <Paper withBorder radius="lg" p={{ base: 'md', sm: 'lg' }}>
      <Group justify="space-between" mb="sm" align="flex-end" wrap="nowrap">
        <Stack gap={4}>
          <Title order={2}>Topics overview</Title>
          <Group gap="xs">
            <Badge color="teal" variant="light">
              {counters.active} active
            </Badge>
            <Badge color="gray" variant="light">
              {counters.idle} idle
            </Badge>
            {counters.warning > 0 && (
              <Badge color="yellow" variant="light">
                {counters.warning} with errors
              </Badge>
            )}
            {counters.critical > 0 && (
              <Badge color="red" variant="light">
                {counters.critical} critical
              </Badge>
            )}
          </Group>
        </Stack>
        <Group gap="xs" wrap="nowrap">
          <TextInput
            aria-label="Search topics"
            leftSection={<Search size={15} />}
            onChange={(event) => setSearch(event.currentTarget.value)}
            placeholder="Search topics"
            size="xs"
            value={search}
            w={200}
          />
          <Tooltip label="Refresh">
            <ActionIcon
              aria-label="Refresh topic overview"
              loading={isLoading}
              onClick={() => refresh()}
              size="lg"
              variant="light"
            >
              <RefreshCw size={19} />
            </ActionIcon>
          </Tooltip>
        </Group>
      </Group>

      <div ref={ref}>
        {isLoading && topics.length === 0 ? (
          <Center py="xl">
            <Loader />
          </Center>
        ) : visibleTopics.length === 0 ? (
          <Text c="dimmed" ta="center" py="lg">
            {overview.length === 0 ? 'No topics reported' : 'No topics match the search'}
          </Text>
        ) : (
          <div
            className="topic-tile-grid"
            style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}
          >
            {visibleTopics.map((topic) => (
              <TopicTile
                key={topic.name}
                topic={topic}
                onOpen={() => navigate(`/topics?topic=${encodeURIComponent(topic.name)}`)}
              />
            ))}
          </div>
        )}
      </div>
    </Paper>
  );
}
