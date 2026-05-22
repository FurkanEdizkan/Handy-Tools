<script lang="ts">
  /* Sidebar mascot card — a compact avatar plus a per-state speech line.
     Clicking it swaps the character (a playful affordance, mirrored in
     Settings → Mascot → Character). */
  import MascotAvatar from './components/MascotAvatar.svelte';
  import { mascotMood } from './stores/mascot';
  import { mascotCharacter } from './stores/theme';

  const name = $derived($mascotCharacter === 'wrenly' ? 'Wrenly' : 'Hopper');

  function swap(): void {
    mascotCharacter.set($mascotCharacter === 'wrenly' ? 'hopper' : 'wrenly');
  }
</script>

<button class="mascot-pill" onclick={swap} title="Click to swap character">
  <span class="avatar">
    <MascotAvatar character={$mascotCharacter} state={$mascotMood.state} size={40} />
  </span>
  <span class="text">
    <span class="name">
      <span class="state-dot {$mascotMood.state}"></span>
      {name}
      <span class="mstate">{$mascotMood.state}</span>
    </span>
    <span class="speech">{$mascotMood.speech}</span>
  </span>
</button>
