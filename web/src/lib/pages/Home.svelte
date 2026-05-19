<script lang="ts">
  const tools = [
    { id: 'convert', label: 'Convert images', desc: 'PNG · JPEG · HEIC · WebP' },
    { id: 'archive', label: 'Archive', desc: 'pack or extract zip / tar / 7z' },
    { id: 'pdf', label: 'PDF utilities', desc: 'merge · split · render · extract' },
  ];
</script>

<section>
  <header class="mb-6">
    <h1 class="text-xl font-semibold tracking-tight">Home</h1>
    <p class="text-sm text-text-dim">Pick a tool to get started.</p>
  </header>

  <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
    {#each tools as t}
      <a
        href={`#/tool/${t.id}`}
        class="block rounded-lg border border-border bg-surface p-4 hover:border-accent transition-colors"
      >
        <div class="text-sm font-semibold">{t.label}</div>
        <div class="text-xs text-text-dim mt-1">{t.desc}</div>
      </a>
    {/each}
  </div>
</section>
