import httpClient from '@/api/httpClient';
import type { DashboardStat, RepeatStat, TopicStat } from '@/api/types';

const dataBus = {
  async getDashboardStat(): Promise<DashboardStat> {
    const { data } = await httpClient.post<DashboardStat>('/dashboard/stat', {});
    return data;
  },

  async getTopicStat(): Promise<TopicStat[]> {
    const { data } = await httpClient.post<{ list: TopicStat[] | null }>('/topic/stat', {});
    return data.list ?? [];
  },

  async getRepeatStat(): Promise<RepeatStat[]> {
    const { data } = await httpClient.post<{ list: RepeatStat[] | null }>('/repeat/stat', {});
    return data.list ?? [];
  },

  async repeatTopicGroup(topic: string, group: string): Promise<void> {
    await httpClient.post('/repeat/repeatTopicGroup', { topic, group });
  },
};

export default dataBus;
