<script lang="ts">
  interface Props {
    params?: { id?: string };
  }
  let { params }: Props = $props();
</script>

<section>
  <header class="mb-6 flex items-baseline gap-2">
    <h1 class="text-xl font-semibold tracking-tight">Tool</h1>
    <code class="text-xs text-accent font-mono">/{params?.id ?? '?'}</code>
  </header>
  <p class="text-sm text-text-dim">
    Tool detail page placeholder. Real dropzone + options grid lands in #72.
  </p>
</section>
