<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'
  import { runtime } from '../lib/api'

  let { title, subtitle, back, backLabel = '' }: { title: string; subtitle: string; back?: () => void; backLabel?: string } = $props()
</script>

<header class="drag">
  {#if back}
    <button class="back no-drag" onclick={back} aria-label={backLabel || L('Kembali', 'Back')} title={backLabel || L('Kembali', 'Back')}><Icon name="arrowLeft" size={18} /></button>
  {/if}
  <div class="text">
    <h1>{title}</h1>
    <p>{subtitle}</p>
  </div>
  <div class="controls no-drag">
    <button aria-label={L('Perkecil jendela', 'Minimize window')} onclick={() => runtime.minimise()}><Icon name="minus" size={16} /></button>
    <button aria-label={L('Perbesar jendela', 'Maximize window')} onclick={() => runtime.toggleMaximise()}><Icon name="square" size={14} /></button>
    <button class="close" aria-label={L('Tutup aplikasi', 'Close app')} onclick={() => runtime.quit()}><Icon name="x" size={16} /></button>
  </div>
</header>

<style>
  header {
    height: 76px;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 0 12px 0 28px;
  }
  .back {
    width: 38px;
    height: 38px;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 10px;
    border: 1px solid var(--border);
    background: var(--surface-2);
    color: var(--text-2);
  }
  .back:hover {
    color: var(--text);
    border-color: var(--border-strong);
  }
  .text {
    flex-grow: 1;
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }
  h1 {
    margin: 0;
    font-size: 22px;
    font-weight: 800;
    letter-spacing: -0.01em;
  }
  p {
    margin: 0;
    font-size: 13px;
    color: var(--text-2);
  }
  .controls {
    display: flex;
    gap: 2px;
    align-self: flex-start;
    margin-top: 10px;
  }
  .controls button {
    width: 40px;
    height: 32px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--text-3);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .controls button:hover {
    background: var(--surface-2);
    color: var(--text);
  }
  .controls .close:hover {
    background: #c42b1c;
    color: #fff;
  }
</style>
