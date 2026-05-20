<script lang="ts">
  import { toolById } from '../tools';
  import ToolImage from './ToolImage.svelte';
  import ToolPdf from './ToolPdf.svelte';

  interface Props {
    params?: { id?: string };
  }
  let { params }: Props = $props();

  // Dispatch on the :id route param. ToolImage + ToolPdf land here (#130);
  // the archive forms (#131) and Doctor (#132) still fall through to the
  // placeholder until those sub-issues land.
  let tool = $derived(toolById(params?.id ?? ''));
</script>

<section>
  {#if tool}
    <header class="mb-6 flex items-baseline gap-2">
      <span class="text-xl text-accent" aria-hidden="true">{tool.glyph}</span>
      <h1 class="text-xl font-semibold tracking-tight">{tool.label}</h1>
      <code class="text-xs text-accent font-mono">/{tool.id}</code>
    </header>

    {#if tool.id === 'convert-image'}
      <ToolImage />
    {:else if tool.id === 'pdf'}
      <ToolPdf />
    {:else}
      <p class="text-sm text-text-dim">{tool.desc}</p>
      <p class="mt-4 text-sm text-text-dim">
        The tool form for <code class="text-accent">{tool.id}</code> arrives in
        #131–#132.
      </p>
    {/if}
  {:else}
    <header class="mb-6">
      <h1 class="text-xl font-semibold tracking-tight">Unknown tool</h1>
    </header>
    <p class="text-sm text-text-dim">
      No tool with id <code class="text-accent">{params?.id ?? ''}</code>.
      <a class="text-accent underline" href="#/">Back to Home</a>
    </p>
  {/if}
</section>
