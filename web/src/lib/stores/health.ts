import { readable, type Readable } from 'svelte/store';

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
const REQUEST_TIMEOUT_MS = 3_000;

const initial: HealthSnapshot = {
  level: 'unknown',
  version: null,
  uptimeSeconds: null,
  lastSuccessAt: null,
};

interface RawHealth {
  status?: string;
  version?: string;
  uptime_seconds?: number;
}

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
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS);
  try {
    const res = await fetch('/v1/health', {
      headers: { accept: 'application/json' },
      signal: controller.signal,
    });
    if (!res.ok) {
      return { ...initial, level: 'offline' };
    }
    const body = (await res.json()) as RawHealth;
    return {
      level: levelFromStatus(body.status),
      version: typeof body.version === 'string' ? body.version : null,
      uptimeSeconds:
        typeof body.uptime_seconds === 'number' ? body.uptime_seconds : null,
      lastSuccessAt: new Date(),
    };
  } catch {
    return { ...initial, level: 'offline' };
  } finally {
    clearTimeout(timer);
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
