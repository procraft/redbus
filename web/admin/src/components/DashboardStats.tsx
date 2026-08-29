import {
  Group,
  Paper,
  SimpleGrid,
  Skeleton,
  Stack,
  Text,
  ThemeIcon,
  UnstyledButton,
} from '@mantine/core';
import { Cable, Layers3, TriangleAlert } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router';

import dataBus from '@/api/dataBus';
import type { DashboardStat } from '@/api/types';
import { useRequest } from '@/hooks/useRequest';
import { useServerEvents } from '@/hooks/useServerEvents';

const initialStat: DashboardStat = {
  consumerCount: 0,
  consumeTopicCount: 0,
  repeatAllCount: 0,
  repeatFailedCount: 0,
};

const numberFormatter = new Intl.NumberFormat('en-US');

export function DashboardStats() {
  const navigate = useNavigate();
  const [stat, setStat] = useState(initialStat);
  const { execute, isLoading } = useRequest();

  const loadStat = useCallback(() => {
    void execute(dataBus.getDashboardStat, setStat);
  }, [execute]);

  useEffect(loadStat, [loadStat]);

  const updateConsumers = useCallback((data: Pick<DashboardStat, 'consumerCount' | 'consumeTopicCount'>) => {
    setStat((current) => ({ ...current, ...data }));
  }, []);

  const updateRepeater = useCallback((data: { allCount: number; failedCount: number }) => {
    setStat((current) => ({
      ...current,
      repeatAllCount: data.allCount,
      repeatFailedCount: data.failedCount,
    }));
  }, []);

  useServerEvents({ onConsumers: updateConsumers, onRepeater: updateRepeater, onOpen: loadStat });

  const cards = [
    {
      label: 'Consume topics',
      value: numberFormatter.format(stat.consumeTopicCount),
      icon: Layers3,
      color: 'blue',
      path: '/topics',
    },
    {
      label: 'Consumers',
      value: numberFormatter.format(stat.consumerCount),
      icon: Cable,
      color: 'teal',
      path: '/consumers',
    },
    {
      label: 'Failed repeat',
      value: `${numberFormatter.format(stat.repeatFailedCount)} / ${numberFormatter.format(stat.repeatAllCount)}`,
      icon: TriangleAlert,
      color: stat.repeatFailedCount > 0 ? 'red' : 'gray',
      path: '/failed-repeats',
    },
  ];

  return (
    <SimpleGrid cols={{ base: 1, sm: 2, lg: 3 }}>
      {cards.map((card) => (
        <UnstyledButton key={card.label} onClick={() => navigate(card.path)} className="stat-card-button">
          <Paper withBorder radius="lg" p="lg" className="stat-card">
            <Group justify="space-between" align="flex-start">
              <Stack gap={6}>
                <Text c="dimmed" fw={700} size="xs" tt="uppercase">
                  {card.label}
                </Text>
                <Skeleton visible={isLoading} width={120}>
                  <Text fw={700} size="2rem" lh={1.2}>
                    {card.value}
                  </Text>
                </Skeleton>
              </Stack>
              <ThemeIcon color={card.color} variant="light" size={48} radius="md">
                <card.icon size={26} strokeWidth={1.7} />
              </ThemeIcon>
            </Group>
          </Paper>
        </UnstyledButton>
      ))}
    </SimpleGrid>
  );
}
