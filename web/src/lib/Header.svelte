<script lang="ts">
  import { connectionStatus } from './stores/connection';

  const version = '0.0.0-dev';

  const statusLabel: Record<'online' | 'offline' | 'unknown', string> = {
    online: 'connected',
    offline: 'offline',
    unknown: 'connecting…',
  };
</script>

<header
  class="h-14 flex items-center gap-3 px-5 border-b border-border bg-surface flex-shrink-0"
>
  <div class="flex items-center gap-2">
    <span
      class="w-7 h-7 rounded-md grid place-items-center text-mascot-eye font-bold"
      style="background: var(--color-accent);"
      aria-hidden="true"
    >
      h
    </span>
    <div class="leading-tight">
      <div class="text-sm font-semibold tracking-tight">Handy Tools</div>
      <div class="text-[10px] text-text-dim font-mono">v{version}</div>
    </div>
  </div>

  <div class="flex-1"></div>

  <div
    class="flex items-center gap-2 px-2.5 py-1 rounded-full border border-border bg-bg font-mono text-[11px]"
    title="htoolsd connection"
  >
    <span
      class="w-1.5 h-1.5 rounded-full"
      class:bg-success={$connectionStatus === 'online'}
      class:bg-error={$connectionStatus === 'offline'}
      class:bg-text-dim={$connectionStatus === 'unknown'}
    ></span>
    <span class="text-text-dim">{statusLabel[$connectionStatus]}</span>
  </div>
</header>
