import { describe, it, expect, vi } from 'vitest';

// Mock the API client so probe() can be exercised without a backend.
const { fetchHealth } = vi.hoisted(() => ({ fetchHealth: vi.fn() }));
vi.mock('../api', () => ({ api: { fetchHealth } }));

import {
  nextPollDelay,
  formatUptime,
  probe,
  POLL_OK_MS,
  POLL_OFFLINE_MS,
} from './health';

describe('nextPollDelay', () => {
  it('polls briskly when online and backs off otherwise', () => {
    expect(nextPollDelay('online')).toBe(POLL_OK_MS);
    expect(nextPollDelay('offline')).toBe(POLL_OFFLINE_MS);
    expect(nextPollDelay('unknown')).toBe(POLL_OFFLINE_MS);
    expect(nextPollDelay('degraded')).toBe(POLL_OFFLINE_MS);
  });
});

describe('formatUptime', () => {
  it('formats by magnitude', () => {
    expect(formatUptime(null)).toBe('—');
    expect(formatUptime(42)).toBe('42s');
    expect(formatUptime(120)).toBe('2m');
    expect(formatUptime(7_200)).toBe('2h');
    expect(formatUptime(172_800)).toBe('2d');
  });
});

describe('probe', () => {
  it('maps a successful /v1/health response to online', async () => {
    fetchHealth.mockResolvedValueOnce({
      version: 'v9.9.9',
      uptimeSeconds: 123,
      transports: ['grpc', 'http'],
      toolsAvailable: [],
    });
    const snap = await probe();
    expect(snap.level).toBe('online');
    expect(snap.version).toBe('v9.9.9');
    expect(snap.uptimeSeconds).toBe(123);
    expect(snap.lastSuccessAt).toBeInstanceOf(Date);
  });

  it('collapses any fetch error to offline', async () => {
    fetchHealth.mockRejectedValueOnce(new Error('network down'));
    const snap = await probe();
    expect(snap.level).toBe('offline');
    expect(snap.version).toBeNull();
    expect(snap.uptimeSeconds).toBeNull();
  });
});
