<script lang="ts">
  /**
   * DownloadResult — reveals a download link once every job of a browser run
   * has finished (#191). It subscribes to each job's progress stream; when all
   * complete cleanly it shows an `<a download>` pointing at the upload
   * workspace's /v1/uploads/{id}/download endpoint. If any job fails it points
   * the user at the Queue panel instead.
   *
   * Only meaningful in browser mode — the desktop build writes output straight
   * to the user's disk, so callers pass an upload id only when one exists.
   */
  import { api, type ProgressEvent } from '../api';

  interface Props {
    uploadId: string;
    jobIds: string[];
  }
  let { uploadId, jobIds }: Props = $props();

  let done = $state(0);
  let failed = $state(false);

  // Re-subscribe whenever a new run replaces uploadId / jobIds. The effect's
  // cleanup aborts the previous run's streams.
  $effect(() => {
    done = 0;
    failed = false;
    const subs = jobIds.map((id) => {
      // The callback runs only on async SSE events, by which point `ac` is
      // initialised — so it can abort its own stream once the job completes.
      const ac: AbortController = api.subscribeJob(id, (e: ProgressEvent) => {
        if (!e.completed) return;
        if (e.error) failed = true;
        done += 1;
        ac.abort(); // job finished — drop the stream, no reconnect/replay
      });
      return ac;
    });
    return () => subs.forEach((ac) => ac.abort());
  });

  let ready = $derived(jobIds.length > 0 && done >= jobIds.length && !failed);
</script>

{#if ready}
  <a class="download" href={api.uploadDownloadUrl(uploadId)} download>
    ⤓ Download result
  </a>
{:else if failed}
  <span class="failed">Conversion failed — see the Queue panel.</span>
{:else if jobIds.length > 0}
  <span class="pending">Converting… the download appears here when it's done.</span>
{/if}

<style>
  .download {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    border-radius: 6px;
    background: var(--color-accent);
    color: var(--color-bg);
    font-size: 13px;
    font-weight: 600;
    padding: 6px 12px;
    text-decoration: none;
  }
  .download:hover {
    opacity: 0.9;
  }
  .failed,
  .pending {
    font-size: 12px;
  }
  .failed {
    color: var(--color-error);
  }
  .pending {
    color: var(--color-text-dim);
  }
</style>
