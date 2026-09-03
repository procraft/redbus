import {
  ActionIcon,
  Alert,
  Badge,
  Box,
  Button,
  Center,
  Collapse,
  Group,
  Loader,
  Modal,
  NumberInput,
  Paper,
  Select,
  Stack,
  Table,
  Text,
  Title,
  Tooltip,
} from '@mantine/core';
import {
  ChevronDown,
  ChevronRight,
  RefreshCw,
  RotateCcw,
  Trash2,
  TriangleAlert,
} from 'lucide-react';
import { Fragment, useCallback, useEffect, useState } from 'react';

import dataBus from '@/api/dataBus';
import type { RepeatErrorStat, RepeatStat } from '@/api/types';
import { useRequest } from '@/hooks/useRequest';

type PeriodUnit = 'hours' | 'days';

type RestartTarget = {
  item: RepeatStat;
  errorStat?: RepeatErrorStat;
};

type DeleteTarget = {
  item: RepeatStat;
  errorStat: RepeatErrorStat;
};

function formatDate(value?: string | null): string {
  if (!value || value.startsWith('0001-')) return '—';

  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString();
}

export function RepeatList() {
  const [items, setItems] = useState<RepeatStat[]>([]);
  const [restartingKey, setRestartingKey] = useState<string>();
  const [deletingKey, setDeletingKey] = useState<string>();
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [restartTarget, setRestartTarget] = useState<RestartTarget>();
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget>();
  const [periodAmount, setPeriodAmount] = useState<number | string>(24);
  const [periodUnit, setPeriodUnit] = useState<PeriodUnit>('hours');
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

  const restart = async () => {
    if (!restartTarget) return;

    const amount = typeof periodAmount === 'number' ? periodAmount : Number(periodAmount);
    const unitSeconds = periodUnit === 'days' ? 24 * 60 * 60 : 60 * 60;
    const lookbackSeconds = amount * unitSeconds;
    if (!Number.isInteger(amount) || amount <= 0 || !Number.isSafeInteger(lookbackSeconds)) return;

    const { item, errorStat } = restartTarget;
    const key = errorStat
      ? `error:${item.topic}\u0000${item.group}\u0000${errorStat.error}`
      : `group:${item.topic}\u0000${item.group}`;
    setRestartingKey(key);

    const succeeded = await execute(
      () =>
        errorStat
          ? dataBus.repeatError(item.topic, item.group, errorStat.error, lookbackSeconds)
          : dataBus.repeatTopicGroupSince(item.topic, item.group, lookbackSeconds),
      undefined,
      `Restart requested for ${item.topic} / ${item.group}${errorStat ? `: ${errorStat.error || 'empty error'}` : ''}`,
    );

    if (succeeded) {
      setRestartTarget(undefined);
      await refresh(false);
    }

    setRestartingKey(undefined);
  };

  const toggleExpanded = (key: string) => {
    setExpanded((current) => {
      const next = new Set(current);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  const deleteFinished = async () => {
    if (!deleteTarget) return;

    const { item, errorStat } = deleteTarget;
    const key = `delete:${item.topic}\u0000${item.group}\u0000${errorStat.error}`;
    setDeletingKey(key);

    const succeeded = await execute(
      () => dataBus.deleteError(item.topic, item.group, errorStat.error),
      undefined,
      `Finished messages deleted for ${item.topic} / ${item.group}: ${errorStat.error || 'empty error'}`,
    );

    if (succeeded) {
      setDeleteTarget(undefined);
      await refresh(false);
    }

    setDeletingKey(undefined);
  };

  return (
    <>
      <Modal
        centered
        onClose={() => !restartingKey && setRestartTarget(undefined)}
        opened={restartTarget !== undefined}
        title="Restart failed messages"
      >
        <Stack gap="md">
          <div>
            <Text fw={600}>
              {restartTarget?.item.topic} / {restartTarget?.item.group}
            </Text>
            {restartTarget?.errorStat && (
              <Text c="dimmed" lineClamp={3} size="sm">
                {restartTarget.errorStat.error || 'Empty error'}
              </Text>
            )}
          </div>
          <Text size="sm">
            Restart messages that became permanently failed during the selected period.
          </Text>
          <Group align="flex-end" grow>
            <NumberInput
              allowDecimal={false}
              label="Last"
              min={1}
              onChange={setPeriodAmount}
              value={periodAmount}
            />
            <Select
              allowDeselect={false}
              data={[
                { value: 'hours', label: 'Hours' },
                { value: 'days', label: 'Days' },
              ]}
              label="Unit"
              onChange={(value) => value && setPeriodUnit(value as PeriodUnit)}
              value={periodUnit}
            />
          </Group>
          <Group justify="flex-end">
            <Button
              disabled={Boolean(restartingKey)}
              onClick={() => setRestartTarget(undefined)}
              variant="default"
            >
              Cancel
            </Button>
            <Button
              loading={Boolean(restartingKey)}
              onClick={() => void restart()}
              disabled={
                !Number.isInteger(Number(periodAmount)) ||
                Number(periodAmount) <= 0 ||
                !Number.isSafeInteger(
                  Number(periodAmount) * (periodUnit === 'days' ? 24 * 60 * 60 : 60 * 60),
                )
              }
            >
              Restart
            </Button>
          </Group>
        </Stack>
      </Modal>

      <Modal
        centered
        onClose={() => !deletingKey && setDeleteTarget(undefined)}
        opened={deleteTarget !== undefined}
        title="Delete failed messages permanently?"
      >
        <Stack gap="md">
          <div>
            <Text fw={600}>
              {deleteTarget?.item.topic} / {deleteTarget?.item.group}
            </Text>
            <Text c="dimmed" lineClamp={3} size="sm">
              {deleteTarget?.errorStat.error || 'Empty error'}
            </Text>
          </div>
          <Alert color="red" icon={<TriangleAlert size={20} />} title="Permanent deletion">
            All {deleteTarget?.errorStat.failedCount ?? 0} finished repeat messages in this error
            class will be deleted. This action cannot be undone, and the messages cannot be
            restarted or recovered.
          </Alert>
          <Group justify="flex-end">
            <Button
              disabled={Boolean(deletingKey)}
              onClick={() => setDeleteTarget(undefined)}
              variant="default"
            >
              Cancel
            </Button>
            <Button
              color="red"
              leftSection={<Trash2 size={17} />}
              loading={Boolean(deletingKey)}
              onClick={() => void deleteFinished()}
            >
              Delete permanently
            </Button>
          </Group>
        </Stack>
      </Modal>

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
        <Table.ScrollContainer minWidth={760}>
          <Table striped highlightOnHover withTableBorder verticalSpacing="sm">
            <Table.Thead>
              <Table.Tr>
                <Table.Th w={42} />
                <Table.Th>Topic / group</Table.Th>
                <Table.Th>Failed / total</Table.Th>
                <Table.Th>Last error</Table.Th>
                <Table.Th ta="right">Actions</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {items.map((item) => {
                const key = `${item.topic}\u0000${item.group}`;
                const groupRestartKey = `group:${key}`;
                const errors = item.errors ?? [];
                const isExpanded = errors.length > 0 && expanded.has(key);
                return (
                  <Fragment key={key}>
                    <Table.Tr>
                      <Table.Td>
                        <ActionIcon
                          aria-label={`${isExpanded ? 'Collapse' : 'Expand'} errors for ${item.topic} / ${item.group}`}
                          aria-expanded={isExpanded}
                          disabled={errors.length === 0}
                          onClick={() => toggleExpanded(key)}
                          size="sm"
                          variant="subtle"
                        >
                          {isExpanded ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                        </ActionIcon>
                      </Table.Td>
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
                        <Tooltip label="Restart failed messages for a period">
                          <ActionIcon
                            aria-label={`Restart all failed messages for ${item.topic} / ${item.group}`}
                            color="orange"
                            disabled={item.failedCount === 0}
                            loading={restartingKey === groupRestartKey}
                            onClick={() => setRestartTarget({ item })}
                            variant="light"
                          >
                            <RotateCcw size={18} />
                          </ActionIcon>
                        </Tooltip>
                      </Table.Td>
                    </Table.Tr>
                    {errors.length > 0 && (
                      <Table.Tr>
                        <Table.Td colSpan={5} p={0}>
                          <Collapse expanded={isExpanded}>
                            <Box p="md" bg="var(--mantine-color-default-hover)">
                              <Table withTableBorder verticalSpacing="xs">
                                <Table.Thead>
                                  <Table.Tr>
                                    <Table.Th>Error class</Table.Th>
                                    <Table.Th w={120}>Failed</Table.Th>
                                    <Table.Th w={190}>First failed</Table.Th>
                                    <Table.Th w={190}>Last failed</Table.Th>
                                    <Table.Th w={112} ta="right">
                                      Actions
                                    </Table.Th>
                                  </Table.Tr>
                                </Table.Thead>
                                <Table.Tbody>
                                  {errors.map((errorStat) => {
                                    const errorRestartKey = `error:${key}\u0000${errorStat.error}`;
                                    return (
                                      <Table.Tr key={errorStat.error}>
                                        <Table.Td>
                                          <Text
                                            c={errorStat.error ? undefined : 'dimmed'}
                                            size="sm"
                                            style={{ overflowWrap: 'anywhere' }}
                                          >
                                            {errorStat.error || 'Empty error'}
                                          </Text>
                                        </Table.Td>
                                        <Table.Td>
                                          <Badge color="red" variant="light">
                                            {errorStat.failedCount}
                                          </Badge>
                                        </Table.Td>
                                        <Table.Td>
                                          <Text size="xs">{formatDate(errorStat.firstFailedAt)}</Text>
                                        </Table.Td>
                                        <Table.Td>
                                          <Text size="xs">{formatDate(errorStat.lastFailedAt)}</Text>
                                        </Table.Td>
                                        <Table.Td ta="right">
                                          <Group gap={4} justify="flex-end" wrap="nowrap">
                                            <Tooltip label="Restart this error class">
                                              <ActionIcon
                                                aria-label={`Restart ${errorStat.failedCount} messages with error ${errorStat.error || 'empty error'}`}
                                                color="orange"
                                                loading={restartingKey === errorRestartKey}
                                                onClick={() => setRestartTarget({ item, errorStat })}
                                                variant="light"
                                              >
                                                <RotateCcw size={17} />
                                              </ActionIcon>
                                            </Tooltip>
                                            <Tooltip label="Delete all finished messages in this error class">
                                              <ActionIcon
                                                aria-label={`Permanently delete ${errorStat.failedCount} finished messages with error ${errorStat.error || 'empty error'}`}
                                                color="red"
                                                loading={
                                                  deletingKey ===
                                                  `delete:${key}\u0000${errorStat.error}`
                                                }
                                                onClick={() => setDeleteTarget({ item, errorStat })}
                                                variant="light"
                                              >
                                                <Trash2 size={17} />
                                              </ActionIcon>
                                            </Tooltip>
                                          </Group>
                                        </Table.Td>
                                      </Table.Tr>
                                    );
                                  })}
                                </Table.Tbody>
                              </Table>
                            </Box>
                          </Collapse>
                        </Table.Td>
                      </Table.Tr>
                    )}
                  </Fragment>
                );
              })}
              {items.length === 0 && (
                <Table.Tr>
                  <Table.Td colSpan={5}>
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
    </>
  );
}
