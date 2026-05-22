<script lang="ts">
  import ToolCard from '../components/ToolCard.svelte';
  import { defaultTools } from '../tools';
</script>

<section>
  <header class="mb-6">
    <h1 class="text-xl font-semibold tracking-tight">Home</h1>
    <p class="text-sm text-text-dim">Pick a tool to get started.</p>
  </header>

  <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
    {#each defaultTools as t (t.id)}
      <ToolCard
        glyph={t.glyph}
        title={t.label}
        description={t.desc}
        href={t.id === 'doctor' ? '#/doctor' : `#/tool/${t.id}`}
      />
    {/each}
  </div>
</section>
