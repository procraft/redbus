import assert from 'node:assert/strict';
import { test } from 'node:test';
import { summarizeLag, topicLag } from '../src/utils/lag.ts';

test('valid offsets keep numeric totals', () => {
  assert.deepEqual(summarizeLag([{ n: 0, lag: 2 }, { n: 1, lag: 15 }]), { total: 17, max: 15, reason: '' });
});

test('mixed valid and unavailable partitions never produce partial totals', () => {
  const result = summarizeLag([{ n: 0, lag: 2 }, { n: 1, lag: -1, lagError: 'LEADER_NOT_AVAILABLE' }]);
  assert.equal(result.total, null);
  assert.equal(result.max, null);
  assert.match(result.reason, /p1: LEADER_NOT_AVAILABLE/);
});

test('legacy negative lag and absent assignments are unknown', () => {
  assert.equal(summarizeLag([{ n: 0, lag: -5 }]).total, null);
  assert.equal(summarizeLag([]).total, null);
});

test('topic metadata errors and groups without offsets suppress totals', () => {
  assert.equal(topicLag({ name: 'orders', error: 'metadata unavailable', groups: [], partitions: [] }).total, null);
  assert.equal(topicLag({ name: 'orders', groups: [{ name: 'billing', kafkaGroupId: '', state: '', error: '', partitions: [], consumers: [] }], partitions: [] }).total, null);
});
