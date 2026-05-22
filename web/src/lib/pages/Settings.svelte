<script lang="ts">
  import {
    currentTheme,
    setTheme,
    THEMES,
    currentDensity,
    setDensity,
    DENSITIES,
    mascotCharacter,
    mascotEnabled,
    type ThemeName,
    type Density,
    type MascotCharacter,
  } from '../stores/theme';
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
    Appearance
    <div class="desc">Theme and density preferences for this device.</div>
  </div>
  <div class="setting-row">
    <div class="lbl-block">
      <div class="lbl">Theme</div>
      <div class="sub">Forge ships orange-on-dark; Snow swaps the accent cyan; Ember runs warm.</div>
    </div>
    <div class="control">
      <div class="seg">
        {#each THEMES as t (t)}
          <button class={$currentTheme === t ? 'on' : ''} onclick={() => setTheme(t as ThemeName)}>{t}</button>
        {/each}
      </div>
    </div>
  </div>
  <div class="setting-row">
    <div class="lbl-block">
      <div class="lbl">Density</div>
      <div class="sub">Adjusts the header and dock heights for cramped windows.</div>
    </div>
    <div class="control">
      <div class="seg">
        {#each DENSITIES as d (d)}
          <button class={$currentDensity === d ? 'on' : ''} onclick={() => setDensity(d as Density)}>{d}</button>
        {/each}
      </div>
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

<div class="setting-section">
  <div class="s-head">
    Server &amp; safety
    <div class="desc">
      Used when running against htoolsd. The desktop build embeds a loopback server with these
      same roots.
    </div>
  </div>
  <div class="setting-row">
    <div class="lbl-block">
      <div class="lbl">htoolsd address</div>
      <div class="sub">Where the GUI connects when served over the network.</div>
    </div>
    <div class="control">
      <input class="text-input" value="localhost:7777" readonly />
    </div>
  </div>
  <div class="setting-row">
    <div class="lbl-block">
      <div class="lbl">Allow-roots</div>
      <div class="sub">Paths the server accepts inputs from. Empty = reject everything (fail-closed).</div>
    </div>
    <div class="control stretch">
      <div class="allow-roots">
        <span class="root">~/Desktop</span>
        <span class="root">~/Documents</span>
      </div>
    </div>
  </div>
  <div class="setting-row">
    <div class="lbl-block">
      <div class="lbl">Default JPEG quality</div>
      <div class="sub">Used when a tool page doesn't override it.</div>
    </div>
    <div class="control">
      <input class="text-input" style="width:80px;text-align:right" value="90" readonly />
    </div>
  </div>
  <div class="setting-row">
    <div class="lbl-block">
      <div class="lbl">Default PDF DPI</div>
      <div class="sub">Used when rendering PDF pages to images.</div>
    </div>
    <div class="control">
      <input class="text-input" style="width:80px;text-align:right" value="150" readonly />
    </div>
  </div>
</div>
