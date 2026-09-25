<script lang="ts" module>
  /** Before/after preview of a converted picture (IMG-10). */
  export const compare = $state<{ open: boolean; name: string; before: string; after: string; beforeSize: number; afterSize: number }>({
    open: false,
    name: '',
    before: '',
    after: '',
    beforeSize: 0,
    afterSize: 0,
  })

  export function showCompare(name: string, before: string, after: string, beforeSize: number, afterSize: number) {
    Object.assign(compare, { name, before, after, beforeSize, afterSize, open: true })
  }
</script>

<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'
  import { api, imageUrl } from '../lib/api'
  import { bytes } from '../lib/format'

  let dialog: HTMLDialogElement
  let split = $state(50)

  $effect(() => {
    if (compare.open && dialog && !dialog.open) {
      split = 50
      dialog.showModal()
    }
    if (!compare.open && dialog?.open) dialog.close()
  })

  const diff = $derived(compare.beforeSize ? Math.round(((compare.afterSize - compare.beforeSize) / compare.beforeSize) * 100) : 0)
</script>

<dialog bind:this={dialog} onclose={() => (compare.open = false)} onclick={(e) => e.target === dialog && (compare.open = false)}>
  <div class="box">
    <div class="head">
      <b class="ellipsis">{compare.name}</b>
      <span class="sizes">{bytes(compare.beforeSize)} → {bytes(compare.afterSize)} <span class={diff <= 0 ? 'saved' : 'grew'}>{diff <= 0 ? `−${Math.abs(diff)}%` : `+${diff}%`}</span></span>
      <button class="btn" onclick={() => api.revealFile(compare.after)}><Icon name="folderOpen" size={16} />{L('Lihat file', 'Show file')}</button>
      <button class="btn icon" aria-label={L('Tutup', 'Close')} onclick={() => (compare.open = false)}><Icon name="x" size={16} /></button>
    </div>
    {#if compare.open}
      <div class="stage">
        <img class="img" src={imageUrl(compare.after, 1400)} alt={L('Sesudah', 'After')} />
        <div class="before" style="clip-path: inset(0 {100 - split}% 0 0)">
          <img class="img" src={imageUrl(compare.before, 1400)} alt={L('Sebelum', 'Before')} />
        </div>
        <div class="line" style="left: {split}%"></div>
        <span class="tag l">{L('Sebelum', 'Before')}</span>
        <span class="tag r">{L('Sesudah', 'After')}</span>
      </div>
      <input type="range" min="0" max="100" bind:value={split} aria-label={L('Geser pembanding', 'Comparison slider')} />
    {/if}
  </div>
</dialog>

<style>
  dialog {
    padding: 0;
    border: 1px solid var(--border-strong);
    border-radius: 16px;
    background: var(--surface);
    color: var(--text);
    width: min(1100px, 92vw);
    box-shadow: 0 24px 64px rgba(0, 0, 0, 0.55);
  }
  dialog::backdrop {
    background: rgba(5, 6, 8, 0.7);
  }
  .box {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 16px 18px 18px;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .head b {
    flex-grow: 1;
    font-size: 15px;
  }
  .sizes {
    font-size: 13px;
    color: var(--text-2);
    white-space: nowrap;
  }
  .saved {
    color: var(--ok);
    font-weight: 800;
  }
  .grew {
    color: var(--warn);
    font-weight: 800;
  }
  .stage {
    position: relative;
    height: min(64vh, 640px);
    border-radius: 12px;
    overflow: hidden;
    background: repeating-conic-gradient(var(--surface-2) 0% 25%, var(--surface-3) 0% 50%) 50% / 20px 20px;
  }
  .before {
    position: absolute;
    inset: 0;
  }
  .img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: contain;
  }
  .line {
    position: absolute;
    top: 0;
    bottom: 0;
    width: 2px;
    background: var(--accent);
    transform: translateX(-1px);
    pointer-events: none;
  }
  .tag {
    position: absolute;
    top: 10px;
    padding: 3px 8px;
    border-radius: 6px;
    background: rgba(0, 0, 0, 0.6);
    color: #fff;
    font-size: 12px;
    font-weight: 700;
  }
  .tag.l {
    left: 10px;
  }
  .tag.r {
    right: 10px;
  }
  input[type='range'] {
    width: 100%;
    accent-color: var(--accent);
  }
</style>
