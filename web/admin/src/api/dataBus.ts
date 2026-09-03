import httpClient from '@/api/httpClient';
import type { ConsumerStat, DashboardStat, RepeatStat, TopicStat } from '@/api/types';

const dataBus = {
  async getDashboardStat(): Promise<DashboardStat> {
    const { data } = await httpClient.post<DashboardStat>('/dashboard/stat', {});
    return data;
  },

  async getTopicStat(): Promise<TopicStat[]> {
    const { data } = await httpClient.post<{ list: TopicStat[] | null }>('/topic/stat', {});
    return data.list ?? [];
  },

  async getConsumerStat(): Promise<ConsumerStat[]> {
    const { data } = await httpClient.post<{ list: ConsumerStat[] | null }>('/consumer/stat', {});
    return data.list ?? [];
  },

  async getRepeatStat(): Promise<RepeatStat[]> {
    const { data } = await httpClient.post<{ list: RepeatStat[] | null }>('/repeat/stat', {});
    return data.list ?? [];
  },

  async repeatTopicGroupSince(
    topic: string,
    group: string,
    lookbackSeconds: number,
  ): Promise<void> {
    await httpClient.post('/repeat/repeatTopicGroupSince', { topic, group, lookbackSeconds });
  },

  async repeatError(
    topic: string,
    group: string,
    error: string,
    lookbackSeconds: number,
  ): Promise<void> {
    await httpClient.post('/repeat/repeatError', { topic, group, error, lookbackSeconds });
  },
};

export default dataBus;
