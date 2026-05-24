<script lang="ts">
  import Router from 'svelte-spa-router';
  import Layout from './lib/Layout.svelte';
  import { routes } from './lib/routes';
</script>

<Layout>
  <Router {routes} />
</Layout>
