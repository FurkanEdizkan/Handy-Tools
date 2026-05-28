<script lang="ts">
  /* Route dispatcher for /tool/:id. Each tool component renders its own
     page-header; this just picks the right one. */
  import { toolById } from '../tools';
  import ToolImage from './ToolImage.svelte';
  import ToolPdf from './ToolPdf.svelte';
  import ToolArchivePack from './ToolArchivePack.svelte';
  import ToolArchiveExtract from './ToolArchiveExtract.svelte';
  import ToolDiffTree from './ToolDiffTree.svelte';
  import ToolRename from './ToolRename.svelte';

  interface Props {
    params?: { id?: string };
  }
  let { params }: Props = $props();
  let tool = $derived(toolById(params?.id ?? ''));
</script>

{#if tool && tool.id === 'convert-image'}
  <ToolImage />
{:else if tool && tool.id === 'pdf'}
  <ToolPdf />
{:else if tool && tool.id === 'zip-pack'}
  <ToolArchivePack />
{:else if tool && tool.id === 'archive-extract'}
  <ToolArchiveExtract />
{:else if tool && tool.id === 'diff-tree'}
  <ToolDiffTree />
{:else if tool && tool.id === 'rename'}
  <ToolRename />
{:else}
  <div class="page-header">
    <div class="icon-block">⚠</div>
    <div style="flex:1">
      <h1>Unknown tool</h1>
      <div class="desc">No tool with id “{params?.id ?? ''}”.</div>
    </div>
    <div class="actions"><a class="btn ghost" href="#/">← Home</a></div>
  </div>
{/if}
