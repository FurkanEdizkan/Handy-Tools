<script lang="ts">
  import { mascotCharacter, mascotEnabled, type MascotCharacter } from '../stores/theme';
</script>

<div class="page-header">
  <div class="icon-block">⚙</div>
  <div style="flex:1">
    <h1>Settings</h1>
    <div class="desc">
      Stored at <span style="font-family:var(--mono);color:var(--text)">~/.config/handy-tools/config.yaml</span>
    </div>
  </div>
</div>

<div class="setting-section">
  <div class="s-head">
    Mascot
    <div class="desc">The companion in the sidebar — toggle it off or pick an alternate sprite.</div>
  </div>
  <div class="setting-row">
    <div class="lbl-block">
      <div class="lbl">Character</div>
      <div class="sub">Wrenly is the orange panda; Hopper is the lilac rabbit alternate.</div>
    </div>
    <div class="control">
      <div class="seg">
        {#each ['wrenly', 'hopper'] as c (c)}
          <button
            class={$mascotCharacter === c ? 'on' : ''}
            onclick={() => mascotCharacter.set(c as MascotCharacter)}>{c}</button>
        {/each}
      </div>
    </div>
  </div>
  <div class="setting-row">
    <div class="lbl-block">
      <div class="lbl">Show in sidebar</div>
      <div class="sub">When off, the sidebar shows only the brand mark.</div>
    </div>
    <div class="control">
      <button
        class="toggle {$mascotEnabled ? 'on' : ''}"
        aria-label="Show mascot in sidebar"
        onclick={() => mascotEnabled.set(!$mascotEnabled)}
      ></button>
    </div>
  </div>
</div>
