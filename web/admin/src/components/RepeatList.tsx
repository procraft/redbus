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
import { RefreshCw, RotateCcw } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';

import dataBus from '@/api/dataBus';
import type { RepeatStat } from '@/api/types';
import { useRequest } from '@/hooks/useRequest';

export function RepeatList() {
  const [items, setItems] = useState<RepeatStat[]>([]);
  const [restartingKey, setRestartingKey] = useState<string>();
  const { execute, isLoading } = useRequest();

  const refresh = useCallback(
    (notify = true) =>
      execute(
        dataBus.getRepeatStat,
        setItems,
        notify ? 'Repeater statistics refreshed' : undefined,
      ),
    [execute],
  );

  useEffect(() => {
    void refresh(false);
  }, [refresh]);

  const restart = async (item: RepeatStat) => {
    const key = `${item.topic}:${item.group}`;
    setRestartingKey(key);

    const succeeded = await execute(
      () => dataBus.repeatTopicGroup(item.topic, item.group),
      undefined,
      `Restart requested for ${item.topic} / ${item.group}`,
    );

    if (succeeded) {
      await refresh(false);
    }

    setRestartingKey(undefined);
  };

  return (
    <Paper withBorder radius="lg" p={{ base: 'md', sm: 'lg' }}>
      <Group justify="space-between" mb="md">
        <Stack gap={2}>
          <Title order={2}>Repeater statistics</Title>
          <Text c="dimmed" size="sm">
            Failed messages grouped by topic and consumer group
          </Text>
        </Stack>
        <Tooltip label="Refresh">
          <ActionIcon
            aria-label="Refresh repeater statistics"
            loading={isLoading && !restartingKey}
            onClick={() => void refresh()}
            size="lg"
            variant="light"
          >
            <RefreshCw size={19} />
          </ActionIcon>
        </Tooltip>
      </Group>

      {isLoading && items.length === 0 ? (
        <Center py="xl">
          <Loader />
        </Center>
      ) : (
        <Table.ScrollContainer minWidth={720}>
          <Table striped highlightOnHover withTableBorder verticalSpacing="sm">
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Topic / group</Table.Th>
                <Table.Th>Failed / total</Table.Th>
                <Table.Th>Last error</Table.Th>
                <Table.Th ta="right">Actions</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {items.map((item) => {
                const key = `${item.topic}:${item.group}`;
                return (
                  <Table.Tr key={key}>
                    <Table.Td>
                      <Text fw={600}>{item.topic}</Text>
                      <Text c="dimmed" size="xs">
                        {item.group}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <Badge color={item.failedCount > 0 ? 'red' : 'teal'} variant="light">
                        {item.failedCount} / {item.allCount}
                      </Badge>
                    </Table.Td>
                    <Table.Td maw={520}>
                      <Text lineClamp={3} size="sm">
                        {item.lastError || '—'}
                      </Text>
                    </Table.Td>
                    <Table.Td ta="right">
                      <Tooltip label="Restart all failed messages">
                        <ActionIcon
                          aria-label={`Restart failed messages for ${item.topic} / ${item.group}`}
                          color="orange"
                          disabled={item.failedCount === 0}
                          loading={restartingKey === key}
                          onClick={() => void restart(item)}
                          variant="light"
                        >
                          <RotateCcw size={18} />
                        </ActionIcon>
                      </Tooltip>
                    </Table.Td>
                  </Table.Tr>
                );
              })}
              {items.length === 0 && (
                <Table.Tr>
                  <Table.Td colSpan={4}>
                    <Text c="dimmed" ta="center" py="lg">
                      No repeat records reported
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
