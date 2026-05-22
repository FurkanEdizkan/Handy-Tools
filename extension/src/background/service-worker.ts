/**
 * Background service worker.
 *
 * Its only job is to start result downloads via chrome.downloads. The popup
 * posts a {type:'download', url} message; routing the download through the
 * worker (which outlives the popup) keeps it robust if the popup closes the
 * instant the user clicks. The worker has no EventSource and never touches
 * the conversion flow — that all runs in the popup (see src/lib/flow.ts).
 */

interface DownloadMessage {
  type: 'download';
  url: string;
}

function isDownloadMessage(m: unknown): m is DownloadMessage {
  return (
    !!m &&
    typeof m === 'object' &&
    (m as DownloadMessage).type === 'download' &&
    typeof (m as DownloadMessage).url === 'string'
  );
}

chrome.runtime.onMessage.addListener((msg: unknown, _sender, sendResponse) => {
  if (!isDownloadMessage(msg)) return false;
  chrome.downloads.download({ url: msg.url }).then(
    (id) => sendResponse({ ok: true, id }),
    (err: unknown) => sendResponse({ ok: false, error: String(err) }),
  );
  return true; // keep the message channel open for the async response
});
