/**
 * Typed wrappers over chrome.storage.
 *
 * - Settings (endpoint) live in chrome.storage.local — they persist.
 * - FlowState lives in chrome.storage.session — it survives a popup close
 *   within the browser session, so a re-opened popup can rehydrate an
 *   in-flight conversion.
 */

export const DEFAULT_ENDPOINT = 'http://127.0.0.1:8080';

export interface Settings {
  endpoint: string;
}

export async function loadSettings(): Promise<Settings> {
  const got = await chrome.storage.local.get('endpoint');
  const endpoint = typeof got.endpoint === 'string' && got.endpoint ? got.endpoint : DEFAULT_ENDPOINT;
  return { endpoint };
}

export async function saveSettings(s: Settings): Promise<void> {
  await chrome.storage.local.set({ endpoint: s.endpoint });
}

export type FlowStatus = 'idle' | 'uploading' | 'converting' | 'done' | 'error';

export interface FlowState {
  status: FlowStatus;
  uploadId?: string;
  jobId?: string;
  message?: string;
}

export async function loadFlowState(): Promise<FlowState> {
  const got = await chrome.storage.session.get('flow');
  return (got.flow as FlowState | undefined) ?? { status: 'idle' };
}

export async function saveFlowState(s: FlowState): Promise<void> {
  await chrome.storage.session.set({ flow: s });
}
