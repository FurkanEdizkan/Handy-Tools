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

const POLL_INTERVAL_MS = 5_000;

const initial: HealthSnapshot = {
  level: 'unknown',
  version: null,
  uptimeSeconds: null,
  lastSuccessAt: null,
};

function levelFromStatus(status: string | undefined): HealthLevel {
  switch (status) {
    case 'ok':
    case 'serving':
      return 'online';
    case 'degraded':
      return 'degraded';
    default:
      return 'offline';
  }
}

async function probe(): Promise<HealthSnapshot> {
  try {
    const body = await api.fetchHealth();
    return {
      level: levelFromStatus(body.status),
      version: typeof body.version === 'string' ? body.version : null,
      uptimeSeconds:
        typeof body.uptimeSeconds === 'number' ? body.uptimeSeconds : null,
      lastSuccessAt: new Date(),
    };
  } catch {
    // Network errors and ApiError both collapse to "offline" for the badge —
    // the user only cares whether the backend is reachable.
    return { ...initial, level: 'offline' };
  }
}

export const health: Readable<HealthSnapshot> = readable(initial, (set) => {
  let cancelled = false;

  async function tick(): Promise<void> {
    if (cancelled) return;
    const snap = await probe();
    if (!cancelled) set(snap);
  }

  void tick();
  const handle = setInterval(() => {
    void tick();
  }, POLL_INTERVAL_MS);

  return () => {
    cancelled = true;
    clearInterval(handle);
  };
});

export function formatUptime(seconds: number | null): string {
  if (seconds === null) return '—';
  if (seconds < 60) return `${Math.floor(seconds)}s`;
  if (seconds < 3_600) return `${Math.floor(seconds / 60)}m`;
  if (seconds < 86_400) return `${Math.floor(seconds / 3_600)}h`;
  return `${Math.floor(seconds / 86_400)}d`;
}
