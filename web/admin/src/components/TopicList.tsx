import {
  ActionIcon,
  Badge,
  Box,
  Center,
  Collapse,
  Group,
  Loader,
  Paper,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
  Tooltip,
} from '@mantine/core';
import { ChevronDown, ChevronRight, RefreshCw, Search } from 'lucide-react';
import { Fragment, useCallback, useEffect, useMemo, useState } from 'react';

import dataBus from '@/api/dataBus';
import type { TopicGroup, TopicStat } from '@/api/types';
import { useRequest } from '@/hooks/useRequest';

const numberFormatter = new Intl.NumberFormat('en-US');

type TopicSummary = {
  retained: number;
  consumerCount: number;
  totalLag: number;
  maxLag: number;
  status: 'empty' | 'healthy' | 'lagging' | 'no-consumers';
};

function summarizeTopic(topic: TopicStat): TopicSummary {
  const partitions = topic.partitions ?? [];
  const groups = topic.groups ?? [];
  const retained = partitions.reduce(
    (total, partition) => total + Math.max(0, partition.lastOffset - partition.firstOffset),
    0,
  );
  const lags = groups.flatMap((group) => (group.partitions ?? []).map((partition) => partition.lag));
  const totalLag = lags.reduce((total, lag) => total + lag, 0);
  const maxLag = lags.length > 0 ? Math.max(...lags) : 0;
  const consumerCount = groups.reduce((total, group) => total + (group.consumers?.length ?? 0), 0);
  return {
    retained,
    consumerCount,
    totalLag,
    maxLag,
    status:
      groups.length === 0
        ? 'no-consumers'
        : totalLag > 0
          ? 'lagging'
          : retained === 0
            ? 'empty'
            : 'healthy',
  };
}

function topicStatusBadge(status: TopicSummary['status']) {
  const options = {
    empty: { color: 'gray', label: 'Empty' },
    healthy: { color: 'teal', label: 'Healthy' },
    lagging: { color: 'orange', label: 'Lagging' },
    'no-consumers': { color: 'yellow', label: 'No consumers' },
  }[status];
  return <Badge color={options.color}>{options.label}</Badge>;
}

function groupStateColor(state: string): string {
  switch (state.toLowerCase()) {
    case 'stable':
      return 'teal';
    case 'preparingrebalance':
    case 'completingrebalance':
      return 'yellow';
    case 'dead':
      return 'red';
    default:
      return 'gray';
  }
}

function GroupDetails({ group }: { group: TopicGroup }) {
  return (
    <Paper withBorder radius="md" p="md">
      <Group justify="space-between" mb="sm">
        <div>
          <Group gap="xs">
            <Text fw={700}>{group.name}</Text>
            <Badge color={groupStateColor(group.state || 'unknown')} variant="light">
              {group.state || 'Unknown'}
            </Badge>
          </Group>
          <Text c="dimmed" size="xs">
            Kafka group: {group.kafkaGroupId || '—'}
          </Text>
        </div>
        <Text c="dimmed" size="sm">
          {group.consumers?.length ?? 0} consumer(s)
        </Text>
      </Group>
      {group.error && (
        <Text c="red" size="sm" mb="sm">
          {group.error}
        </Text>
      )}
      <Table.ScrollContainer minWidth={680}>
        <Table verticalSpacing="xs">
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Partition</Table.Th>
              <Table.Th>First / last</Table.Th>
              <Table.Th>Committed offset</Table.Th>
              <Table.Th>Lag</Table.Th>
              <Table.Th>Assigned consumer</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {(group.partitions ?? []).map((partition) => (
              <Table.Tr key={partition.n}>
                <Table.Td>{partition.n}</Table.Td>
                <Table.Td>
                  {numberFormatter.format(partition.firstOffset)} /{' '}
                  {numberFormatter.format(partition.lastOffset)}
                </Table.Td>
                <Table.Td>
                  {partition.committed ? numberFormatter.format(partition.offset) : 'Not committed'}
                </Table.Td>
                <Table.Td fw={partition.lag > 0 ? 700 : undefined} c={partition.lag > 0 ? 'orange' : undefined}>
                  {numberFormatter.format(partition.lag)}
                </Table.Td>
                <Table.Td>{partition.consumerId || 'Unassigned'}</Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      </Table.ScrollContainer>
    </Paper>
  );
}

export function TopicList() {
  const [topics, setTopics] = useState<TopicStat[]>([]);
  const [search, setSearch] = useState('');
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const { execute, isLoading } = useRequest();

  const refresh = useCallback(
    (notify = true) =>
      execute(dataBus.getTopicStat, setTopics, notify ? 'Topic statistics refreshed' : undefined),
    [execute],
  );

  useEffect(() => {
    void refresh(false);
  }, [refresh]);

  const filteredTopics = useMemo(() => {
    const query = search.trim().toLowerCase();
    return topics.filter((topic) => !query || topic.name.toLowerCase().includes(query));
  }, [search, topics]);

  const toggleTopic = (topic: string) => {
    setExpanded((current) => {
      const next = new Set(current);
      if (next.has(topic)) next.delete(topic);
      else next.add(topic);
      return next;
    });
  };

  return (
    <Paper withBorder radius="lg" p={{ base: 'md', sm: 'lg' }}>
      <Group justify="space-between" mb="md" align="flex-end">
        <Stack gap={2}>
          <Title order={2}>Topics</Title>
          <Text c="dimmed" size="sm">
            Kafka inventory, retained records and consumer-group lag
          </Text>
        </Stack>
        <Tooltip label="Refresh">
          <ActionIcon
            aria-label="Refresh topic statistics"
            loading={isLoading}
            onClick={() => void refresh()}
            size="lg"
            variant="light"
          >
            <RefreshCw size={19} />
          </ActionIcon>
        </Tooltip>
      </Group>

      <TextInput
        aria-label="Search topics"
        leftSection={<Search size={16} />}
        mb="md"
        onChange={(event) => setSearch(event.currentTarget.value)}
        placeholder="Search topics"
        value={search}
      />

      {isLoading && topics.length === 0 ? (
        <Center py="xl">
          <Loader />
        </Center>
      ) : (
        <Table.ScrollContainer minWidth={900}>
          <Table striped highlightOnHover withTableBorder verticalSpacing="sm">
            <Table.Thead>
              <Table.Tr>
                <Table.Th w={42} />
                <Table.Th>Topic</Table.Th>
                <Table.Th>Partitions</Table.Th>
                <Table.Th>Retained</Table.Th>
                <Table.Th>Groups</Table.Th>
                <Table.Th>Consumers</Table.Th>
                <Table.Th>Total lag</Table.Th>
                <Table.Th>Max lag</Table.Th>
                <Table.Th>Status</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {filteredTopics.map((topic) => {
                const summary = summarizeTopic(topic);
                const isExpanded = expanded.has(topic.name);
                return (
                  <Fragment key={topic.name}>
                    <Table.Tr>
                      <Table.Td>
                        <ActionIcon
                          aria-label={`${isExpanded ? 'Collapse' : 'Expand'} ${topic.name}`}
                          disabled={(topic.groups?.length ?? 0) === 0}
                          onClick={() => toggleTopic(topic.name)}
                          size="sm"
                          variant="subtle"
                        >
                          {isExpanded ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                        </ActionIcon>
                      </Table.Td>
                      <Table.Td fw={700}>{topic.name}</Table.Td>
                      <Table.Td>{topic.partitions?.length ?? 0}</Table.Td>
                      <Table.Td>{numberFormatter.format(summary.retained)}</Table.Td>
                      <Table.Td>{topic.groups?.length ?? 0}</Table.Td>
                      <Table.Td>{summary.consumerCount}</Table.Td>
                      <Table.Td>{numberFormatter.format(summary.totalLag)}</Table.Td>
                      <Table.Td>{numberFormatter.format(summary.maxLag)}</Table.Td>
                      <Table.Td>{topicStatusBadge(summary.status)}</Table.Td>
                    </Table.Tr>
                    {(topic.groups?.length ?? 0) > 0 && (
                      <Table.Tr>
                        <Table.Td colSpan={9} p={0}>
                          <Collapse expanded={isExpanded}>
                            <Box p="md" bg="var(--mantine-color-default-hover)">
                              <Stack gap="sm">
                                {(topic.groups ?? []).map((group) => (
                                  <GroupDetails key={group.name} group={group} />
                                ))}
                              </Stack>
                            </Box>
                          </Collapse>
                        </Table.Td>
                      </Table.Tr>
                    )}
                  </Fragment>
                );
              })}
              {filteredTopics.length === 0 && (
                <Table.Tr>
                  <Table.Td colSpan={9}>
                    <Text c="dimmed" ta="center" py="lg">
                      {topics.length === 0 ? 'No topics reported' : 'No topics match the search'}
                    </Text>
                  </Table.Td>
                </Table.Tr>
              )}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      )}
    </Paper>
  );
}
