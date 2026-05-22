# Handy Tools — Chrome extension

A Chrome (Manifest V3) extension that converts images, PDFs and archives
through a local or hosted [`htoolsd`](../cmd/htoolsd) server. It is a thin
client over the same browser-upload API the web app uses (`POST /v1/uploads`
→ a tool endpoint → `GET /v1/uploads/{id}/download`).

## Running htoolsd

The extension needs an `htoolsd` with the HTTP transport enabled:

```sh
htoolsd --http :8080
```

No `--allow-roots` is required — uploaded files are staged in a sandboxed
server-owned workspace. The default endpoint the extension targets is
`http://127.0.0.1:8080`; change it on the extension's Settings page.

## Build

```sh
npm install
npm run build      # tsc --noEmit && vite build → dist/
npm test           # vitest
```

## Load it in Chrome

1. `npm run build`
2. Open `chrome://extensions`, enable **Developer mode**.
3. **Load unpacked** → select `extension/dist`.

## Icons

`public/icons/icon-{16,32,48,128}.png` are rasterized from the Wrenly mascot
at [`docs/brand/wrenly.svg`](../docs/brand/wrenly.svg). To regenerate after the
SVG changes, render it onto a padded square canvas, e.g.:

```sh
rsvg-convert -w 128 -h 128 --keep-aspect-ratio docs/brand/wrenly.svg \
  -o extension/public/icons/icon-128.png
# …repeat for 16, 32, 48
```
