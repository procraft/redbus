import {
  ActionIcon,
  Badge,
  Center,
  Group,
  Loader,
  Paper,
  Stack,
  Table,
  Text,
  Title,
  Tooltip,
} from '@mantine/core';
import { RefreshCw } from 'lucide-react';
import { useCallback, useEffect, useMemo, useState } from 'react';

import dataBus from '@/api/dataBus';
import type { TopicGroupPartition, TopicPartition, TopicStat } from '@/api/types';
import { useRequest } from '@/hooks/useRequest';

type TopicRow = {
  key: string;
  topic: string;
  partition: TopicPartition;
  group?: string;
  groupPartition?: TopicGroupPartition;
};

function flattenTopics(topics: TopicStat[]): TopicRow[] {
  return topics.flatMap((topic) =>
    (topic.partitions ?? []).flatMap((partition) => {
      const groupRows = (topic.groups ?? []).flatMap((group) =>
        (group.partitions ?? [])
          .filter((groupPartition) => groupPartition.n === partition.n)
          .map((groupPartition) => ({
            key: `${topic.name}:${partition.n}:${group.name}:${groupPartition.consumerId}`,
            topic: topic.name,
            partition,
            group: group.name,
            groupPartition,
          })),
      );

      return groupRows.length > 0
        ? groupRows
        : [{ key: `${topic.name}:${partition.n}`, topic: topic.name, partition }];
    }),
  );
}

function stateColor(state?: string): string {
  switch (state) {
    case 'connected':
      return 'teal';
    case 'connecting':
      return 'yellow';
    case 'reconnecting':
      return 'orange';
    default:
      return 'gray';
  }
}

export function TopicList() {
  const [topics, setTopics] = useState<TopicStat[]>([]);
  const { execute, isLoading } = useRequest();

  const refresh = useCallback(
    (notify = true) =>
      execute(
        dataBus.getTopicStat,
        setTopics,
        notify ? 'Topic statistics refreshed' : undefined,
      ),
    [execute],
  );

  useEffect(() => {
    void refresh(false);
  }, [refresh]);

  const rows = useMemo(() => flattenTopics(topics), [topics]);

  return (
    <Paper withBorder radius="lg" p={{ base: 'md', sm: 'lg' }}>
      <Group justify="space-between" mb="md">
        <Stack gap={2}>
          <Title order={2}>Topic statistics</Title>
          <Text c="dimmed" size="sm">
            Kafka offsets and connected consumer groups
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

      {isLoading && rows.length === 0 ? (
        <Center py="xl">
          <Loader />
        </Center>
      ) : (
        <Table.ScrollContainer minWidth={860}>
          <Table striped highlightOnHover withTableBorder verticalSpacing="sm">
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Topic</Table.Th>
                <Table.Th>Partition</Table.Th>
                <Table.Th>First / last offset</Table.Th>
                <Table.Th>Group</Table.Th>
                <Table.Th>Group offset</Table.Th>
                <Table.Th>Consumer</Table.Th>
                <Table.Th>State</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {rows.map((row) => (
                <Table.Tr key={row.key}>
                  <Table.Td fw={600}>{row.topic}</Table.Td>
                  <Table.Td>{row.partition.n}</Table.Td>
                  <Table.Td>
                    {row.partition.firstOffset} / {row.partition.lastOffset}
                  </Table.Td>
                  <Table.Td>{row.group ?? '—'}</Table.Td>
                  <Table.Td>{row.groupPartition?.offset ?? '—'}</Table.Td>
                  <Table.Td>{row.groupPartition?.consumerId || '—'}</Table.Td>
                  <Table.Td>
                    {row.groupPartition?.consumerState ? (
                      <Badge color={stateColor(row.groupPartition.consumerState)} variant="light">
                        {row.groupPartition.consumerState}
                      </Badge>
                    ) : (
                      '—'
                    )}
                  </Table.Td>
                </Table.Tr>
              ))}
              {rows.length === 0 && (
                <Table.Tr>
                  <Table.Td colSpan={7}>
                    <Text c="dimmed" ta="center" py="lg">
                      No topics reported
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
