/**
 * flow.ts — orchestrates one conversion: upload the picked files, start a
 * tool job against the staged paths, and follow the job to completion.
 *
 * The flow runs in the popup (it needs EventSource, which an MV3 service
 * worker lacks). Every state transition is mirrored into chrome.storage.session
 * so a re-opened popup can rehydrate via resumeFlow.
 */

import type { HtoolsClient } from './api';
import type { JobResponse, UploadedFile } from './types';
import { saveFlowState, type FlowStatus } from './storage';

export interface FlowCallbacks {
  onStatus: (status: FlowStatus, message?: string) => void;
  onProgress: (fraction: number) => void;
}

/** Builds the tool job from staged upload paths — one per tool family. */
export type StartJob = (sources: UploadedFile[], outputDir: string) => Promise<JobResponse>;

/**
 * runFlow uploads files, starts the job and resolves with the uploadId once
 * the job completes. It rejects if the upload, the job start, or the job
 * itself fails.
 */
export async function runFlow(
  client: HtoolsClient,
  files: File[],
  startJob: StartJob,
  cb: FlowCallbacks,
): Promise<string> {
  cb.onStatus('uploading');
  await saveFlowState({ status: 'uploading' });

  const up = await client.uploadFiles(files);
  cb.onStatus('converting');
  await saveFlowState({ status: 'converting', uploadId: up.uploadId });

  const job = await startJob(up.files, up.outputDir);
  await saveFlowState({ status: 'converting', uploadId: up.uploadId, jobId: job.jobId });

  await followJob(client, job.jobId, cb);
  await saveFlowState({ status: 'done', uploadId: up.uploadId, jobId: job.jobId });
  cb.onStatus('done');
  return up.uploadId;
}

/**
 * resumeFlow re-attaches to a job that a previous popup session started.
 * Returns the uploadId so the caller can re-offer the download.
 */
export async function resumeFlow(
  client: HtoolsClient,
  jobId: string,
  uploadId: string,
  cb: FlowCallbacks,
): Promise<string> {
  cb.onStatus('converting');
  await followJob(client, jobId, cb);
  await saveFlowState({ status: 'done', uploadId, jobId });
  cb.onStatus('done');
  return uploadId;
}

/**
 * followJob resolves when the job reaches a clean terminal event and rejects
 * on a job error. It listens on the SSE stream; if the stream errors (server
 * unreachable, proxy drop) it falls back to polling GET /v1/jobs.
 */
function followJob(client: HtoolsClient, jobId: string, cb: FlowCallbacks): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    let settled = false;
    let pollTimer: ReturnType<typeof setInterval> | undefined;

    const finish = (err?: Error) => {
      if (settled) return;
      settled = true;
      ac.abort();
      if (pollTimer) clearInterval(pollTimer);
      if (err) reject(err);
      else resolve();
    };

    const ac = client.subscribeJob(
      jobId,
      (e) => {
        if (typeof e.fraction === 'number') cb.onProgress(e.fraction);
        if (!e.completed) return;
        finish(e.error ? new Error(e.error.message) : undefined);
      },
      () => startPolling(),
    );

    const startPolling = () => {
      if (settled || pollTimer) return;
      pollTimer = setInterval(() => {
        client
          .fetchJobs()
          .then((res) => {
            const job = res.jobs.find((j) => j.jobId === jobId);
            if (!job || !job.completed) return;
            finish(job.error ? new Error(job.error.message) : undefined);
          })
          .catch(() => {
            /* keep polling — a transient failure is not terminal */
          });
      }, 1500);
    };
  });
}
