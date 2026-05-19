import { readable, type Readable } from 'svelte/store';

export type ConnectionStatus = 'online' | 'offline' | 'unknown';

/**
 * Placeholder connection store. Real polling against /v1/health lands in #94
 * once that endpoint exists (#64); for now we report `unknown` so the badge
 * renders something sensible without faking up state.
 */
export const connectionStatus: Readable<ConnectionStatus> = readable('unknown');
