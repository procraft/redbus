import { Stack, Text, Title } from '@mantine/core';

import { DashboardStats } from '@/components/DashboardStats';
import { TopicOverview } from '@/components/TopicOverview';

export function DashboardPage() {
  return (
    <Stack gap="lg">
      <div>
        <Title order={1}>Dashboard</Title>
        <Text c="dimmed">Live RED Bus health and workload overview</Text>
      </div>
      <DashboardStats />
      <TopicOverview />
    </Stack>
  );
}
