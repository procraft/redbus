import { Stack } from '@mantine/core';

import { DashboardStats } from '@/components/DashboardStats';
import { TopicList } from '@/components/TopicList';

export function TopicsPage() {
  return (
    <Stack gap="lg">
      <DashboardStats />
      <TopicList />
    </Stack>
  );
}
