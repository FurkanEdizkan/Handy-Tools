import { readable, type Readable } from 'svelte/store';
import { api } from '../api';

export type HealthLevel = 'online' | 'degraded' | 'offline' | 'unknown';

export interface HealthSnapshot {
  level: HealthLevel;
  /** Backend version string, or null when unknown. */
  version: string | null;
  /** Uptime in seconds, or null when unknown. */
  uptimeSeconds: number | null;
  /** Wall-clock timestamp of the last successful probe. */
  lastSuccessAt: Date | null;
}

/** Poll cadence: brisk while the backend answers, backed off while it doesn't. */
export const POLL_OK_MS = 5_000;
export const POLL_OFFLINE_MS = 30_000;

const initial: HealthSnapshot = {
  level: 'unknown',
  version: null,
  uptimeSeconds: null,
  lastSuccessAt: null,
};

/**
 * nextPollDelay backs the poll off when the backend is unreachable so a dead
 * server isn't hammered every 5s.
 */
export function nextPollDelay(level: HealthLevel): number {
  return level === 'online' ? POLL_OK_MS : POLL_OFFLINE_MS;
}

/**
 * probe hits GET /v1/health once. A successful response is itself the liveness
 * signal — the endpoint carries no status field — so success maps to "online"
 * and any error (network, TLS, non-2xx) collapses to "offline".
 */
export async function probe(): Promise<HealthSnapshot> {
  try {
    const body = await api.fetchHealth();
    return {
      level: 'online',
      version: typeof body.version === 'string' ? body.version : null,
      uptimeSeconds: typeof body.uptimeSeconds === 'number' ? body.uptimeSeconds : null,
      lastSuccessAt: new Date(),
    };
  } catch {
    return { ...initial, level: 'offline' };
  }
}

/**
 * health polls GET /v1/health and exposes the latest snapshot. The poll is
 * self-scheduling: it reschedules after each probe with nextPollDelay, so an
 * offline backend is retried every 30s instead of every 5s.
 */
export const health: Readable<HealthSnapshot> = readable(initial, (set) => {
  let cancelled = false;
  let handle: ReturnType<typeof setTimeout> | undefined;

  async function tick(): Promise<void> {
    if (cancelled) return;
    const snap = await probe();
    if (cancelled) return;
    set(snap);
    handle = setTimeout(() => void tick(), nextPollDelay(snap.level));
  }

  void tick();

  return () => {
    cancelled = true;
    if (handle !== undefined) clearTimeout(handle);
  };
});

export function formatUptime(seconds: number | null): string {
  if (seconds === null) return '—';
  if (seconds < 60) return `${Math.floor(seconds)}s`;
  if (seconds < 3_600) return `${Math.floor(seconds / 60)}m`;
  if (seconds < 86_400) return `${Math.floor(seconds / 3_600)}h`;
  return `${Math.floor(seconds / 86_400)}d`;
}
