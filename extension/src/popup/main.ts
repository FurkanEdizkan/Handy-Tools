/**
 * Popup controller — three tool tabs (Image / PDF / Archive) over the htoolsd
 * HTTP API. It owns the whole conversion flow (upload → job → progress →
 * download) because the MV3 service worker cannot open an SSE stream.
 */

import { HtoolsClient } from '../lib/api';
import { runFlow, resumeFlow, type FlowCallbacks, type StartJob } from '../lib/flow';
import { loadSettings, loadFlowState, saveFlowState } from '../lib/storage';
import type { ImageTargetFormat } from '../lib/types';

const $ = <T extends HTMLElement>(id: string): T => document.getElementById(id) as T;

const els = {
  health: $<HTMLSpanElement>('health'),
  endpoint: $<HTMLSpanElement>('endpoint'),
  tabs: $<HTMLElement>('tabs'),
  run: $<HTMLButtonElement>('run'),
  status: $<HTMLDivElement>('status'),
  bar: $<HTMLElement>('bar'),
  message: $<HTMLParagraphElement>('message'),
  download: $<HTMLAnchorElement>('download'),
  imageFiles: $<HTMLInputElement>('image-files'),
  imageFormat: $<HTMLSelectElement>('image-format'),
  imageQuality: $<HTMLInputElement>('image-quality'),
  pdfFiles: $<HTMLInputElement>('pdf-files'),
  pdfOp: $<HTMLSelectElement>('pdf-op'),
  pdfDpi: $<HTMLInputElement>('pdf-dpi'),
  pdfEvery: $<HTMLInputElement>('pdf-every'),
  archiveFiles: $<HTMLInputElement>('archive-files'),
  archiveOp: $<HTMLSelectElement>('archive-op'),
  archiveFormat: $<HTMLSelectElement>('archive-format'),
};

type Family = 'image' | 'pdf' | 'archive';
let family: Family = 'image';
let client: HtoolsClient;
let busy = false;

const joinPath = (dir: string, name: string): string => dir.replace(/\/+$/, '') + '/' + name;
const stem = (name: string): string => {
  const base = name.slice(Math.max(name.lastIndexOf('/'), name.lastIndexOf('\\')) + 1);
  const dot = base.lastIndexOf('.');
  return dot > 0 ? base.slice(0, dot) : base;
};

const callbacks: FlowCallbacks = {
  onStatus(status, message) {
    els.status.hidden = false;
    if (status === 'uploading') els.message.textContent = 'Uploading…';
    else if (status === 'converting') els.message.textContent = 'Converting…';
    else if (status === 'done') els.message.textContent = 'Done.';
    else if (status === 'error') els.message.textContent = message ?? 'Failed.';
    els.message.classList.toggle('error', status === 'error');
  },
  onProgress(fraction) {
    els.bar.style.width = `${Math.round(fraction * 100)}%`;
  },
};

function setBusy(on: boolean): void {
  busy = on;
  els.run.disabled = on;
}

function showDownload(uploadId: string): void {
  els.status.hidden = false;
  els.bar.style.width = '100%';
  els.download.hidden = false;
  els.download.dataset.url = client.uploadDownloadUrl(uploadId);
}

function showError(err: unknown): void {
  callbacks.onStatus('error', err instanceof Error ? err.message : 'Conversion failed.');
}

/** Builds the StartJob for the active tab from the current form values. */
function startJobFor(fam: Family): StartJob {
  if (fam === 'image') {
    const targetFormat = els.imageFormat.value as ImageTargetFormat;
    const quality = Number(els.imageQuality.value);
    return (sources, outputDir) =>
      client.imageBatchConvert({
        sources: sources.map((s) => ({ path: s.path })),
        targetFormat,
        options: { quality, maxWidth: 0, maxHeight: 0, stripMetadata: false },
        output: { directory: outputDir, overwrite: true },
      });
  }
  if (fam === 'pdf') {
    const op = els.pdfOp.value;
    return (sources, outputDir) => {
      const first = { path: sources[0].path };
      if (op === 'render') {
        return client.pdfToImage({
          source: first,
          pages: { from: 0, to: 0 },
          dpi: Number(els.pdfDpi.value),
          targetFormat: 'PNG',
          output: { directory: outputDir },
        });
      }
      if (op === 'text') {
        return client.pdfToText({
          source: first,
          pages: { from: 0, to: 0 },
          layout: true,
          output: { file: joinPath(outputDir, stem(sources[0].name) + '.txt') },
        });
      }
      if (op === 'split') {
        return client.pdfSplit({
          source: first,
          pageRanges: [],
          everyN: Number(els.pdfEvery.value),
          output: { directory: outputDir },
        });
      }
      return client.pdfMerge({
        sources: sources.map((s) => ({ path: s.path })),
        output: { file: joinPath(outputDir, 'merged.pdf') },
      });
    };
  }
  // archive
  const op = els.archiveOp.value;
  return (sources, outputDir) => {
    if (op === 'extract') {
      return client.archiveExtract({
        source: { path: sources[0].path },
        parts: [],
        destinationDir: outputDir,
        password: '',
        overwrite: true,
        autoAcceptMultiPart: true,
      });
    }
    return client.archiveCompress({
      sources: sources.map((s) => ({ path: s.path })),
      destination: { file: joinPath(outputDir, 'archive.' + els.archiveFormat.value), overwrite: true },
      format: '',
      compressionLevel: 6,
    });
  };
}

function filesFor(fam: Family): File[] {
  const input = fam === 'image' ? els.imageFiles : fam === 'pdf' ? els.pdfFiles : els.archiveFiles;
  return Array.from(input.files ?? []);
}

async function onRun(): Promise<void> {
  if (busy) return;
  const files = filesFor(family);
  if (files.length === 0) {
    callbacks.onStatus('error', 'Pick at least one file.');
    return;
  }
  els.download.hidden = true;
  els.bar.style.width = '0%';
  setBusy(true);
  try {
    const uploadId = await runFlow(client, files, startJobFor(family), callbacks);
    showDownload(uploadId);
  } catch (err) {
    showError(err);
    await saveFlowState({ status: 'error', message: err instanceof Error ? err.message : 'failed' });
  } finally {
    setBusy(false);
  }
}

function selectTab(next: Family): void {
  family = next;
  for (const tab of els.tabs.querySelectorAll<HTMLButtonElement>('.tab')) {
    tab.setAttribute('aria-selected', String(tab.dataset.family === next));
  }
  for (const pane of document.querySelectorAll<HTMLElement>('.pane')) {
    pane.hidden = pane.dataset.family !== next;
  }
}

/** Shows only the option rows relevant to the selected PDF / archive op. */
function syncConditionalRows(): void {
  for (const row of document.querySelectorAll<HTMLElement>('[data-pdf-op]')) {
    row.hidden = row.dataset.pdfOp !== els.pdfOp.value;
  }
  for (const row of document.querySelectorAll<HTMLElement>('[data-archive-op]')) {
    row.hidden = row.dataset.archiveOp !== els.archiveOp.value;
  }
}

async function checkHealth(): Promise<void> {
  try {
    await client.fetchHealth();
    els.health.classList.add('ok');
    els.health.title = 'Server reachable';
  } catch {
    els.health.classList.add('down');
    els.health.title = 'Server unreachable — check Settings';
  }
}

/** Rehydrates an in-flight or finished conversion from a prior popup session. */
async function rehydrate(): Promise<void> {
  const flow = await loadFlowState();
  if (flow.status === 'converting' && flow.jobId && flow.uploadId) {
    setBusy(true);
    try {
      const uploadId = await resumeFlow(client, flow.jobId, flow.uploadId, callbacks);
      showDownload(uploadId);
    } catch (err) {
      showError(err);
    } finally {
      setBusy(false);
    }
  } else if (flow.status === 'done' && flow.uploadId) {
    callbacks.onStatus('done');
    showDownload(flow.uploadId);
  } else if (flow.status === 'error') {
    callbacks.onStatus('error', flow.message);
  }
}

async function init(): Promise<void> {
  const settings = await loadSettings();
  client = new HtoolsClient({ baseUrl: settings.endpoint });
  els.endpoint.textContent = settings.endpoint;

  els.tabs.addEventListener('click', (e) => {
    const tab = (e.target as HTMLElement).closest<HTMLButtonElement>('.tab');
    if (tab?.dataset.family) selectTab(tab.dataset.family as Family);
  });
  els.pdfOp.addEventListener('change', syncConditionalRows);
  els.archiveOp.addEventListener('change', syncConditionalRows);
  els.run.addEventListener('click', () => void onRun());
  els.download.addEventListener('click', (e) => {
    e.preventDefault();
    const url = els.download.dataset.url;
    if (url) void chrome.runtime.sendMessage({ type: 'download', url });
  });
  $<HTMLAnchorElement>('open-options').addEventListener('click', (e) => {
    e.preventDefault();
    chrome.runtime.openOptionsPage();
  });

  selectTab('image');
  syncConditionalRows();
  await Promise.all([checkHealth(), rehydrate()]);
}

void init();
