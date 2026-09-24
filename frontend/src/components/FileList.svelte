<script lang="ts">
  import type { Snippet } from 'svelte'
  import Icon from './Icon.svelte'
  import StatusCell from './StatusCell.svelte'
  import { api } from '../lib/api'
  import { showDetail } from '../lib/stores/app.svelte'
  import type { Converter } from '../lib/stores/converter.svelte'
  import { bytes } from '../lib/format'
  import type { FileItem } from '../lib/types'

  let {
    conv,
    noun,
    formats,
    dropText,
    resultWidth = 176,
    rowHeight = 60,
    meta,
    result,
    thumb,
  }: {
    conv: Converter
    noun: string
    formats: string
    dropText: string
    resultWidth?: number
    rowHeight?: number
    meta: (it: FileItem) => string
    result: Snippet<[FileItem]>
    thumb: Snippet<[FileItem]>
  } = $props()

  const cols = $derived(`minmax(0, 1fr) ${resultWidth}px 128px 64px`)
</script>

<section class="card list" aria-label="Daftar {noun}">
  <div class="toolbar">
    <div class="count">
      <span class="n">{conv.items.length} {noun}</span>
      <span class="size">{bytes(conv.totalSize)} total</span>
      {#if conv.adding}<span class="loading"><Icon name="loader" size={14} class="spin" /> Membaca file…</span>{/if}
    </div>
    <button class="btn" onclick={() => conv.pickFiles()} disabled={conv.adding}><Icon name="plus" size={16} />Tambah file</button>
    <button class="btn" onclick={() => conv.pickFolder()} disabled={conv.adding}><Icon name="folder" size={16} />Tambah folder</button>
    {#if conv.items.some((it) => conv.state(it) === 'done' || conv.state(it) === 'skipped')}
      <button class="btn" onclick={() => conv.clearFinished()} title="Hapus yang sudah selesai dari daftar"><Icon name="check" size={16} />Bersihkan selesai</button>
    {/if}
    <button class="btn icon" aria-label="Kosongkan daftar" title="Kosongkan daftar" onclick={() => conv.clear()}><Icon name="trash" size={16} /></button>
  </div>

  <div class="drop">
    <Icon name="upload" />
    <span class="dt">{dropText}</span>
    <span class="df">{formats}</span>
  </div>

  <div class="head" style="grid-template-columns: {cols}">
    <span>FILE</span><span>HASIL</span><span>STATUS</span><span></span>
  </div>

  <div class="rows">
    {#each conv.items as it (it.id)}
      {@const st = conv.state(it)}
      {@const task = conv.task(it)}
      <div class="row" class:active={st === 'running'} class:dim={st === 'canceled'} style="grid-template-columns: {cols}; height: {rowHeight}px">
        <div class="file">
          {#if it.error}
            {@const t = { bg: '#3A1C1E', fg: '#FF8A8A' }}
            <div class="tile" style="background: {t.bg}; color: {t.fg}"><Icon name="alert" /></div>
          {:else}
            {@render thumb(it)}
          {/if}
          <div class="fname">
            <span class="name ellipsis" title={it.path}>{it.name}</span>
            <span class="meta ellipsis">{meta(it)}</span>
          </div>
        </div>
        <div class="result">
          {#if it.error}
            <span class="bad">{it.error}</span>
          {:else if st === 'failed' && task}
            <span class="bad ellipsis" title={task.message}>{task.message || 'Gagal'}</span>
            <button class="link" onclick={() => showDetail(it.name, task.message, task.detail)}>Lihat detail</button>
          {:else if st === 'skipped' && task}
            <span class="r1">{task.message}</span>
            {#if task.output}<button class="link" onclick={() => api.revealFile(task.output)}>Lihat file</button>{/if}
          {:else}
            {@render result(it)}
          {/if}
        </div>
        <div><StatusCell state={st} {task} /></div>
        <div class="actions">
          {#if st === 'done' && task?.output}
            <button class="mini" aria-label="Tampilkan file hasil" title="Tampilkan file hasil" onclick={() => api.revealFile(task.output)}><Icon name="folderOpen" size={16} /></button>
          {/if}
          <button
            class="mini"
            aria-label={st === 'running' || st === 'queued' ? `Batalkan ${it.name}` : `Hapus ${it.name} dari daftar`}
            title={st === 'running' || st === 'queued' ? 'Batalkan' : 'Hapus dari daftar'}
            onclick={() => conv.remove(it)}><Icon name="x" size={16} /></button
          >
        </div>
      </div>
    {/each}
  </div>
</section>

<style>
  .list {
    flex-grow: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .toolbar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 14px 16px;
    border-bottom: 1px solid var(--border);
  }
  .count {
    flex-grow: 1;
    display: flex;
    align-items: baseline;
    gap: 10px;
    min-width: 0;
  }
  .n {
    font-size: 15px;
    font-weight: 700;
    white-space: nowrap;
  }
  .size {
    font-size: 13px;
    color: var(--text-3);
    white-space: nowrap;
  }
  .loading {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--accent-text-2);
    align-self: center;
  }
  .drop {
    margin: 12px 16px 8px;
    min-height: 48px;
    border: 1.5px dashed var(--border-strong);
    border-radius: 12px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 10px;
    padding: 0 12px;
    font-size: 13px;
    color: var(--accent);
    flex-shrink: 0;
  }
  .dt {
    font-weight: 600;
    color: var(--text-4);
    white-space: nowrap;
  }
  .df {
    color: var(--text-3);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .head {
    display: grid;
    gap: 12px;
    padding: 6px 16px 8px;
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.06em;
    color: var(--text-3);
    border-bottom: 1px solid var(--border-soft);
  }
  .rows {
    flex-grow: 1;
    min-height: 0;
    overflow-y: auto;
  }
  .row {
    display: grid;
    align-items: center;
    gap: 12px;
    padding: 0 12px 0 16px;
    border-bottom: 1px solid var(--border-soft);
  }
  .row.active {
    background: #1c2029;
  }
  .row.dim .file {
    opacity: 0.55;
  }
  .file {
    display: flex;
    align-items: center;
    gap: 12px;
    min-width: 0;
  }
  .tile {
    width: 40px;
    height: 40px;
    flex-shrink: 0;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .fname {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .name {
    font-size: 14px;
    font-weight: 600;
  }
  .meta {
    font-size: 12px;
    color: var(--text-3);
  }
  .result {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .result :global(.r1) {
    font-size: 13px;
    font-weight: 600;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .result :global(.r2) {
    font-size: 12px;
    color: var(--text-3);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .result :global(.saved) {
    color: var(--ok);
    font-weight: 700;
  }
  .result :global(.grew) {
    color: var(--warn);
    font-weight: 700;
  }
  .bad {
    font-size: 13px;
    font-weight: 600;
    color: var(--err);
  }
  .link {
    align-self: flex-start;
    padding: 0;
    border: 0;
    background: none;
    font-size: 12px;
    font-weight: 600;
    color: var(--accent-text-2);
  }
  .link:hover {
    color: var(--accent-text);
    text-decoration: underline;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 2px;
  }
  .mini {
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--text-3);
  }
  .mini:hover {
    background: var(--surface-2);
    color: var(--text);
  }
  :global(.kmb-tile) {
    width: 40px;
    height: 40px;
    flex-shrink: 0;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
  }
</style>
