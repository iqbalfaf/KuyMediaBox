<script lang="ts">
  import { onMount } from 'svelte'
  import { L } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'
  import { api } from '../lib/api'
  import { downloadModel, loadModels, modelStore } from '../lib/stores/models.svelte'

  let {
    kind,
    value = $bindable(),
    labels,
  }: { kind: 'whisper' | 'bgremove'; value: string; labels: Record<string, [string, string]> } = $props()

  onMount(() => loadModels(kind))

  const list = $derived(modelStore[kind] ?? [])
  const size = (mb: number) => (mb >= 1000 ? `${(mb / 1000).toLocaleString(undefined, { maximumFractionDigits: 1 })} GB` : `${mb} MB`)
</script>

<div class="models" role="radiogroup" aria-label={L('Model', 'Model')}>
  {#each list as m (m.id)}
    {@const lab = labels[m.id] ?? [m.id, '']}
    <div class="row" class:on={value === m.id}>
      <button role="radio" aria-checked={value === m.id} class="pick" onclick={() => (value = m.id)}>
        <span class="dot"></span>
        <span class="txt">
          <b>{lab[0]}</b>
          <span>{lab[1]}{lab[1] ? ' · ' : ''}{size(m.sizeMB)}</span>
        </span>
      </button>
      {#if m.busy}
        <div class="prog">
          <div class="bar"><div style="width: {m.progress * 100}%"></div></div>
          <button class="icon" aria-label={L('Batalkan', 'Cancel')} title={L('Batalkan', 'Cancel')} onclick={() => api.cancelModel(kind, m.id)}><Icon name="x" size={14} /></button>
        </div>
      {:else if m.installed}
        <span class="ok" title={L('Sudah diunduh', 'Downloaded')}><Icon name="check" size={14} /></span>
        <button class="icon" aria-label={L('Hapus model', 'Delete model')} title={L('Hapus model', 'Delete model')} onclick={() => api.deleteModel(kind, m.id)}><Icon name="trash" size={14} /></button>
      {:else}
        <button class="get" onclick={() => downloadModel(kind, m.id)}><Icon name="download" size={13} />{L('Unduh', 'Get')}</button>
      {/if}
    </div>
    {#if m.error}<p class="err">{m.error}</p>{/if}
  {/each}
</div>

<style>
  .models {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 6px 4px 4px;
    border-radius: 10px;
    border: 1px solid var(--border);
    background: var(--surface-2);
  }
  .row.on {
    border-color: var(--accent);
    background: var(--accent-soft);
  }
  .pick {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px;
    border: 0;
    background: none;
    text-align: left;
    color: var(--text);
  }
  .dot {
    width: 14px;
    height: 14px;
    flex-shrink: 0;
    border-radius: 50%;
    border: 2px solid var(--text-4);
  }
  .on .dot {
    border: 4px solid var(--accent);
  }
  .txt {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .txt b {
    font-size: 13px;
  }
  .txt span {
    font-size: 11px;
    color: var(--text-3);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .get {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    height: 28px;
    padding: 0 10px;
    border-radius: 8px;
    border: 1px solid var(--accent);
    background: none;
    color: var(--accent-text-2);
    font-size: 12px;
    font-weight: 700;
    white-space: nowrap;
  }
  .ok {
    color: var(--ok);
    display: flex;
  }
  .icon {
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 0;
    border-radius: 7px;
    background: none;
    color: var(--text-3);
  }
  .icon:hover {
    background: var(--surface-3);
    color: var(--text);
  }
  .prog {
    display: flex;
    align-items: center;
    gap: 4px;
    width: 96px;
  }
  .prog .bar {
    flex: 1;
  }
  .err {
    margin: 0;
    font-size: 11px;
    color: var(--err);
  }
</style>
