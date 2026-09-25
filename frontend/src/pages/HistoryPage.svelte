<script lang="ts">
  import { L, locale } from '../lib/i18n.svelte'
  import PageHeader from '../components/PageHeader.svelte'
  import Select from '../components/Select.svelte'
  import Icon from '../components/Icon.svelte'
  import { api } from '../lib/api'
  import { showDetail } from '../lib/stores/app.svelte'
  import { clearHistory, history, reloadHistory, removeHistory } from '../lib/stores/history.svelte'
  import { bytes } from '../lib/format'
  import type { HistoryEntry } from '../lib/types'

  let kind = $state<string>('all')
  let status = $state<string>('all')
  let query = $state('')
  let shown = $state(200)
  let confirmClear = $state(false)

  const kindIcon: Record<string, string> = { image: 'image', video: 'video', audio: 'music', download: 'download', pdf: 'fileText' }
  const kindName = $derived<Record<string, string>>({ image: L('Gambar', 'Image'), video: 'Video', audio: 'Audio', download: 'Download', pdf: 'PDF' })

  const filtered = $derived.by(() => {
    const q = query.trim().toLowerCase()
    return history.list.filter((e) => {
      if (kind !== 'all' && e.kind !== kind) return false
      if (status !== 'all' && e.status !== status) return false
      if (q && !`${e.title} ${e.input} ${e.output}`.toLowerCase().includes(q)) return false
      return true
    })
  })
  const totals = $derived.by(() => {
    let inSize = 0
    let outSize = 0
    for (const e of filtered) {
      if (e.status === 'done' && e.inSize && e.outSize) {
        inSize += e.inSize
        outSize += e.outSize
      }
    }
    return { inSize, outSize }
  })

  function when(ms: number): string {
    if (!ms) return '—'
    return new Date(ms).toLocaleString(locale(), { day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' })
  }

  function took(e: HistoryEntry): string {
    if (!e.started || !e.time || e.time < e.started) return ''
    const s = Math.round((e.time - e.started) / 1000)
    if (s < 60) return `${s} ${L('dtk', 's')}`
    return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
  }

  function change(e: HistoryEntry): string {
    if (!e.inSize || !e.outSize || e.kind === 'download') return ''
    const d = Math.round(((e.outSize - e.inSize) / e.inSize) * 100)
    return d <= 0 ? `−${Math.abs(d)}%` : `+${d}%`
  }

  function base(p: string): string {
    return p.split(/[\\/]/).pop() || p
  }

  const statusLabel = $derived<Record<string, string>>({
    done: L('Selesai', 'Done'), failed: L('Gagal', 'Failed'), skipped: L('Dilewati', 'Skipped'), canceled: L('Dibatalkan', 'Canceled'),
  })
  const statusClass: Record<string, string> = { done: 'ok', failed: 'err', skipped: 'muted', canceled: 'muted' }

  async function doClear() {
    if (!confirmClear) {
      confirmClear = true
      setTimeout(() => (confirmClear = false), 4000)
      return
    }
    confirmClear = false
    await clearHistory()
  }
</script>

<PageHeader title={L('Riwayat', 'History')} subtitle={L('Semua tugas yang pernah selesai: tanggal, file, ukuran sebelum/sesudah, dan status.', 'Every finished task: date, files, size before/after and status.')} />

<div class="body">
  <section class="card list" aria-label={L('Riwayat tugas', 'Task history')}>
    <div class="toolbar">
      <div class="search">
        <Icon name="search" size={16} />
        <input class="q" bind:value={query} placeholder={L('Cari nama file…', 'Search file names…')} aria-label={L('Cari', 'Search')} />
      </div>
      <div class="sel">
        <Select
          label={L('Jenis', 'Type')}
          bind:value={kind}
          options={[
            { value: 'all', label: L('Semua jenis', 'All types') }, { value: 'image', label: L('Gambar', 'Images') }, { value: 'video', label: 'Video' },
            { value: 'audio', label: 'Audio' }, { value: 'download', label: 'Download' }, { value: 'pdf', label: 'PDF' },
          ]}
        />
      </div>
      <div class="sel">
        <Select
          label="Status"
          bind:value={status}
          options={[
            { value: 'all', label: L('Semua status', 'Any status') }, { value: 'done', label: L('Berhasil', 'Done') },
            { value: 'failed', label: L('Gagal', 'Failed') }, { value: 'skipped', label: L('Dilewati', 'Skipped') },
            { value: 'canceled', label: L('Dibatalkan', 'Canceled') },
          ]}
        />
      </div>
      <button class="btn icon" title={L('Muat ulang', 'Reload')} aria-label={L('Muat ulang', 'Reload')} onclick={reloadHistory}><Icon name="refresh" size={16} /></button>
      <button class="btn" class:danger={confirmClear} onclick={doClear} disabled={history.list.length === 0}>
        <Icon name="trash" size={16} />{confirmClear ? L('Yakin hapus semua?', 'Delete everything?') : L('Hapus riwayat', 'Clear history')}
      </button>
    </div>

    <div class="summary">
      <span>{filtered.length} {L('tugas', filtered.length === 1 ? 'task' : 'tasks')}</span>
      {#if totals.inSize}
        <span>· {L('Ukuran', 'Size')} {bytes(totals.inSize)} → {bytes(totals.outSize)}
          <b class={totals.outSize <= totals.inSize ? 'saved' : 'grew'}>({totals.outSize <= totals.inSize ? '−' : '+'}{bytes(Math.abs(totals.inSize - totals.outSize))})</b></span>
      {/if}
    </div>

    <div class="head">
      <span>{L('WAKTU', 'TIME')}</span><span>FILE</span><span>{L('HASIL', 'OUTPUT')}</span><span>{L('UKURAN', 'SIZE')}</span><span>STATUS</span><span></span>
    </div>
    <div class="rows">
      {#if !history.loaded}
        <div class="empty"><Icon name="loader" class="spin" /> {L('Memuat…', 'Loading…')}</div>
      {:else if filtered.length === 0}
        <div class="empty">
          <Icon name="history" size={28} />
          <span>{history.list.length ? L('Tidak ada yang cocok dengan filter.', 'Nothing matches the filters.') : L('Belum ada riwayat. Tugas yang selesai akan muncul di sini.', 'No history yet. Finished tasks show up here.')}</span>
        </div>
      {:else}
        {#each filtered.slice(0, shown) as e (e.id)}
          <div class="row">
            <div class="time">
              <span>{when(e.time)}</span>
              {#if took(e)}<span class="sub">{took(e)}</span>{/if}
            </div>
            <div class="file">
              <span class="kic" title={kindName[e.kind]}><Icon name={kindIcon[e.kind] ?? 'file'} size={16} /></span>
              <div class="ft">
                <span class="ellipsis" title={e.input || e.title}>{e.title}</span>
                <span class="sub ellipsis" title={e.input}>{e.input && e.input !== e.title ? e.input : kindName[e.kind]}</span>
              </div>
            </div>
            <div class="out">
              {#if e.output}
                <button class="link ellipsis" title={e.output} onclick={() => api.revealFile(e.output)}>{base(e.output)}</button>
              {:else if e.message}
                <span class="sub ellipsis" title={e.message}>{e.message}</span>
              {:else}
                <span class="sub">—</span>
              {/if}
            </div>
            <div class="size">
              {#if e.inSize || e.outSize}
                <span>{e.inSize ? bytes(e.inSize) : '?'} → {e.outSize ? bytes(e.outSize) : '?'}</span>
                {#if change(e)}<span class="sub {change(e).startsWith('−') ? 'saved' : 'grew'}">{change(e)}</span>{/if}
              {:else}
                <span class="sub">—</span>
              {/if}
            </div>
            <div>
              {#if e.status === 'failed'}
                <button class="pill err as-btn" title={e.message} onclick={() => showDetail(e.title, e.message, e.detail)}>{statusLabel.failed} · {L('detail', 'details')}</button>
              {:else}
                <span class="pill {statusClass[e.status] ?? 'muted'}" title={e.message}>{statusLabel[e.status] ?? e.status}</span>
              {/if}
            </div>
            <button class="mini" title={L('Hapus dari riwayat', 'Remove from history')} aria-label={L('Hapus dari riwayat', 'Remove from history')} onclick={() => removeHistory([e.id])}><Icon name="x" size={14} /></button>
          </div>
        {/each}
        {#if filtered.length > shown}
          <button class="more btn" onclick={() => (shown += 300)}>{L('Tampilkan lebih banyak', 'Show more')} ({filtered.length - shown})</button>
        {/if}
      {/if}
    </div>
  </section>
</div>

<style>
  .body {
    flex-grow: 1;
    min-height: 0;
    display: flex;
    padding: 0 24px 24px 28px;
  }
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
    gap: 10px;
    padding: 14px 16px;
    border-bottom: 1px solid var(--border);
    flex-wrap: wrap;
  }
  .search {
    position: relative;
    display: flex;
    align-items: center;
    gap: 8px;
    height: 36px;
    padding: 0 10px;
    border-radius: 10px;
    border: 1px solid var(--border);
    background: var(--surface-2);
    color: var(--text-3);
    flex: 1 1 200px;
    min-width: 180px;
  }
  .q {
    flex-grow: 1;
    min-width: 0;
    border: 0;
    background: transparent;
    outline: none;
    font-size: 13px;
    color: var(--text);
  }
  .sel {
    width: 170px;
    flex-shrink: 0;
  }
  .summary {
    display: flex;
    gap: 6px;
    padding: 8px 16px;
    font-size: 12px;
    color: var(--text-3);
    border-bottom: 1px solid var(--border-soft);
  }
  .head,
  .row {
    display: grid;
    grid-template-columns: 150px minmax(0, 1.4fr) minmax(0, 1fr) 150px 130px 32px;
    gap: 12px;
    align-items: center;
  }
  .head {
    padding: 8px 16px;
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
    min-height: 54px;
    padding: 4px 8px 4px 16px;
    border-bottom: 1px solid var(--border-soft);
    font-size: 13px;
  }
  .time,
  .size {
    display: flex;
    flex-direction: column;
    gap: 2px;
    font-variant-numeric: tabular-nums;
  }
  .file {
    display: flex;
    align-items: center;
    gap: 10px;
    min-width: 0;
  }
  .kic {
    width: 32px;
    height: 32px;
    flex-shrink: 0;
    border-radius: 8px;
    background: var(--surface-2);
    color: var(--accent-text-2);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .ft,
  .out {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .ft > span:first-child {
    font-weight: 600;
  }
  .sub {
    font-size: 12px;
    color: var(--text-3);
  }
  .saved {
    color: var(--ok);
    font-weight: 700;
  }
  .grew {
    color: var(--warn);
    font-weight: 700;
  }
  .link {
    padding: 0;
    border: 0;
    background: none;
    text-align: left;
    font-size: 13px;
    font-weight: 600;
    color: var(--accent-text-2);
  }
  .link:hover {
    text-decoration: underline;
  }
  .as-btn {
    border: 0;
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
    background: var(--hover);
    color: var(--text);
  }
  .empty {
    height: 100%;
    min-height: 220px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 10px;
    color: var(--text-3);
    font-size: 13px;
  }
  .more {
    margin: 12px auto;
    display: flex;
  }
</style>
