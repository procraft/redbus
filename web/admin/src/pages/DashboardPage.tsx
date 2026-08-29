import { Stack, Text, Title } from '@mantine/core';

import { DashboardStats } from '@/components/DashboardStats';

export function DashboardPage() {
  return (
    <Stack gap="lg">
      <div>
        <Title order={1}>Dashboard</Title>
        <Text c="dimmed">Live RED Bus health and workload overview</Text>
      </div>
      <DashboardStats />
    </Stack>
  );
}
