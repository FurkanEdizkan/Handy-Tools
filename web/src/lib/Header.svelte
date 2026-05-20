<script lang="ts">
  import { fly } from 'svelte/transition';
  import { health, formatUptime } from './stores/health';
  import { currentTheme, cycleTheme } from './stores/theme';

  const version = '0.0.0-dev';

  let cogOpen = $state(false);

  function closeOnOutsideClick(node: HTMLElement) {
    function handler(ev: MouseEvent) {
      if (!node.contains(ev.target as Node)) cogOpen = false;
    }
    document.addEventListener('mousedown', handler);
    return {
      destroy() {
        document.removeEventListener('mousedown', handler);
      },
    };
  }
</script>

<header class="topbar">
  <div class="brand">
    <span class="brand-mark" aria-hidden="true">h</span>
    <div class="brand-text">
      <span class="name">Handy Tools</span>
      <span class="ver">v{version}</span>
    </div>
  </div>

  <div class="spacer"></div>

  <button
    type="button"
    class="theme-btn"
    title="Cycle theme (forge → snow → ember)"
    aria-label="Cycle theme"
    onclick={cycleTheme}
  >
    <span class="theme-glyph" aria-hidden="true">◐</span>
    <span class="theme-label">{$currentTheme}</span>
  </button>

  <div
    class="health-badge"
    data-level={$health.level}
    title={`htoolsd · v${$health.version ?? '?'} · uptime ${formatUptime($health.uptimeSeconds)}`}
    aria-label={`Backend status: ${$health.level}`}
  >
    <span class="dot" aria-hidden="true"></span>
    <span class="label">{$health.level}</span>
  </div>

  <div class="cog-wrap" use:closeOnOutsideClick>
    <button
      type="button"
      class="cog-btn"
      title="Open settings"
      aria-label="Open settings"
      aria-expanded={cogOpen}
      onclick={() => (cogOpen = !cogOpen)}
    >
      <span aria-hidden="true">⚙</span>
    </button>

    {#if cogOpen}
      <div class="cog-popover" role="menu" transition:fly={{ y: -6, duration: 120 }}>
        <a href="#/settings" class="popover-item" role="menuitem" onclick={() => (cogOpen = false)}>
          Settings…
        </a>
        <a href="#/doctor" class="popover-item" role="menuitem" onclick={() => (cogOpen = false)}>
          Doctor
        </a>
        <div class="popover-divider"></div>
        <div class="popover-meta">
          <div>backend: <code>{$health.version ?? '—'}</code></div>
          <div>uptime: <code>{formatUptime($health.uptimeSeconds)}</code></div>
        </div>
      </div>
    {/if}
  </div>
</header>

<style>
  .topbar {
    height: 56px;
    flex-shrink: 0;
    padding: 0 18px;
    display: flex;
    align-items: center;
    gap: 10px;
    background: var(--color-surface);
    border-bottom: 1px solid var(--color-border);
    position: relative;
  }

  .brand { display: flex; align-items: center; gap: 10px; }
  .brand-mark {
    width: 28px; height: 28px;
    border-radius: 6px;
    background: var(--color-accent);
    color: var(--color-mascot-eye);
    display: grid; place-items: center;
    font-weight: 700;
    font-size: 13px;
  }
  .brand-text { display: flex; flex-direction: column; line-height: 1.1; }
  .brand-text .name { font-size: 13px; font-weight: 600; color: var(--color-text); }
  .brand-text .ver { font-size: 10px; color: var(--color-text-dim); font-family: ui-monospace, monospace; }

  .spacer { flex: 1; }

  .theme-btn {
    display: inline-flex; align-items: center; gap: 6px;
    padding: 5px 10px;
    border-radius: 6px;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    color: var(--color-text-dim);
    font-family: ui-monospace, monospace;
    font-size: 11px;
    cursor: pointer;
    transition: border-color 0.12s, color 0.12s;
  }
  .theme-btn:hover { color: var(--color-text); border-color: var(--color-accent); }
  .theme-glyph { color: var(--color-accent); }

  .health-badge {
    display: inline-flex; align-items: center; gap: 6px;
    padding: 4px 10px 4px 8px;
    border: 1px solid var(--color-border);
    border-radius: 999px;
    background: var(--color-bg);
    font-family: ui-monospace, monospace;
    font-size: 11px;
    color: var(--color-text-dim);
    cursor: default;
  }
  .health-badge .dot {
    width: 6px; height: 6px; border-radius: 50%;
    background: var(--color-text-dim);
  }
  .health-badge[data-level='online']   .dot { background: var(--color-success); box-shadow: 0 0 6px var(--color-success); }
  .health-badge[data-level='online']   .label { color: var(--color-success); }
  .health-badge[data-level='degraded'] .dot { background: var(--color-warning); box-shadow: 0 0 6px var(--color-warning); }
  .health-badge[data-level='degraded'] .label { color: var(--color-warning); }
  .health-badge[data-level='offline']  .dot { background: var(--color-error);   box-shadow: 0 0 6px var(--color-error); }
  .health-badge[data-level='offline']  .label { color: var(--color-error); }

  .cog-wrap { position: relative; }
  .cog-btn {
    width: 32px; height: 32px;
    display: grid; place-items: center;
    background: transparent;
    border: 1px solid transparent;
    border-radius: 6px;
    color: var(--color-text-dim);
    cursor: pointer;
    font-size: 15px;
    transition: background 0.12s, color 0.12s;
  }
  .cog-btn:hover { background: var(--color-bg); color: var(--color-text); }
  .cog-btn[aria-expanded='true'] {
    background: var(--color-bg);
    color: var(--color-accent);
    border-color: var(--color-border);
  }

  .cog-popover {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    z-index: 20;
    min-width: 200px;
    padding: 4px;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: 8px;
    box-shadow: 0 14px 36px -16px rgba(0,0,0,0.6);
  }
  .popover-item {
    display: block;
    padding: 8px 10px;
    border-radius: 4px;
    color: var(--color-text);
    font-size: 12px;
    text-decoration: none;
  }
  .popover-item:hover { background: var(--color-bg); color: var(--color-accent); }
  .popover-divider { height: 1px; background: var(--color-border); margin: 4px 2px; }
  .popover-meta {
    padding: 4px 10px 6px;
    font-family: ui-monospace, monospace;
    font-size: 10px;
    color: var(--color-text-dim);
    line-height: 1.6;
  }
  .popover-meta code { color: var(--color-text); }
</style>
