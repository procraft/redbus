import { Stack } from '@mantine/core';

import { ConsumerList } from '@/components/ConsumerList';
import { DashboardStats } from '@/components/DashboardStats';

export function ConsumersPage() {
  return (
    <Stack gap="lg">
      <DashboardStats />
      <ConsumerList />
    </Stack>
  );
}
