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

test('topics without groups have neutral lag and never invent zero', () => {
  const result = topicLag({ name: 'orders', groups: [], partitions: [{ n: 0, firstOffset: 0, lastOffset: 3 }] });
  assert.deepEqual(result, { total: null, max: null, reason: 'No consumer groups', noGroups: true });
  assert.equal(topicLag({ name: 'orders', groups: null, partitions: [] }).noGroups, true);
});

test('no-group topics retain explicit metadata and partition failures', () => {
  for (const topic of [
    { name: 'orders', groups: [], partitions: [], error: 'metadata unavailable' },
    { name: 'orders', groups: [], partitions: [{ n: 0, firstOffset: -1, lastOffset: -1, error: 'LEADER_NOT_AVAILABLE' }] },
  ]) {
    const result = topicLag(topic);
    assert.equal(result.noGroups, false);
    assert.equal(result.total, null);
    assert.notEqual(result.reason, 'No consumer groups');
    assert.match(result.reason, /metadata unavailable|LEADER_NOT_AVAILABLE/);
  }
});

test('known groups retain lag even with zero consumers', () => {
  const result = topicLag({ name: 'orders', partitions: [], groups: [{
    name: 'billing', kafkaGroupId: 'billing-orders', state: 'Empty', error: '', consumers: [],
    partitions: [{ n: 0, firstOffset: 0, lastOffset: 5, offset: 2, lag: 3, committed: true, consumerId: '', consumerState: '' }],
  }] });
  assert.equal(result.noGroups, false);
  assert.equal(result.total, 3);
  assert.equal(result.max, 3);
  assert.equal(result.reason, '');
});

test('known groups missing or failing offsets remain unavailable', () => {
  const group = { name: 'billing', kafkaGroupId: 'billing-orders', state: '', error: '', consumers: [] };
  for (const partitions of [[], [{ n: 0, firstOffset: 0, lastOffset: 5, offset: -1, lag: -1, committed: false, consumerId: '', consumerState: '', lagError: 'request timeout' }]]) {
    const result = topicLag({ name: 'orders', partitions: [], groups: [{ ...group, partitions }] });
    assert.equal(result.noGroups, false);
    assert.equal(result.total, null);
    assert.match(result.reason, /No partition offsets available|request timeout/);
  }
});
