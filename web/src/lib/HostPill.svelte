<script lang="ts">
  /* Read-only topbar pill showing the detected host: the Wails desktop build
     vs. the SPA served by htoolsd in a browser. */
  import { isDesktop } from './native';

  const desktop = isDesktop();
</script>

<div class="host-pill {desktop ? 'desktop' : 'server'}" title={desktop ? 'Running as the Wails desktop app' : 'Served by htoolsd'}>
  <span class="dot"></span>
  <span class="kind">{desktop ? 'Desktop' : 'Server'}</span>
  <span style="color:var(--text-mute)">·</span>
  <span>{desktop ? 'Wails' : 'htoolsd'}</span>
</div>
