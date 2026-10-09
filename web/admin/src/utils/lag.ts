import type { TopicStat } from '../api/types';

type LagPartition = { n: number; lag: number; lagError?: string };

export function summarizeLag(partitions: LagPartition[], errors: string[] = []) {
  const reasons = errors.filter(Boolean);
  for (const partition of partitions) {
    if (partition.lagError || !Number.isFinite(partition.lag) || partition.lag < 0) {
      reasons.push(`p${partition.n}: ${partition.lagError || 'invalid lag offset'}`);
    }
  }
  const available = reasons.length === 0 && partitions.length > 0;
  return {
    total: available ? partitions.reduce((sum, partition) => sum + partition.lag, 0) : null,
    max: available ? Math.max(...partitions.map((partition) => partition.lag)) : null,
    reason: reasons.join('; ') || (available ? '' : 'No partition offsets available'),
  };
}

export function topicLag(topic: TopicStat) {
  const groups = topic.groups ?? [];
  const errors = [
    topic.error || '',
    ...(groups.length === 0 ? topic.partitions ?? [] : []).filter((partition) => partition.error).map((partition) => `p${partition.n}: ${partition.error}`),
    ...groups.flatMap((group) => [
      group.error ? `${group.name}: ${group.error}` : '',
      !(group.partitions?.length) ? `${group.name}: No partition offsets available` : '',
    ]),
  ];
  const lag = summarizeLag(groups.flatMap((group) => (group.partitions ?? []).map((partition) => ({
    ...partition,
    lagError: partition.lagError ? `${group.name}: ${partition.lagError}` : '',
  }))), errors);
  const noGroups = groups.length === 0 && !errors.some(Boolean);
  return { ...lag, noGroups, reason: noGroups ? 'No consumer groups' : lag.reason };
}

export function formatLag(value: number | null) {
  return value === null ? 'Unavailable' : new Intl.NumberFormat('en-US').format(value);
}
