/**
 * Options page — configure the htoolsd endpoint.
 *
 * localhost / 127.0.0.1 are covered by the manifest's host_permissions; a
 * hosted https endpoint needs a runtime permission grant from
 * optional_host_permissions, requested here on Save.
 */

import { HtoolsClient } from '../lib/api';
import { loadSettings, saveSettings, DEFAULT_ENDPOINT } from '../lib/storage';

const endpointInput = document.getElementById('endpoint') as HTMLInputElement;
const result = document.getElementById('result') as HTMLParagraphElement;

function setResult(text: string, kind: 'ok' | 'error' | ''): void {
  result.textContent = text;
  result.className = 'result' + (kind ? ' ' + kind : '');
}

function parseEndpoint(raw: string): URL | null {
  try {
    const u = new URL(raw);
    if (u.protocol !== 'http:' && u.protocol !== 'https:') return null;
    return u;
  } catch {
    return null;
  }
}

/** True once the extension may reach the endpoint's host. localhost is always
 *  permitted by the manifest; other hosts need a runtime grant. */
async function ensurePermission(u: URL): Promise<boolean> {
  if (u.hostname === 'localhost' || u.hostname === '127.0.0.1') return true;
  const origins = [`${u.protocol}//${u.hostname}/*`];
  if (await chrome.permissions.contains({ origins })) return true;
  return chrome.permissions.request({ origins });
}

async function onSave(): Promise<void> {
  const raw = endpointInput.value.trim() || DEFAULT_ENDPOINT;
  const u = parseEndpoint(raw);
  if (!u) {
    setResult('Enter a valid http(s) URL.', 'error');
    return;
  }
  if (!(await ensurePermission(u))) {
    setResult('Permission to reach that host was denied.', 'error');
    return;
  }
  await saveSettings({ endpoint: raw });
  setResult('Saved.', 'ok');
}

async function onCheck(): Promise<void> {
  const raw = endpointInput.value.trim() || DEFAULT_ENDPOINT;
  const u = parseEndpoint(raw);
  if (!u) {
    setResult('Enter a valid http(s) URL.', 'error');
    return;
  }
  setResult('Checking…', '');
  try {
    const health = await new HtoolsClient({ baseUrl: raw }).fetchHealth();
    setResult(`Connected — htoolsd ${health.version}.`, 'ok');
  } catch {
    setResult('Could not reach htoolsd. Is it running with --http?', 'error');
  }
}

async function init(): Promise<void> {
  const settings = await loadSettings();
  endpointInput.value = settings.endpoint;
  endpointInput.placeholder = DEFAULT_ENDPOINT;
  document.getElementById('save')?.addEventListener('click', () => void onSave());
  document.getElementById('check')?.addEventListener('click', () => void onCheck());
}

void init();
