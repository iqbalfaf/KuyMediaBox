<script lang="ts">
  import { L, locale } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'
  import Segmented from './Segmented.svelte'
  import { api } from '../lib/api'
  import { bytes } from '../lib/format'
  import type { HistoryEntry } from '../lib/types'

  let { list, onclose }: { list: HistoryEntry[]; onclose: () => void } = $props()

  const DAY = 86_400_000
  let range = $state<'7' | '30' | '365' | 'all'>('30')

  const kindIcon: Record<string, string> = { image: 'image', video: 'video', audio: 'music', download: 'download', pdf: 'fileText', subtitle: 'subtitles' }
  const kindName = $derived<Record<string, string>>({ image: L('Gambar', 'Images'), video: 'Video', audio: 'Audio', download: 'Download', pdf: 'PDF', subtitle: 'Subtitle' })

  function dayStart(ms: number) {
    const d = new Date(ms)
    d.setHours(0, 0, 0, 0)
    return d.getTime()
  }

  const inRange = $derived.by(() => {
    if (range === 'all') return list
    const from = dayStart(Date.now()) - (Number(range) - 1) * DAY
    return list.filter((e) => e.time >= from)
  })

  // Saved space counts conversions that made files smaller. Downloads have no "before", and
  // subtitles or PDF → text/pictures are new files rather than smaller copies.
  const isPdf = (p: string) => /\.pdf$/i.test(p)
  const comparable = (e: HistoryEntry) => ['image', 'video', 'audio'].includes(e.kind) || (e.kind === 'pdf' && isPdf(e.input) && isPdf(e.output))
  const saving = (e: HistoryEntry) => (e.status === 'done' && comparable(e) && e.inSize > 0 && e.outSize > 0 && e.outSize < e.inSize ? e.inSize - e.outSize : 0)

  const stats = $derived.by(() => {
    let done = 0,
      failed = 0,
      saved = 0,
      downloaded = 0,
      downloads = 0,
      busy = 0
    const kinds: Record<string, { count: number; saved: number; out: number }> = {}
    for (const e of inRange) {
      if (e.status === 'done') done++
      else if (e.status === 'failed') failed++
      if (e.status !== 'done') continue
      const k = (kinds[e.kind] ??= { count: 0, saved: 0, out: 0 })
      k.count++
      k.out += e.outSize || 0
      const s = saving(e)
      k.saved += s
      saved += s
      if (e.kind === 'download') {
        downloads++
        downloaded += e.outSize || 0
      }
      if (e.started && e.time > e.started) busy += e.time - e.started
    }
    const rows = Object.entries(kinds)
      .map(([kind, v]) => ({ kind, ...v }))
      .sort((a, b) => b.count - a.count)
    return { done, failed, saved, downloaded, downloads, busy, rows, rate: done + failed ? Math.round((done / (done + failed)) * 100) : 0 }
  })
  const maxCount = $derived(Math.max(1, ...stats.rows.map((r) => r.count)))

  // Tasks per day (or per month for long ranges).
  const chart = $derived.by(() => {
    const monthly = range === '365' || range === 'all'
    const buckets: { key: number; label: string; done: number; failed: number }[] = []
    const now = new Date()
    if (monthly) {
      let first = new Date(now.getFullYear(), now.getMonth() - 11, 1)
      if (range === 'all' && list.length) {
        const oldest = new Date(Math.min(...list.map((e) => e.time)))
        const o = new Date(oldest.getFullYear(), oldest.getMonth(), 1)
        if (o < first) first = o
      }
      for (let d = new Date(first); d <= now; d = new Date(d.getFullYear(), d.getMonth() + 1, 1)) {
        buckets.push({ key: d.getTime(), label: d.toLocaleDateString(locale(), { month: 'short' }), done: 0, failed: 0 })
      }
    } else {
      const n = Number(range)
      const today = dayStart(Date.now())
      for (let i = n - 1; i >= 0; i--) {
        const t = today - i * DAY
        buckets.push({ key: t, label: new Date(t).toLocaleDateString(locale(), n <= 7 ? { weekday: 'short' } : { day: 'numeric' }), done: 0, failed: 0 })
      }
    }
    const find = (ms: number) => {
      const d = new Date(ms)
      const key = monthly ? new Date(d.getFullYear(), d.getMonth(), 1).getTime() : dayStart(ms)
      return buckets.find((b) => b.key === key)
    }
    for (const e of inRange) {
      const b = find(e.time)
      if (!b) continue
      if (e.status === 'done') b.done++
      else if (e.status === 'failed') b.failed++
    }
    return { buckets, max: Math.max(1, ...buckets.map((b) => b.done + b.failed)) }
  })

  const top = $derived(
    inRange
      .filter((e) => saving(e) > 0)
      .sort((a, b) => saving(b) - saving(a))
      .slice(0, 5),
  )

  function hours(ms: number): string {
    const m = Math.round(ms / 60000)
    if (m < 60) return `${m} ${L('mnt', 'min')}`
    const h = Math.floor(m / 60)
    return `${h} ${L('jam', 'h')} ${m % 60} ${L('mnt', 'min')}`
  }
  const base = (p: string) => p.split(/[\\/]/).pop() || p
  const labelEvery = $derived(Math.ceil(chart.buckets.length / 15))
</script>

<section class="card stats" aria-label={L('Statistik', 'Statistics')}>
  <div class="top">
    <div class="tt">
      <button class="btn icon" onclick={onclose} title={L('Kembali ke daftar', 'Back to the list')} aria-label={L('Kembali ke daftar', 'Back to the list')}><Icon name="arrowLeft" size={16} /></button>
      <b>{L('Statistik', 'Statistics')}</b>
    </div>
    <div class="range">
      <Segmented
        label={L('Rentang waktu', 'Time range')}
        bind:value={range}
        options={[
          { value: '7', label: L('7 hari', '7 days') },
          { value: '30', label: L('30 hari', '30 days') },
          { value: '365', label: L('1 tahun', '1 year') },
          { value: 'all', label: L('Semua', 'All') },
        ]}
      />
    </div>
  </div>

  <div class="scroll">
    <div class="tiles">
      <div class="tile accent">
        <span class="tl">{L('Total hemat', 'Space saved')}</span>
        <span class="tv">{bytes(stats.saved)}</span>
        <span class="ts">{L('dari file yang jadi lebih kecil', 'from files that got smaller')}</span>
      </div>
      <div class="tile">
        <span class="tl">{L('Tugas selesai', 'Tasks done')}</span>
        <span class="tv">{stats.done.toLocaleString(locale())}</span>
        <span class="ts">{stats.done + stats.failed ? `${stats.rate}% ${L('berhasil', 'succeeded')}` : '—'}</span>
      </div>
      <div class="tile">
        <span class="tl">{L('Didownload', 'Downloaded')}</span>
        <span class="tv">{bytes(stats.downloaded)}</span>
        <span class="ts">{stats.downloads.toLocaleString(locale())} {L('file', stats.downloads === 1 ? 'file' : 'files')}</span>
      </div>
      <div class="tile">
        <span class="tl">{L('Waktu proses', 'Processing time')}</span>
        <span class="tv">{hours(stats.busy)}</span>
        <span class="ts">{L('dikerjakan otomatis', 'done for you')}</span>
      </div>
    </div>

    {#if inRange.length === 0}
      <p class="hint center">{L('Belum ada tugas di rentang ini.', 'No tasks in this range yet.')}</p>
    {:else}
      <div class="sec">
        <span class="label">{L('Aktivitas', 'Activity')}</span>
        <div class="chart" role="img" aria-label={L('Jumlah tugas per waktu', 'Tasks over time')}>
          {#each chart.buckets as b, i (b.key)}
            <div class="col" title="{b.label}: {b.done} {L('berhasil', 'done')}{b.failed ? `, ${b.failed} ${L('gagal', 'failed')}` : ''}">
              <div class="bar">
                {#if b.failed}<span class="f" style="height: {(b.failed / chart.max) * 100}%"></span>{/if}
                {#if b.done}<span class="d" style="height: {(b.done / chart.max) * 100}%"></span>{/if}
              </div>
              <span class="cl">{i % labelEvery === 0 ? b.label : ''}</span>
            </div>
          {/each}
        </div>
      </div>

      <div class="grid">
        <div class="sec">
          <span class="label">{L('Per jenis', 'By type')}</span>
          {#each stats.rows as r (r.kind)}
            <div class="krow">
              <span class="kn"><Icon name={kindIcon[r.kind] ?? 'file'} size={15} />{kindName[r.kind] ?? r.kind}</span>
              <div class="track"><span style="width: {(r.count / maxCount) * 100}%"></span></div>
              <span class="kc">{r.count.toLocaleString(locale())}</span>
              <span class="ks">{r.kind === 'download' ? bytes(r.out) : r.saved ? `−${bytes(r.saved)}` : ''}</span>
            </div>
          {/each}
        </div>
        <div class="sec">
          <span class="label">{L('Hemat terbesar', 'Biggest savings')}</span>
          {#if top.length === 0}
            <p class="hint">{L('Belum ada file yang diperkecil.', 'No files were made smaller yet.')}</p>
          {/if}
          {#each top as e (e.id)}
            <button class="trow" title={e.output} onclick={() => e.output && api.revealFile(e.output)}>
              <Icon name={kindIcon[e.kind] ?? 'file'} size={15} />
              <span class="ellipsis">{base(e.input || e.title)}</span>
              <span class="tsz">{bytes(e.inSize)} → {bytes(e.outSize)}</span>
              <b>−{Math.round((saving(e) / e.inSize) * 100)}%</b>
            </button>
          {/each}
        </div>
      </div>
    {/if}
  </div>
</section>

<style>
  .stats {
    flex-grow: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 14px 16px;
    border-bottom: 1px solid var(--border);
  }
  .tt {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .top b {
    font-size: 15px;
  }
  .range {
    width: 360px;
  }
  .scroll {
    flex-grow: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 18px;
    display: flex;
    flex-direction: column;
    gap: 22px;
  }
  .tiles {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 12px;
  }
  .tile {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 14px 16px;
    border-radius: 12px;
    background: var(--canvas);
    border: 1px solid var(--border);
    min-width: 0;
  }
  .tile.accent {
    background: var(--accent-tint);
    border-color: transparent;
  }
  .tl {
    font-size: 12px;
    font-weight: 600;
    color: var(--text-3);
  }
  .tv {
    font-size: 24px;
    font-weight: 800;
    white-space: nowrap;
  }
  .tile.accent .tv {
    color: var(--accent);
  }
  .ts {
    font-size: 12px;
    color: var(--text-3);
  }
  .sec {
    display: flex;
    flex-direction: column;
    gap: 10px;
    min-width: 0;
  }
  .chart {
    display: flex;
    align-items: stretch;
    gap: 3px;
    height: 150px;
  }
  .col {
    flex: 1 1 0;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .bar {
    flex-grow: 1;
    display: flex;
    flex-direction: column-reverse;
    background: var(--canvas);
    border-radius: 4px;
    overflow: hidden;
  }
  .bar .d {
    background: var(--accent);
  }
  .bar .f {
    background: var(--err);
    opacity: 0.8;
  }
  .cl {
    height: 14px;
    font-size: 10px;
    color: var(--text-3);
    text-align: center;
    white-space: nowrap;
    overflow: visible;
  }
  .grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: 24px;
  }
  .krow {
    display: grid;
    grid-template-columns: 110px minmax(0, 1fr) 48px 84px;
    align-items: center;
    gap: 10px;
    font-size: 13px;
  }
  .kn {
    display: flex;
    align-items: center;
    gap: 6px;
    font-weight: 600;
  }
  .track {
    height: 8px;
    border-radius: 4px;
    background: var(--canvas);
    overflow: hidden;
  }
  .track span {
    display: block;
    height: 100%;
    background: var(--accent);
    border-radius: 4px;
  }
  .kc {
    text-align: right;
    font-weight: 700;
  }
  .ks {
    text-align: right;
    font-size: 12px;
    color: var(--ok);
    white-space: nowrap;
  }
  .trow {
    display: grid;
    grid-template-columns: 16px minmax(0, 1fr) auto 44px;
    align-items: center;
    gap: 8px;
    padding: 6px 8px;
    border: 0;
    border-radius: 8px;
    background: none;
    color: inherit;
    font-size: 13px;
    text-align: left;
  }
  .trow:hover {
    background: var(--canvas);
  }
  .tsz {
    font-size: 12px;
    color: var(--text-3);
    white-space: nowrap;
  }
  .trow b {
    color: var(--ok);
    text-align: right;
  }
  .center {
    text-align: center;
  }
</style>
