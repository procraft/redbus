import {
  ActionIcon,
  Badge,
  Center,
  Group,
  Loader,
  Paper,
  Select,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
  Tooltip,
} from '@mantine/core';
import { RefreshCw, Search } from 'lucide-react';
import { useCallback, useEffect, useMemo, useState } from 'react';

import dataBus from '@/api/dataBus';
import type { ConsumerStat } from '@/api/types';
import { useRequest } from '@/hooks/useRequest';

const numberFormatter = new Intl.NumberFormat('en-US');

function stateColor(state: string): string {
  switch (state) {
    case 'connected':
      return 'teal';
    case 'connecting':
      return 'yellow';
    case 'reconnecting':
      return 'orange';
    case 'broker-member':
      return 'blue';
    default:
      return 'gray';
  }
}

function validDate(value?: string | null): Date | null {
  if (!value || value.startsWith('0001-')) return null;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

function formatDate(value?: string | null): string {
  return validDate(value)?.toLocaleString() ?? '—';
}

function formatAge(value?: string | null): string {
  const date = validDate(value);
  if (!date) return '—';
  const seconds = Math.max(0, Math.floor((Date.now() - date.getTime()) / 1000));
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ${minutes % 60}m ago`;
  return `${Math.floor(hours / 24)}d ${hours % 24}h ago`;
}

function averageRate(consumer: ConsumerStat): number {
  const connectedAt = validDate(consumer.connectedAt);
  if (!connectedAt) return 0;
  const seconds = Math.max(1, (Date.now() - connectedAt.getTime()) / 1000);
  return consumer.messagesProcessed / seconds;
}

export function ConsumerList() {
  const [consumers, setConsumers] = useState<ConsumerStat[]>([]);
  const [search, setSearch] = useState('');
  const [stateFilter, setStateFilter] = useState<string | null>('all');
  const { execute, isLoading } = useRequest();

  const refresh = useCallback(
    (notify = true) =>
      execute(
        dataBus.getConsumerStat,
        setConsumers,
        notify ? 'Consumer statistics refreshed' : undefined,
      ),
    [execute],
  );

  useEffect(() => {
    void refresh(false);
  }, [refresh]);

  const filteredConsumers = useMemo(() => {
    const query = search.trim().toLowerCase();
    return [...consumers]
      .filter((consumer) => stateFilter === 'all' || consumer.state === stateFilter)
      .filter(
        (consumer) =>
          !query ||
          [consumer.id, consumer.topic, consumer.group, consumer.clientHost].some((value) =>
            value.toLowerCase().includes(query),
          ),
      )
      .sort(
        (left, right) =>
          left.topic.localeCompare(right.topic) ||
          left.group.localeCompare(right.group) ||
          left.id.localeCompare(right.id),
      );
  }, [consumers, search, stateFilter]);

  const connectedCount = consumers.filter((consumer) => consumer.state === 'connected').length;
  const reconnectingCount = consumers.filter((consumer) => consumer.state === 'reconnecting').length;

  return (
    <Paper withBorder radius="lg" p={{ base: 'md', sm: 'lg' }}>
      <Group justify="space-between" mb="md" align="flex-end">
        <Stack gap={4}>
          <Title order={2}>Consumers</Title>
          <Text c="dimmed" size="sm">
            Connected RED Bus clients, assignments, activity and errors
          </Text>
          <Group gap="xs" mt={4}>
            <Badge color="gray" variant="light">
              {consumers.length} total
            </Badge>
            <Badge color="teal" variant="light">
              {connectedCount} connected
            </Badge>
            {reconnectingCount > 0 && (
              <Badge color="orange" variant="light">
                {reconnectingCount} reconnecting
              </Badge>
            )}
          </Group>
        </Stack>
        <Tooltip label="Refresh">
          <ActionIcon
            aria-label="Refresh consumer statistics"
            loading={isLoading}
            onClick={() => void refresh()}
            size="lg"
            variant="light"
          >
            <RefreshCw size={19} />
          </ActionIcon>
        </Tooltip>
      </Group>

      <Group align="flex-end" mb="md" grow>
        <TextInput
          aria-label="Search consumers"
          leftSection={<Search size={16} />}
          onChange={(event) => setSearch(event.currentTarget.value)}
          placeholder="Search by consumer, topic, group or host"
          value={search}
        />
        <Select
          aria-label="Filter consumers by state"
          allowDeselect={false}
          data={[
            { value: 'all', label: 'All states' },
            { value: 'connected', label: 'Connected' },
            { value: 'connecting', label: 'Connecting' },
            { value: 'reconnecting', label: 'Reconnecting' },
            { value: 'broker-member', label: 'Broker member only' },
          ]}
          onChange={setStateFilter}
          value={stateFilter}
        />
      </Group>

      {isLoading && consumers.length === 0 ? (
        <Center py="xl">
          <Loader />
        </Center>
      ) : (
        <Table.ScrollContainer minWidth={1160}>
          <Table striped highlightOnHover withTableBorder verticalSpacing="sm">
            <Table.Thead>
              <Table.Tr>
                <Table.Th style={{ whiteSpace: 'nowrap' }}>Consumer / topic / group</Table.Th>
                <Table.Th style={{ whiteSpace: 'nowrap' }}>State / connection</Table.Th>
                <Table.Th>Partitions and lag</Table.Th>
                <Table.Th style={{ whiteSpace: 'nowrap' }}>Lag / activity</Table.Th>
                <Table.Th>Reconnects</Table.Th>
                <Table.Th>Last error</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {filteredConsumers.map((consumer) => {
                const partitions = [...(consumer.partitions ?? [])].sort((left, right) => left.n - right.n);
                const totalLag = partitions.reduce((total, partition) => total + partition.lag, 0);
                const maxLag = partitions.length > 0 ? Math.max(...partitions.map((partition) => partition.lag)) : 0;
                return (
                  <Table.Tr key={`${consumer.topic}:${consumer.group}:${consumer.id}:${consumer.kafkaMemberId}`}>
                    <Table.Td>
                      <Text fw={700} style={{ whiteSpace: 'nowrap' }}>
                        {consumer.id}
                      </Text>
                      <Text fw={600} size="sm" style={{ whiteSpace: 'nowrap' }}>
                        {consumer.topic} / {consumer.group}
                      </Text>
                      <Group gap="xs" wrap="nowrap">
                        <Badge color="gray" size="xs" variant="light">
                          Retry: {consumer.repeatStrategy || 'default'}
                        </Badge>
                        <Text c="dimmed" size="xs" style={{ whiteSpace: 'nowrap' }}>
                          {consumer.clientHost || 'Host unknown'}
                        </Text>
                      </Group>
                    </Table.Td>
                    <Table.Td>
                      <Group gap="xs" wrap="nowrap">
                        <Badge color={stateColor(consumer.state)} variant="light">
                          {consumer.state}
                        </Badge>
                        <Text c="dimmed" size="xs">
                          {formatAge(consumer.stateSince)}
                        </Text>
                      </Group>
                      <Tooltip label={formatDate(consumer.connectedAt)}>
                        <Text size="sm" mt={4}>
                          Connected: {formatAge(consumer.connectedAt)}
                        </Text>
                      </Tooltip>
                      <Text c="dimmed" size="xs">
                        Kafka: {consumer.kafkaMemberId || 'joining'}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <Stack gap={2}>
                        {partitions.length > 0
                          ? partitions.map((partition) => (
                              <Text key={partition.n} size="sm">
                                p{partition.n}: {partition.committed ? partition.groupOffset : '—'} /{' '}
                                {partition.lastOffset}, lag {numberFormatter.format(partition.lag)}
                              </Text>
                            ))
                          : 'Waiting for assignment'}
                      </Stack>
                    </Table.Td>
                    <Table.Td>
                      <Text fw={totalLag > 0 ? 700 : undefined} c={totalLag > 0 ? 'orange' : undefined}>
                        Lag: {numberFormatter.format(totalLag)} / {numberFormatter.format(maxLag)}
                      </Text>
                      <Text>{numberFormatter.format(consumer.messagesProcessed)} messages</Text>
                      <Text c="dimmed" size="xs">
                        {averageRate(consumer).toFixed(2)} msg/s avg
                      </Text>
                      <Tooltip label={formatDate(consumer.lastMessageAt)}>
                        <Text c="dimmed" size="xs">
                          Last: {formatAge(consumer.lastMessageAt)}
                        </Text>
                      </Tooltip>
                    </Table.Td>
                    <Table.Td>{numberFormatter.format(consumer.reconnectCount)}</Table.Td>
                    <Table.Td maw={260}>
                      {consumer.lastError ? (
                        <Tooltip label={consumer.lastError} multiline w={420}>
                          <Text c="red" lineClamp={2} size="sm">
                            {consumer.lastError}
                          </Text>
                        </Tooltip>
                      ) : (
                        '—'
                      )}
                    </Table.Td>
                  </Table.Tr>
                );
              })}
              {filteredConsumers.length === 0 && (
                <Table.Tr>
                  <Table.Td colSpan={6}>
                    <Text c="dimmed" ta="center" py="lg">
                      {consumers.length === 0
                        ? 'No consumers connected'
                        : 'No consumers match the filters'}
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
