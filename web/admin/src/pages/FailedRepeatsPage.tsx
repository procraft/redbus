import { Stack } from '@mantine/core';

import { DashboardStats } from '@/components/DashboardStats';
import { RepeatList } from '@/components/RepeatList';

export function FailedRepeatsPage() {
  return (
    <Stack gap="lg">
      <DashboardStats />
      <RepeatList />
    </Stack>
  );
}
