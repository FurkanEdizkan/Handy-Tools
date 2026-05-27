<script lang="ts">
  /* Sidebar — brand, search, two-section nav, mascot pill. */
  import { router, push } from 'svelte-spa-router';
  import { navItems, type NavItem } from './routes';
  import { jobs } from './stores/jobs';
  import { health } from './stores/health';
  import { mascotEnabled, mascotCharacter } from './stores/theme';
  import MascotAvatar from './components/MascotAvatar.svelte';
  import MascotPill from './MascotPill.svelte';

  let search = $state('');

  const counts = $derived.by(() => {
    let running = 0;
    let fail = 0;
    for (const j of $jobs) {
      if (j.status === 'running') running++;
      else if (j.status === 'fail') fail++;
    }
    return { running, fail };
  });

  // Version label shown under the brand. The commit suffix (when present)
  // is how a dev can tell two `make gui` builds apart — without it every
  // local build looks like the identical "v0.0.0-dev".
  const version = $derived.by(() => {
    const v = $health.version ? `v${$health.version}` : 'desktop';
    return $health.commit ? `${v} · ${$health.commit}` : v;
  });

  function visible(section: NavItem['section']): NavItem[] {
    const q = search.trim().toLowerCase();
    return navItems.filter((n) => n.section === section && (q === '' || n.label.toLowerCase().includes(q)));
  }
  const tools = $derived(visible('tools'));
  const system = $derived(visible('system'));
</script>

<aside class="sidebar">
  <div
    class="brand"
    role="button"
    tabindex="0"
    onclick={() => push('/')}
    onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && push('/')}
  >
    <div class="brand-mark">
      <MascotAvatar bare character={$mascotCharacter} size={22} />
    </div>
    <div class="brand-text">
      <div class="name">Handy Tools</div>
      <div class="ver">{version}</div>
    </div>
  </div>

  <div class="sb-search">
    <div class="wrap">
      <span class="icon">⌕</span>
      <input class="input" placeholder="Search tools…" bind:value={search} />
      <span class="kbd">⌘K</span>
    </div>
  </div>

  <nav class="sb-nav">
    {#if tools.length > 0}
      <div class="sb-section">Tools</div>
      {#each tools as n (n.href)}
        <a class="sb-item {n.matches(router.location) ? 'active' : ''}" href={n.href}>
          <span class="glyph">{n.glyph}</span>
          <span class="label">{n.label}</span>
        </a>
      {/each}
    {/if}
    {#if system.length > 0}
      <div class="sb-section">System</div>
      {#each system as n (n.href)}
        <a class="sb-item {n.matches(router.location) ? 'active' : ''}" href={n.href}>
          <span class="glyph">{n.glyph}</span>
          <span class="label">{n.label}</span>
          {#if n.href === '#/jobs' && counts.fail > 0}
            <span class="badge error">{counts.fail}</span>
          {:else if n.href === '#/jobs' && counts.running > 0}
            <span class="badge">{counts.running}</span>
          {/if}
        </a>
      {/each}
    {/if}
  </nav>

  {#if $mascotEnabled}
    <MascotPill />
  {/if}
</aside>
