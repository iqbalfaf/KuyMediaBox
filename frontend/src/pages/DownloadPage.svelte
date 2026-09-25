<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import PageHeader from '../components/PageHeader.svelte'
  import Chips from '../components/Chips.svelte'
  import Segmented from '../components/Segmented.svelte'
  import Switch from '../components/Switch.svelte'
  import RunFooter from '../components/RunFooter.svelte'
  import AudioDownloadOptions from '../components/AudioDownloadOptions.svelte'
  import ToolBanner from '../components/ToolBanner.svelte'
  import Icon from '../components/Icon.svelte'
  import OutputPicker from '../components/OutputPicker.svelte'
  import PresetBar from '../components/PresetBar.svelte'
  import { askText } from '../lib/stores/prompt.svelte'
  import { sendToConverter } from '../lib/stores/open.svelte'
  import { api, errText, runtime } from '../lib/api'
  import { hasTool, settings, showDetail, toast } from '../lib/stores/app.svelte'
  import {
    activeCategory, addLinks, analyze, applyRange, applyScope, cancelAll, categories, categoryOf, clearCategory, dl, isSocial,
    kindSummary, optsOf, pendingCount, rememberOpts, removeRow, rowOpts, rowsOf, runningCount, selectable, selectedEntries,
    setAll, setAllInCategory, sourceClass, sourceIcon, sourceName, startAll, tabLabel, toQueue, typeLabel, visible,
    type Category, type LinkRow,
  } from '../lib/stores/download.svelte'
  import { isActive, tasks } from '../lib/stores/tasks.svelte'
  import { duration, initials, longDuration, parseRange, pct, tile, ymd } from '../lib/format'
  import type { Entry } from '../lib/types'

  const cats = $derived(categories())
  const cat = $derived(activeCategory())
  const catRows = $derived(rowsOf(cat))
  const opts = $derived(cat ? optsOf(cat) : null)
  const social = $derived(!!cat && isSocial(cat))
  const readyRows = $derived(catRows.filter((r) => r.status === 'ready' && r.col))
  const catEntries = $derived(readyRows.flatMap((r) => visible(r).map((e) => ({ r, e }))))
  const catSelectable = $derived(catEntries.filter(({ r, e }) => selectable(e, r)).length)
  const catSelected = $derived(readyRows.reduce((n, r) => n + selectedEntries(r).length, 0))
  const catArchived = $derived(opts?.skipExisting ? catEntries.filter(({ e }) => e.archived).length : 0)
  const kinds = $derived.by(() => {
    const k = { video: false, audio: false, image: false }
    for (const { e } of catEntries) k[e.kind === 'image' ? 'image' : e.kind === 'audio' ? 'audio' : 'video'] = true
    return k
  })
  const hasPlaylist = $derived(readyRows.some((r) => r.col!.type === 'playlist'))
  const hasList = $derived(readyRows.some((r) => r.col!.type === 'album' || r.col!.type === 'playlist' || r.col!.type === 'artist'))
  const catOutputs = $derived.by(() => {
    const out: string[] = []
    for (const r of catRows) for (const tid of Object.values(r.taskOf)) if (tasks[tid]?.status === 'done' && tasks[tid].output) out.push(tasks[tid].output)
    return out
  })

  const dlBuiltins = $derived(
    cat === 'spotify'
      ? [
          { id: 'b-sp-mp3', name: 'MP3 320', value: { audioFormat: 'mp3', audioQuality: '320', playlist: true } },
          { id: 'b-sp-m4a', name: 'M4A 256', value: { audioFormat: 'm4a', audioQuality: '256', playlist: true } },
        ]
      : [
          { id: 'b-1080', name: L('Video 1080p MP4', 'Video 1080p MP4'), value: { mode: 'video', quality: '1080', container: 'mp4' } },
          { id: 'b-720', name: L('Video 720p hemat', 'Video 720p (small)'), value: { mode: 'video', quality: '720', container: 'mp4' } },
          { id: 'b-mp3', name: L('Audio MP3 terbaik', 'Best MP3 audio'), value: { mode: 'audio', audioFormat: 'mp3', audioQuality: 'auto' } },
          { id: 'b-sub', name: L('Video + subtitle ID/EN', 'Video + ID/EN subtitles'), value: { mode: 'video', subtitles: 'embed', subLangs: 'id,en' } },
        ],
  )

  async function setCut(r: LinkRow) {
    const v = await askText(
      L('Unduh sebagian', 'Download a part'),
      L('Rentang waktu (mulai-selesai). Kosongkan selesai untuk sampai akhir.', 'Time range (start-end). Leave the end empty for "to the end".'),
      r.cut || '0:00-1:00',
      '1:30-2:45',
      L('Pakai', 'Use'),
    )
    if (v === null) return
    const [a, b = ''] = v.split('-').map((x) => x.trim())
    const ok = (t: string) => t === '' || /^\d+(:[0-5]?\d){0,2}([.,]\d+)?$/.test(t)
    if (!ok(a) || !ok(b)) {
      toast(L('Format tidak valid. Contoh: 1:30-2:45', 'Invalid format. Example: 1:30-2:45'), 'err')
      return
    }
    r.cut = `${a}-${b}`
  }

  async function setSource(r: LinkRow, e: Entry) {
    if (!r.col) return
    const v = await askText(
      L('Pilih lagu YouTube sendiri', 'Pick the YouTube song yourself'),
      L(`Link YouTube untuk "${e.artist} - ${e.title}". Kosongkan untuk pencocokan otomatis.`, `YouTube link for "${e.artist} - ${e.title}". Leave empty for automatic matching.`),
      e.source,
      'https://www.youtube.com/watch?v=…',
      L('Pakai link ini', 'Use this link'),
    )
    if (v === null) return
    try {
      const updated = await api.setEntrySource(r.col.key, e.id, v === '-' ? '' : v)
      e.source = updated.source
      toast(updated.source ? L('Lagu ini akan diunduh dari link pilihan Anda', 'This song will be downloaded from your link') : L('Kembali ke pencocokan otomatis', 'Back to automatic matching'), 'ok')
    } catch (err) {
      toast(errText(err), 'err')
    }
  }

  const hasSpotify = $derived(dl.rows.some((r) => r.link.source === 'spotify'))
  // gallery-dl only matters for TikTok/Facebook photo posts.
  const needsGallery = $derived(
    dl.rows.some((r) => (r.link.photo && (r.link.source === 'tiktok' || r.link.source === 'facebook')) || r.error.includes('gallery-dl')),
  )
  const toolIds = $derived([
    'ytdlp', 'ffmpeg', 'jsruntime',
    ...(hasSpotify ? ['spotdl'] : []),
    ...(needsGallery ? ['gallerydl'] : []),
  ])
  const pending = $derived(pendingCount())
  const running = $derived(runningCount() > 0)
  const missingTools = $derived(!hasTool('ytdlp') || !hasTool('ffmpeg') || (hasSpotify && !hasTool('spotdl')))

  async function submit() {
    const text = dl.input.trim()
    if (!text) return
    dl.input = ''
    await addLinks(text)
  }

  async function paste() {
    try {
      const text = (await runtime.clipboardText()) ?? ''
      if (!text.trim()) {
        toast(L('Clipboard kosong', 'The clipboard is empty'), 'info')
        return
      }
      await addLinks(text)
    } catch (e) {
      toast(errText(e), 'err')
    }
  }

  /** Selected/total items of a category tab; '' while nothing is ready. */
  function tabCount(rows: LinkRow[]): string {
    let sel = 0
    let total = 0
    for (const r of rows) {
      if (r.status !== 'ready') continue
      sel += selectedEntries(r).length
      total += visible(r).length
    }
    return total ? `${sel}/${total}` : ''
  }

  /** A link that holds exactly one item is shown as a single row, without a group header. */
  function isSingle(r: LinkRow): boolean {
    const t = r.col?.type
    return !!r.col && r.col.entries.length === 1 && t !== 'playlist' && t !== 'channel' && t !== 'album' && t !== 'artist' && t !== 'profile'
  }

  function entryKindLabel(e: Entry | undefined): string {
    if (!e) return 'Post'
    if (e.kind === 'image') return L('Foto', 'Photo')
    if (e.kind === 'audio') return L('Musik', 'Sound')
    return 'Video'
  }

  /** Second line of a group header / single row: what the link is. */
  function linkSummary(r: LinkRow): string {
    const col = r.col!
    const list = visible(r)
    const who = col.subtitle ? ` · ${col.subtitle}` : ''
    if (col.type === 'channel') {
      const tabs = Object.entries(col.tabCounts ?? {}).filter(([, n]) => n > 0).map(([k, n]) => `${n} ${tabLabel[k] ?? k}`).join(' · ')
      return `Channel${who} · ${col.entries.length} ${L('konten', 'items')}${tabs ? ` (${tabs})` : ''}`
    }
    if (isSocial(col.source)) {
      if (col.type === 'profile') return `${typeLabel.profile}${who} · ${list.length} video${L('', list.length === 1 ? '' : 's')}`
      return `${isSingle(r) ? entryKindLabel(list[0]) : 'Post'}${who}${isSingle(r) ? '' : ` · ${kindSummary(list)}`}`
    }
    const dur = list.reduce((s, e) => s + (e.duration || 0), 0)
    const n = list.length
    const noun = col.source === 'spotify' ? L('lagu', n === 1 ? 'track' : 'tracks') : L('video', n === 1 ? 'video' : 'videos')
    return `${typeLabel[col.type] ?? 'Link'}${who}${n > 1 ? ` · ${n} ${noun}` : ''}${dur ? ` · ${n > 1 ? '± ' + longDuration(dur) : duration(dur)}` : ''}`
  }

  function toggle(r: LinkRow, e: Entry) {
    if (!selectable(e, r)) return
    r.selected[e.id] = !r.selected[e.id]
  }

  function groupState(r: LinkRow): { all: boolean; some: boolean } {
    const can = visible(r).filter((e) => selectable(e, r)).length
    const sel = selectedEntries(r).length
    return { all: can > 0 && sel === can, some: sel > 0 && sel < can }
  }

  function onRange(r: LinkRow) {
    if (!r.col) return
    const set = parseRange(r.range, r.col.entries.length)
    if (!set) {
      toast(L('Format rentang tidak valid. Contoh: 1-20 atau 3,5,7-9', 'Invalid range. Example: 1-20 or 3,5,7-9'), 'err')
      return
    }
    applyRange(r, set)
  }

  function optsChanged() {
    if (cat) rememberOpts(cat)
  }

  /** Keeps the user's picks: only links that were fully selected are re-selected (with or without the
   *  already-downloaded items); a link the user cleared stays cleared. */
  function skipChanged() {
    if (!cat) return
    rememberOpts(cat)
    for (const r of readyRows) {
      const fresh = visible(r).filter((e) => !e.unavailable && !e.archived)
      if (fresh.every((e) => r.selected[e.id]) && (fresh.length > 0 || Object.keys(r.selected).length === 0)) applyScope(r)
    }
  }

  function shortUrl(u: string) {
    return u.replace(/^https?:\/\/(www\.)?/, '')
  }

  const breakdown = $derived.by(() => {
    const perCat = new Map<Category, number>()
    for (const r of dl.rows) {
      if (r.status !== 'ready' || !r.col) continue
      const n = toQueue(r).length
      if (n) perCat.set(categoryOf(r.link.source), (perCat.get(categoryOf(r.link.source)) ?? 0) + n)
    }
    return [...perCat].map(([c, n]) => `${n} ${sourceName[c]}`).join(' + ')
  })

  const lastOutput = $derived.by(() => {
    let best = ''
    let at = 0
    for (const r of dl.rows)
      for (const tid of Object.values(r.taskOf)) {
        const t = tasks[tid]
        if (t?.output && t.finished >= at) {
          best = t.output
          at = t.finished
        }
      }
    return best
  })
</script>

{#snippet thumb(src: string, square: boolean, seed: string)}
  {#if src}
    <img class="th" class:sq={square} src={src} alt="" loading="lazy" referrerpolicy="no-referrer" onerror={(ev) => ((ev.currentTarget as HTMLImageElement).style.visibility = 'hidden')} />
  {:else}
    {@const tt = tile(seed)}
    <div class="th" class:sq={square} style="background: {tt.bg}"></div>
  {/if}
{/snippet}

{#snippet entryRow(r: LinkRow, e: Entry, single: boolean)}
  {@const col = r.col!}
  {@const tid = r.taskOf[e.id]}
  {@const t = tid ? tasks[tid] : undefined}
  {@const can = selectable(e, r)}
  <div class="entry" class:single class:dim={!can} class:active={t?.status === 'running'}>
    <input type="checkbox" aria-label={L(`Pilih ${e.title}`, `Select ${e.title}`)} checked={!!r.selected[e.id] && can} disabled={!can} onchange={() => toggle(r, e)} />
    {#if single}
      <span class="idx"></span>
    {:else}
      <span class="idx">{String(e.index).padStart(2, '0')}</span>
    {/if}
    {@render thumb(e.thumbnail || col.thumbnail, col.source === 'spotify' || e.kind === 'image' || (single && isSocial(col.source)), e.title)}
    <div class="et">
      <span class="etitle ellipsis" title={single ? col.title : e.title}>{single ? col.title || e.title : e.title}</span>
      {#if t && (t.status === 'running' || t.status === 'queued') && t.message}
        <span class="esub ellipsis">{t.message}</span>
      {:else if single}
        <span class="esub ellipsis" title={r.link.url}>{e.artist ? `${e.artist}${e.album ? ` · ${e.album}` : ''}` : linkSummary(r)}{r.cut ? ` · ✂ ${r.cut.replace('-', '–')}` : ''}</span>
      {:else if e.artist}
        <span class="esub ellipsis">{e.artist}{e.album ? ` · ${e.album}` : ''}</span>
      {:else if e.unavailable}
        <span class="esub">{L('Tidak tersedia (privat/dihapus)', 'Unavailable (private/deleted)')}</span>
      {:else if isSocial(col.source)}
        <span class="esub kind"><Icon name={e.kind === 'image' ? 'image' : e.kind === 'audio' ? 'music' : 'video'} size={12} />{entryKindLabel(e)}</span>
      {/if}
    </div>
    <div class="eright">
      {#if t}
        {#if t.status === 'running'}
          <div class="erun">
            <span>{t.progress >= 0 ? pct(t.progress) : '…'}</span>
            <div class="bar thin" class:indeterminate={t.progress < 0}><div style="width: {Math.max(0, t.progress) * 100}%"></div></div>
          </div>
        {:else if t.status === 'queued'}
          <span class="pill muted">{L('Menunggu', 'Waiting')}</span>
        {:else if t.status === 'done'}
          <button class="pill ok as-btn" title={L('Tampilkan file', 'Show file')} onclick={() => api.revealFile(t.output)}><Icon name="check" size={12} stroke={3} />{L('Selesai', 'Done')}</button>
          {#if t.output}<button class="mini" title={L('Kirim ke konversi', 'Send to a converter')} aria-label={L('Kirim ke konversi', 'Send to a converter')} onclick={() => sendToConverter([t.output])}><Icon name="send" size={14} /></button>{/if}
        {:else if t.status === 'failed'}
          <button class="pill err as-btn" title={t.message} onclick={() => showDetail(e.title, t.message, t.detail)}>{L('Gagal · detail', 'Failed · details')}</button>
          {#if col.source === 'spotify'}<button class="mini" title={L('Ganti dengan link YouTube', 'Replace with a YouTube link')} aria-label={L('Ganti dengan link YouTube', 'Replace with a YouTube link')} onclick={() => setSource(r, e)}><Icon name="link" size={14} /></button>{/if}
        {:else if t.status === 'skipped'}
          <span class="pill muted" title={t.message}>{t.message || L('Dilewati', 'Skipped')}</span>
        {:else}
          <span class="pill muted">{L('Dibatalkan', 'Canceled')}</span>
        {/if}
      {:else}
        {#if col.source === 'spotify'}
          <button class="mini" class:on={!!e.source} title={e.source ? L(`Link pilihan: ${e.source}`, `Chosen link: ${e.source}`) : L('Pilih link YouTube sendiri', 'Pick a YouTube link yourself')} aria-label={L('Pilih link YouTube', 'Pick a YouTube link')} onclick={() => setSource(r, e)}><Icon name="link" size={14} /></button>
        {:else if single && e.kind !== 'image'}
          <button class="mini" class:on={!!r.cut} title={r.cut ? L(`Hanya ${r.cut}`, `Only ${r.cut}`) : L('Unduh sebagian (potong waktu)', 'Download a part (cut)')} aria-label={L('Potong waktu', 'Cut')} onclick={() => setCut(r)}><Icon name="scissors" size={14} /></button>
        {/if}
        {#if e.date && col.type === 'channel'}<span class="edate">{e.archived && rowOpts(r).skipExisting ? L('Sudah ada', 'Already have') : ymd(e.date)}</span>{:else if e.archived && rowOpts(r).skipExisting}<span class="edate">{L('Sudah ada', 'Already have')}</span>{/if}
        <span class="edur">{duration(e.duration)}</span>
      {/if}
    </div>
    {#if t && isActive(t)}
      <button class="mini" aria-label={L('Batalkan', 'Cancel')} title={L('Batalkan', 'Cancel')} onclick={() => api.cancelTask(t.id)}><Icon name="x" size={14} /></button>
    {:else if single}
      <button class="mini" aria-label={L('Hapus link', 'Remove link')} title={L('Hapus link', 'Remove link')} onclick={() => removeRow(r.id)}><Icon name="trash" size={14} /></button>
    {:else}
      <span></span>
    {/if}
  </div>
{/snippet}

<PageHeader title="Download" subtitle={L('YouTube, TikTok, Instagram, Facebook & Spotify — tempel link-nya, otomatis masuk ke kategorinya.', 'YouTube, TikTok, Instagram, Facebook & Spotify — paste the link and it lands in its category automatically.')} />
<ToolBanner ids={toolIds} why={L('Dibutuhkan untuk membaca link dan mengunduh. Sekali pasang, dipakai seterusnya.', 'Needed to read links and download. Install once, use forever.')} />

<div class="body">
  <div class="left">
    <section class="card links" aria-label="Link">
      <form class="input-row" onsubmit={(e) => { e.preventDefault(); submit() }}>
        <div class="input-wrap">
          <span class="lic"><Icon name="link" /></span>
          <input class="url" aria-label={L('Link yang akan diunduh', 'Link to download')} placeholder={L('Tempel link YouTube, TikTok, Instagram, Facebook, atau Spotify…', 'Paste a YouTube, TikTok, Instagram, Facebook or Spotify link…')} bind:value={dl.input} />
        </div>
        <button type="button" class="btn tall" onclick={paste}><Icon name="clipboard" size={16} />{L('Tempel', 'Paste')}</button>
        <button type="submit" class="btn-accent tall" disabled={!dl.input.trim()}>{L('Periksa link', 'Check link')}</button>
      </form>
    </section>

    <section class="card content" aria-label={L('Isi link', 'Link contents')}>
      {#if dl.rows.length === 0}
        <div class="empty">
          <div class="big-ic"><Icon name="download" size={34} stroke={1.8} /></div>
          <h2>{L('Tempel link untuk mulai', 'Paste a link to start')}</h2>
          <p>{L('Bisa beberapa link sekaligus, satu per baris. Tiap link otomatis masuk ke kategori platformnya.', 'Several links at once work too, one per line. Each link goes into its platform category automatically.')}</p>
          <div class="examples">
            <div><span class="badge yt"><Icon name="play" size={10} stroke={3} />YouTube</span> {L('video · Shorts · playlist · channel (@nama)', 'videos · Shorts · playlists · channels (@name)')}</div>
            <div><span class="badge tt"><Icon name="play" size={10} stroke={3} />TikTok</span> {L('video · foto slide + musik · profil', 'videos · photo slides + sound · profiles')}</div>
            <div><span class="badge ig"><Icon name="image" size={10} stroke={3} />Instagram</span> {L('reel · post foto & video · carousel', 'reels · photo & video posts · carousels')}</div>
            <div><span class="badge fb"><Icon name="play" size={10} stroke={3} />Facebook</span> {L('video · reel · foto', 'videos · reels · photos')}</div>
            <div><span class="badge sp"><Icon name="music" size={10} stroke={3} />Spotify</span> {L('lagu · album · playlist · artis', 'tracks · albums · playlists · artists')}</div>
          </div>
        </div>
      {:else}
        <div class="tabs" role="tablist" aria-label={L('Kategori', 'Categories')}>
          {#each cats as c (c.id)}
            {@const count = tabCount(c.rows)}
            <button role="tab" aria-selected={cat === c.id} class:on={cat === c.id} onclick={() => (dl.cat = c.id)}>
              <span class="dot {sourceClass[c.id]}"></span>{sourceName[c.id]}
              {#if c.rows.some((r) => r.status === 'loading')}
                <Icon name="loader" size={12} class="spin" />
              {:else if c.rows.some((r) => r.status === 'error')}
                <span class="terr" title={L('Ada link yang gagal dibaca', 'A link failed to read')}><Icon name="alert" size={12} /></span>
              {/if}
              {#if count}<span class="tcount">{count}</span>{/if}
            </button>
          {/each}
        </div>

        {#if cat}
          <div class="selbar">
            <input
              id="semua"
              type="checkbox"
              checked={catSelected > 0 && catSelected === catSelectable}
              indeterminate={catSelected > 0 && catSelected < catSelectable}
              disabled={catSelectable === 0}
              onchange={(e) => setAllInCategory(cat, (e.currentTarget as HTMLInputElement).checked)}
            />
            <label for="semua">{L('Pilih semua', 'Select all')}</label>
            <span class="selinfo">
              {#if catEntries.length === 0 && catRows.some((r) => r.status === 'loading')}
                {L('Membaca isi link…', 'Reading the links…')}
              {:else}
                {L(`${catSelected} dari ${catEntries.length} dipilih`, `${catSelected} of ${catEntries.length} selected`)} · {catRows.length} link
              {/if}
            </span>
            {#if catArchived > 0}<span class="arch">{catArchived} {L('sudah pernah diunduh · dilewati', 'downloaded before · skipped')}</span>{/if}
            {#if catOutputs.length}
              <button class="link send" onclick={() => sendToConverter(catOutputs)} title={L('Buka hasil unduhan di halaman konversi yang cocok', 'Open the downloads on the matching converter page')}><Icon name="send" size={12} /> {L(`Kirim ${catOutputs.length} hasil ke konversi`, `Send ${catOutputs.length} to a converter`)}</button>
            {/if}
            <button class="link clear" onclick={() => clearCategory(cat)}>{L(`Hapus semua link ${sourceName[cat]}`, `Remove all ${sourceName[cat]} links`)}</button>
          </div>
        {/if}

        <div class="groups">
          {#each catRows as r (r.id)}
            {#if r.status === 'loading'}
              <div class="gstate">
                <Icon name="loader" size={16} class="spin" />
                <div class="gs-text">
                  <span class="ellipsis" title={r.link.url}>{shortUrl(r.link.url)}</span>
                  <span class="gs-sub">{L('Membaca isi link…', 'Reading the link…')} {r.link.type === 'channel' ? L('Channel besar bisa butuh beberapa menit.', 'Large channels can take a few minutes.') : ''}</span>
                </div>
                <button class="mini" aria-label={L('Hapus link', 'Remove link')} title={L('Hapus link', 'Remove link')} onclick={() => removeRow(r.id)}><Icon name="x" size={14} /></button>
              </div>
            {:else if r.status === 'error'}
              <div class="gstate err">
                <Icon name="alert" size={16} />
                <div class="gs-text">
                  <span class="ellipsis" title={r.link.url}>{shortUrl(r.link.url)}</span>
                  <button class="gs-sub gs-err ellipsis" title={r.error} onclick={() => showDetail(r.link.url, r.error, '')}>{r.error}</button>
                </div>
                <button class="btn small" onclick={() => analyze(r.id)}><Icon name="refresh" size={14} />{L('Coba lagi', 'Try again')}</button>
                <button class="mini" aria-label={L('Hapus link', 'Remove link')} title={L('Hapus link', 'Remove link')} onclick={() => removeRow(r.id)}><Icon name="x" size={14} /></button>
              </div>
            {:else if r.col}
              {@const col = r.col}
              {#if isSingle(r)}
                {@render entryRow(r, col.entries[0], true)}
              {:else}
                {@const gs = groupState(r)}
                <div class="group">
                  <div class="ghead">
                    <input
                      type="checkbox"
                      aria-label={L(`Pilih semua di ${col.title}`, `Select all in ${col.title}`)}
                      checked={gs.all}
                      indeterminate={gs.some}
                      onchange={(e) => setAll(r, (e.currentTarget as HTMLInputElement).checked)}
                    />
                    {#if col.type === 'channel' || col.type === 'profile'}
                      {@const t = tile(col.title)}
                      <div class="avatar" style="background: {t.bg}; color: {t.fg}">{initials(col.title)}</div>
                    {:else if col.thumbnail}
                      <img class="cover" class:sq={col.source === 'spotify' || social} src={col.thumbnail} alt="" referrerpolicy="no-referrer" onerror={(e) => ((e.currentTarget as HTMLImageElement).style.visibility = 'hidden')} />
                    {:else}
                      <div class="cover blank"><Icon name={sourceIcon[col.source] ?? 'play'} /></div>
                    {/if}
                    <div class="ch-text">
                      <span class="ch-title ellipsis" title={col.title}>{col.title || L('Tanpa judul', 'Untitled')}</span>
                      <span class="ch-sub ellipsis">{linkSummary(r)}</span>
                    </div>
                    {#if col.type === 'playlist' || col.type === 'album' || col.type === 'artist'}
                      <form class="range" onsubmit={(e) => { e.preventDefault(); onRange(r) }}>
                        <label for="rentang-{r.id}">{L('Rentang', 'Range')}</label>
                        <input id="rentang-{r.id}" class="text-input" bind:value={r.range} onblur={() => onRange(r)} />
                      </form>
                    {/if}
                    {#if col.source !== 'spotify'}
                      <button class="mini" class:on={!!r.cut} title={r.cut ? L(`Semua video: hanya ${r.cut}`, `Every video: only ${r.cut}`) : L('Unduh sebagian (potong waktu)', 'Download a part (cut)')} aria-label={L('Potong waktu', 'Cut')} onclick={() => setCut(r)}><Icon name="scissors" size={14} /></button>
                    {/if}
                    <button class="mini" aria-label={L('Hapus link', 'Remove link')} title={L('Hapus link', 'Remove link')} onclick={() => removeRow(r.id)}><Icon name="trash" size={14} /></button>
                  </div>
                  {#if col.type === 'channel'}
                    <div class="gfilter">
                      <div class="multi">
                        {#each ['videos', 'shorts', 'streams'] as k}
                          {@const n = col.tabCounts?.[k] ?? 0}
                          <button class="mchip" class:on={r.types[k]} disabled={n === 0} aria-pressed={r.types[k]} onclick={() => { r.types[k] = !r.types[k]; applyScope(r) }}>
                            {tabLabel[k]}<span>{n}</span>
                          </button>
                        {/each}
                      </div>
                      <div class="scope">
                        <Segmented label={L('Cakupan', 'Scope')} bind:value={r.scope} onchange={() => applyScope(r)} options={[{ value: 'all', label: L('Semua', 'All') }, { value: 'latest', label: L('N terbaru', 'Latest N') }, { value: 'since', label: L('Sejak tanggal', 'Since date') }]} />
                      </div>
                      {#if r.scope === 'latest'}
                        <input class="text-input small" aria-label={L('Jumlah terbaru', 'How many latest')} inputmode="numeric" value={r.latestN} oninput={(e) => { const v = parseInt((e.currentTarget as HTMLInputElement).value, 10); r.latestN = isNaN(v) ? 1 : Math.max(1, Math.min(5000, v)); applyScope(r) }} />
                        <span class="t12">{L('terbaru per jenis', 'latest per type')}</span>
                      {:else if r.scope === 'since'}
                        <input class="text-input small" type="date" aria-label={L('Sejak tanggal', 'Since date')} bind:value={r.since} onchange={() => applyScope(r)} />
                        <span class="t12">{L('tanggal perkiraan', 'approximate dates')}</span>
                      {/if}
                    </div>
                  {/if}
                  {#each visible(r) as e (e.id)}
                    {@render entryRow(r, e, false)}
                  {/each}
                </div>
              {/if}
            {/if}
          {/each}
        </div>
      {/if}
    </section>
  </div>

  <aside class="card panel" aria-label={L('Pengaturan download', 'Download settings')}>
    {#if cat && opts}
      <div class="ph">
        <span class="ph-k">{L('PENGATURAN KATEGORI', 'CATEGORY SETTINGS')}</span>
        <span class="ph-v"><span class="dot {sourceClass[cat]}"></span>{sourceName[cat]} · {catRows.length} link</span>
      </div>
      <div class="scroll">
        <p class="hint">{L(`Berlaku untuk semua link ${sourceName[cat]} di daftar.`, `Applies to every ${sourceName[cat]} link in the list.`)}</p>
        <PresetBar module={`download.${cat}`} target={opts} builtins={dlBuiltins} onapply={optsChanged} />
        {#if cat === 'spotify'}
          <div class="info">
            <Icon name="info" size={16} />
            <span>{L('Spotify selalu diunduh sebagai audio. Lagunya dicocokkan dari YouTube, lalu diberi judul, artis, album & cover dari Spotify.', 'Spotify is always downloaded as audio. Songs are matched on YouTube, then tagged with the title, artist, album & cover from Spotify.')}</span>
          </div>
          <AudioDownloadOptions {opts} spotify label={L('Format audio', 'Audio format')} onchange={optsChanged} />
          {#if hasList}
            <Switch bind:checked={opts.numbering} onchange={optsChanged} label={L('Nomor urut di nama file', 'Track numbers in file names')} hint={L('Urutan sama seperti di Spotify', 'Same order as on Spotify')} />
            <Switch bind:checked={opts.playlist} onchange={optsChanged} label={L('Buat file playlist (.m3u8)', 'Create a playlist file (.m3u8)')} hint={L('Untuk album & playlist, bisa dibuka di VLC, foobar, dll.', 'For albums & playlists, opens in VLC, foobar, etc.')} />
          {/if}
        {:else}
          {#if social && kinds.image}
            <div class="sec">
              <span class="label">{L('Format foto', 'Photo format')}</span>
              <Segmented label={L('Format foto', 'Photo format')} bind:value={opts.imageFormat} onchange={optsChanged} options={[{ value: 'original', label: L('Asli', 'Original') }, { value: 'jpg', label: 'JPG' }]} />
              <p class="hint">{opts.imageFormat === 'jpg' ? L('WEBP/PNG/HEIC diubah ke JPG agar bisa dibuka di mana saja.', 'WEBP/PNG/HEIC are converted to JPG so they open anywhere.') : L('Foto disimpan apa adanya dari sumbernya.', 'Photos are saved exactly as the source provides them.')}</p>
            </div>
          {/if}

          {#if !social || kinds.video}
            <div class="sec">
              <span class="label">{L('Unduh video sebagai', 'Download videos as')}</span>
              <Segmented label={L('Unduh sebagai', 'Download as')} bind:value={opts.mode} onchange={optsChanged} options={[{ value: 'video', label: 'Video', icon: 'video' }, { value: 'audio', label: 'Audio', icon: 'music' }]} />
            </div>
            {#if opts.mode === 'video'}
              <div class="sec">
                <span class="label">{L('Kualitas video', 'Video quality')}</span>
                <Chips bind:value={opts.quality} columns={3} small onchange={optsChanged} options={[{ value: 'best', label: L('Terbaik', 'Best') }, { value: '2160', label: '4K' }, { value: '1440', label: '1440p' }, { value: '1080', label: '1080p' }, { value: '720', label: '720p' }, { value: '480', label: '480p' }]} />
              </div>
              <div class="sec">
                <span class="label">{L('Format file', 'File format')}</span>
                <Chips bind:value={opts.container} columns={2} onchange={optsChanged} options={[{ value: 'mp4', label: 'MP4' }, { value: 'mkv', label: 'MKV' }]} />
              </div>
            {:else}
              <AudioDownloadOptions {opts} label={L('Format audio', 'Audio format')} onchange={optsChanged} />
            {/if}
          {:else if kinds.audio}
            <AudioDownloadOptions {opts} label={L('Format musik', 'Sound format')} onchange={optsChanged} />
          {/if}

          {#if !social || kinds.video || kinds.audio}
            <Switch bind:checked={opts.embed} onchange={optsChanged} label={L('Sematkan info & thumbnail', 'Embed info & thumbnail')} hint={L('Judul, channel, dan gambar sampul', 'Title, channel and cover image')} />
          {/if}
          {#if (!social || kinds.video) && opts.mode === 'video'}
            <div class="sec">
              <span class="label">Subtitle</span>
              <Segmented
                label="Subtitle"
                bind:value={opts.subtitles}
                onchange={optsChanged}
                options={[
                  { value: 'none', label: L('Tanpa', 'None') },
                  { value: 'file', label: L('File .srt', '.srt file') },
                  { value: 'embed', label: L('Sematkan', 'Embed') },
                ]}
              />
              {#if opts.subtitles !== 'none'}
                <input class="text-input" aria-label={L('Bahasa subtitle', 'Subtitle languages')} bind:value={opts.subLangs} onblur={optsChanged} placeholder="id,en" />
                <p class="hint">{L('Kode bahasa dipisah koma (id, en, ja…). Subtitle otomatis dipakai bila tidak ada yang asli.', 'Comma-separated language codes (id, en, ja…). Auto-generated subtitles are used when there are no real ones.')}</p>
              {/if}
            </div>
          {/if}
          {#if cat === 'youtube'}
            <div class="sec">
              <span class="label">SponsorBlock</span>
              <Segmented
                label="SponsorBlock"
                bind:value={opts.sponsorBlock}
                onchange={optsChanged}
                options={[
                  { value: 'off', label: L('Mati', 'Off') },
                  { value: 'mark', label: L('Tandai bab', 'Mark chapters') },
                  { value: 'remove', label: L('Buang', 'Remove') },
                ]}
              />
              {#if opts.sponsorBlock !== 'off'}<p class="hint">{opts.sponsorBlock === 'remove' ? L('Bagian sponsor, promosi & ajakan subscribe dipotong dari video (data dari komunitas SponsorBlock).', 'Sponsor, self-promo & "subscribe" parts are cut out (data from the SponsorBlock community).') : L('Bagian sponsor ditandai sebagai bab, videonya utuh.', 'Sponsor parts are marked as chapters; the video stays whole.')}</p>{/if}
            </div>
          {/if}
          {#if hasPlaylist}
            <Switch bind:checked={opts.numbering} onchange={optsChanged} label={L('Nomor urut di nama file', 'Numbers in file names')} hint={L('Untuk playlist: 01 - judul, 02 - judul, …', 'For playlists: 01 - title, 02 - title, …')} />
            <Switch bind:checked={opts.playlist} onchange={optsChanged} label={L('Buat file playlist (.m3u8)', 'Create a playlist file (.m3u8)')} hint={L('Daftar putar sesuai urutan playlist', 'A play list in playlist order')} />
          {/if}
          <p class="hint">{L('Unduh sebagian: klik ikon gunting di tiap link.', 'Download a part: click the scissors icon on a link.')}</p>
        {/if}
        <Switch bind:checked={opts.skipExisting} onchange={skipChanged} label={L('Lewati yang sudah ada', 'Skip existing')} hint={L('Unduh ulang hanya ambil yang baru', 'Downloading again only fetches new items')} />

        <div class="sec">
          <span class="label">{L('Simpan ke', 'Save to')}</span>
          <OutputPicker kind="download" />
          <p class="hint">
            {settings.value?.downloadSubfolders ?? true ? L('Playlist, channel, album & post berisi banyak item otomatis dibuat subfolder sendiri.', 'Playlists, channels, albums & multi-item posts get their own subfolder automatically.') : L('Semua file langsung masuk ke folder ini.', 'All files go straight into this folder.')}
          </p>
        </div>
      </div>
    {:else}
      <div class="ph">
        <span class="ph-k">{L('PENGATURAN', 'SETTINGS')}</span>
        <span class="ph-v">Download</span>
      </div>
      <div class="scroll">
        <p class="hint">{L('Tempel link lalu klik', 'Paste a link, then click')} <b>{L('Periksa link', 'Check link')}</b>. {L('Link dikelompokkan per platform, dan pengaturannya muncul di sini untuk tiap kategori.', 'Links are grouped by platform, and each category gets its settings here.')}</p>
        <div class="sec">
          <span class="label">{L('Simpan ke', 'Save to')}</span>
          <OutputPicker kind="download" />
          <p class="hint">
            {settings.value?.downloadSubfolders ?? true ? L('Playlist, channel, album & post berisi banyak item otomatis dibuat subfolder sendiri.', 'Playlists, channels, albums & multi-item posts get their own subfolder automatically.') : L('Semua file langsung masuk ke folder ini.', 'All files go straight into this folder.')} {L('Bisa diubah di Pengaturan.', 'You can change this in Settings.')}
          </p>
        </div>
      </div>
    {/if}

    <RunFooter
      kind="download"
      {running}
      busy={dl.starting}
      startIcon="download"
      startLabel={pending > 0 ? `${L('Unduh semua', 'Download all')} · ${pending} item${L('', pending === 1 ? '' : 's')}` : L('Unduh semua', 'Download all')}
      disabled={pending === 0 || missingTools}
      disabledHint={missingTools ? L('Pasang tools yang dibutuhkan dulu (lihat banner).', 'Install the required tools first (see the banner).') : dl.rows.length === 0 ? '' : L('Pilih minimal satu item.', 'Select at least one item.')}
      note={pending > 0 ? breakdown : ''}
      {lastOutput}
      queueMore={running ? pending : 0}
      onstart={startAll}
      oncancel={cancelAll}
    />
  </aside>
</div>

<style>
  .body {
    flex-grow: 1;
    min-height: 0;
    display: flex;
    gap: 20px;
    padding: 0 24px 24px 28px;
  }
  .left {
    flex-grow: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .links {
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 10px;
    flex-shrink: 0;
  }
  .input-row {
    display: flex;
    gap: 8px;
  }
  .input-wrap {
    position: relative;
    flex-grow: 1;
  }
  .lic {
    position: absolute;
    left: 14px;
    top: 13px;
    color: var(--text-3);
    display: flex;
  }
  .url {
    width: 100%;
    height: 44px;
    padding: 0 12px 0 42px;
    border-radius: 12px;
    border: 1px solid var(--border-strong);
    background: var(--inset);
    font-size: 14px;
  }
  .url:focus {
    outline: none;
    border-color: var(--accent);
  }
  .tall {
    height: 44px;
    border-radius: 12px;
  }
  .badge {
    width: 82px;
    flex-shrink: 0;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    height: 24px;
    border-radius: 999px;
    font-size: 11px;
    font-weight: 700;
  }
  .badge.yt {
    background: var(--err-soft);
    color: var(--danger-text);
  }
  .badge.sp {
    background: var(--ok-soft);
    color: var(--ok);
  }
  .badge.tt {
    background: var(--tt-bg);
    color: var(--tt-fg);
  }
  .badge.ig {
    background: var(--ig-bg);
    color: var(--ig-fg);
  }
  .badge.fb {
    background: var(--fb-bg);
    color: var(--fb-fg);
  }
  .mini {
    width: 32px;
    height: 32px;
    flex-shrink: 0;
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
  .content {
    flex-grow: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .empty {
    flex-grow: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 10px;
    padding: 24px;
    text-align: center;
  }
  .big-ic {
    width: 72px;
    height: 72px;
    border-radius: 22px;
    background: var(--accent-tint);
    color: var(--accent);
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: 6px;
  }
  .empty h2 {
    margin: 0;
    font-size: 20px;
    font-weight: 800;
  }
  .empty p {
    margin: 0;
    color: var(--text-2);
    font-size: 14px;
  }
  .examples {
    margin-top: 10px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    font-size: 13px;
    color: var(--text-2);
    align-items: flex-start;
  }
  .examples div {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .tabs {
    display: flex;
    gap: 2px;
    padding: 4px 10px 0;
    border-bottom: 1px solid var(--border);
    overflow-x: auto;
    flex-shrink: 0;
  }
  .tabs button {
    height: 42px;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 10px;
    border: 0;
    border-bottom: 2px solid transparent;
    background: transparent;
    color: var(--text-2);
    font-size: 12px;
    font-weight: 600;
    white-space: nowrap;
  }
  .tabs button.on {
    border-bottom-color: var(--accent);
    color: var(--text);
    font-weight: 700;
  }
  .tabs .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--err);
  }
  .tabs .dot.sp {
    background: var(--ok);
  }
  .tabs .dot.tt {
    background: #3ee8e1;
  }
  .tabs .dot.ig {
    background: #ff5fa2;
  }
  .tabs .dot.fb {
    background: #4d94ff;
  }
  .tabs .dot.ot {
    background: var(--text-3);
  }
  .avatar {
    width: 54px;
    height: 54px;
    flex-shrink: 0;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 17px;
    font-weight: 800;
  }
  .cover {
    width: 96px;
    height: 54px;
    flex-shrink: 0;
    border-radius: 8px;
    object-fit: cover;
    background: var(--surface-2);
  }
  .cover.sq {
    width: 54px;
  }
  .cover.blank {
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-3);
  }
  .ch-text {
    flex-grow: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  .ch-title {
    font-size: 15px;
    font-weight: 800;
  }
  .ch-sub {
    font-size: 12px;
    color: var(--text-3);
  }
  .range {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .range label {
    font-size: 12px;
    font-weight: 700;
    color: var(--text-2);
  }
  .range input {
    width: 92px;
    height: 34px;
    text-align: center;
  }
  .selbar {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 38px;
    padding: 0 16px;
    border-bottom: 1px solid var(--border-soft);
    background: var(--surface-3);
    flex-shrink: 0;
  }
  .selbar label {
    font-size: 12px;
    font-weight: 700;
  }
  .selinfo {
    flex-grow: 1;
    font-size: 12px;
    color: var(--text-3);
  }
  .arch {
    height: 22px;
    display: inline-flex;
    align-items: center;
    padding: 0 8px;
    border-radius: 6px;
    background: var(--ok-soft);
    color: var(--ok);
    font-size: 11px;
    font-weight: 700;
    white-space: nowrap;
  }
  input[type='checkbox'] {
    width: 18px;
    height: 18px;
    margin: 0;
    accent-color: var(--accent);
    flex-shrink: 0;
  }
  .groups {
    flex-grow: 1;
    min-height: 0;
    overflow-y: auto;
  }
  .group {
    border-bottom: 1px solid var(--border);
  }
  .ghead {
    position: sticky;
    top: 0;
    z-index: 1;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 8px 10px 16px;
    background: var(--hover);
    border-bottom: 1px solid var(--border-soft);
  }
  .ghead .avatar {
    width: 40px;
    height: 40px;
    font-size: 14px;
  }
  .ghead .cover {
    width: 70px;
    height: 40px;
  }
  .ghead .cover.sq {
    width: 40px;
  }
  .ghead .ch-title {
    font-size: 14px;
  }
  .gfilter {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 10px;
    padding: 8px 16px 8px 46px;
    border-bottom: 1px solid var(--border-soft);
  }
  .gfilter .scope {
    width: 300px;
  }
  .gfilter .t12 {
    font-size: 12px;
    color: var(--text-3);
  }
  .text-input.small {
    width: 120px;
    height: 34px;
  }
  .gstate {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 56px;
    padding: 6px 8px 6px 16px;
    border-bottom: 1px solid var(--border-soft);
    color: var(--text-2);
  }
  .gstate.err {
    color: var(--err);
  }
  .gs-text {
    flex-grow: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
    font-size: 13px;
    color: var(--text);
  }
  .gs-sub {
    font-size: 12px;
    color: var(--text-3);
  }
  .gs-err {
    padding: 0;
    border: 0;
    background: none;
    text-align: left;
    color: var(--err);
    cursor: pointer;
  }
  .btn.small {
    height: 30px;
    padding: 0 10px;
    font-size: 12px;
    flex-shrink: 0;
  }
  .tabs .tcount {
    padding: 1px 7px;
    border-radius: 999px;
    background: var(--surface-2);
    color: var(--text-2);
    font-size: 11px;
    font-variant-numeric: tabular-nums;
  }
  .tabs button.on .tcount {
    background: var(--accent-soft);
    color: var(--accent-text-2);
  }
  .tabs .terr {
    display: inline-flex;
    color: var(--err);
  }
  .selbar .clear {
    margin-left: auto;
  }
  .selbar .send {
    margin-left: auto;
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .selbar .send + .clear {
    margin-left: 12px;
  }
  .mini.on {
    color: var(--accent);
  }
  .ph-v {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .ph-v .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--err);
  }
  .ph-v .dot.sp {
    background: var(--ok);
  }
  .ph-v .dot.tt {
    background: #3ee8e1;
  }
  .ph-v .dot.ig {
    background: #ff5fa2;
  }
  .ph-v .dot.fb {
    background: #4d94ff;
  }
  .ph-v .dot.ot {
    background: var(--text-3);
  }
  .group .entry {
    padding-left: 16px;
  }
  .entry.single {
    min-height: 58px;
  }
  .entry {
    display: grid;
    grid-template-columns: 18px 26px 56px minmax(0, 1fr) 150px 32px;
    align-items: center;
    gap: 12px;
    min-height: 50px;
    padding: 4px 8px 4px 16px;
    border-bottom: 1px solid var(--border-soft);
  }
  .entry.active {
    background: var(--row-active);
  }
  .entry.dim .et,
  .entry.dim .th {
    opacity: 0.5;
  }
  .idx {
    font-size: 12px;
    color: var(--text-3);
    font-variant-numeric: tabular-nums;
  }
  .th {
    width: 56px;
    height: 32px;
    border-radius: 6px;
    object-fit: cover;
    background: var(--surface-2);
  }
  .th.sq {
    width: 36px;
    height: 36px;
  }
  .et {
    display: flex;
    flex-direction: column;
    gap: 1px;
    min-width: 0;
  }
  .etitle {
    font-size: 13px;
    font-weight: 600;
  }
  .esub {
    font-size: 12px;
    color: var(--text-3);
  }
  .esub.kind {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .eright {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 12px;
    min-width: 0;
  }
  .edate {
    font-size: 12px;
    color: var(--text-3);
    white-space: nowrap;
    font-variant-numeric: tabular-nums;
  }
  .edur {
    width: 50px;
    text-align: right;
    font-size: 12px;
    color: var(--text-2);
    font-variant-numeric: tabular-nums;
  }
  .erun {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 5px;
    font-size: 12px;
    font-weight: 700;
    color: var(--accent-text-2);
  }
  .thin {
    height: 4px;
  }
  .as-btn {
    border: 0;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .pill.muted {
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .panel {
    width: 344px;
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .ph {
    padding: 14px 20px;
    border-bottom: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: 2px;
    flex-shrink: 0;
  }
  .ph-k {
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.06em;
    color: var(--text-3);
  }
  .ph-v {
    font-size: 14px;
    font-weight: 800;
  }
  .scroll {
    flex-grow: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 16px 20px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .sec {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .info {
    display: flex;
    gap: 10px;
    padding: 12px;
    border-radius: 10px;
    background: var(--info-soft);
    color: var(--info-text);
    font-size: 12px;
    line-height: 1.5;
  }
  .multi {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 6px;
  }
  .mchip {
    height: 40px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 1px;
    border-radius: 10px;
    border: 1px solid var(--border);
    background: var(--surface-2);
    color: var(--text-4);
    font-size: 12px;
    font-weight: 600;
  }
  .mchip span {
    font-size: 10px;
    color: var(--text-3);
  }
  .mchip.on {
    border-color: var(--accent);
    background: var(--accent-soft);
    color: var(--accent-text);
    font-weight: 700;
  }
  .mchip.on span {
    color: var(--accent-text-2);
  }
  .mchip:disabled {
    opacity: 0.35;
  }
  .t12 {
    font-size: 12px;
    color: var(--text-3);
  }
  input[type='date'] {
    color-scheme: dark;
  }
  .link {
    align-self: flex-start;
    padding: 0;
    border: 0;
    background: none;
    font-size: 12px;
    font-weight: 700;
    color: var(--accent-text-2);
  }
</style>
