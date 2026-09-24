<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import PageHeader from '../components/PageHeader.svelte'
  import Chips from '../components/Chips.svelte'
  import Segmented from '../components/Segmented.svelte'
  import Switch from '../components/Switch.svelte'
  import RunFooter from '../components/RunFooter.svelte'
  import ToolBanner from '../components/ToolBanner.svelte'
  import Icon from '../components/Icon.svelte'
  import OutputPicker from '../components/OutputPicker.svelte'
  import { api, errText, runtime } from '../lib/api'
  import { hasTool, settings, showDetail, toast } from '../lib/stores/app.svelte'
  import {
    activeRow, addLinks, analyze, applyRange, applyScope, cancelAll, dl, isSocial, kindSummary, pendingCount, rememberOpts, removeRow,
    runningCount, selectable, selectedEntries, setAll, sourceClass, sourceName, startAll, tabLabel, toQueue, typeLabel, visible,
    type LinkRow,
  } from '../lib/stores/download.svelte'
  import { isActive, tasks } from '../lib/stores/tasks.svelte'
  import { duration, initials, longDuration, parseRange, pct, tile, ymd } from '../lib/format'
  import type { Collection, Entry } from '../lib/types'

  const row = $derived(activeRow())
  const entries = $derived(row ? visible(row) : [])
  const selCount = $derived(row ? selectedEntries(row).length : 0)
  const selectableCount = $derived(row ? entries.filter((e) => selectable(e, row)).length : 0)
  const archivedCount = $derived(row && row.opts.skipExisting ? entries.filter((e) => e.archived).length : 0)
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
  const social = $derived(!!row?.col && isSocial(row.col.source))
  const kinds = $derived.by(() => {
    const k = { video: false, audio: false, image: false }
    for (const e of entries) k[e.kind === 'image' ? 'image' : e.kind === 'audio' ? 'audio' : 'video'] = true
    return k
  })
  const pending = $derived(pendingCount())
  const running = $derived(runningCount() > 0)
  const missingTools = $derived(!hasTool('ytdlp') || !hasTool('ffmpeg') || (hasSpotify && !hasTool('spotdl')))

  let dirOf = $state<Record<string, string>>({})
  $effect(() => {
    const r = row
    // Re-resolve when the Download folder setting changes.
    void settings.value?.outputs?.download?.mode
    void settings.value?.outputs?.download?.dir
    void settings.value?.downloadSubfolders
    if (r?.col) {
      const key = r.col.key
      api.collectionDir(key).then((d) => (dirOf[key] = d)).catch(() => {})
    }
  })

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

  function countLabel(r: LinkRow): string {
    const col = r.col
    if (!col) return typeLabel[r.link.type] ?? 'Link'
    const n = col.entries.length
    const noun = col.source === 'spotify' ? L('lagu', n === 1 ? 'track' : 'tracks') : L('video', n === 1 ? 'video' : 'videos')
    if (col.type === 'video' || col.type === 'track') return typeLabel[col.type]
    if (col.type === 'channel') return `Channel · ${n} ${L('konten', 'items')}`
    if (col.type === 'post') return n > 1 ? `Post · ${n} item${L('', 's')}` : entryKindLabel(col.entries[0])
    if (col.type === 'profile') return `${typeLabel.profile} · ${n} video${L('', n === 1 ? '' : 's')}`
    return `${typeLabel[col.type] ?? 'Link'} · ${n} ${noun}`
  }

  function entryKindLabel(e: Entry | undefined): string {
    if (!e) return 'Post'
    if (e.kind === 'image') return L('Foto', 'Photo')
    if (e.kind === 'audio') return L('Musik', 'Sound')
    return 'Video'
  }

  function shortUrl(u: string) {
    return u.replace(/^https?:\/\/(www\.)?/, '')
  }

  function tabText(r: LinkRow): string {
    const name = r.col?.title || typeLabel[r.link.type]
    if (r.status === 'loading') return `${typeLabel[r.link.type] ?? 'Link'} · ${L('membaca…', 'reading…')}`
    if (r.status === 'error') return `${typeLabel[r.link.type] ?? 'Link'} · ${L('gagal', 'failed')}`
    const total = visible(r).length
    return `${name.length > 22 ? name.slice(0, 21) + '…' : name} · ${selectedEntries(r).length}/${total}`
  }

  function toggle(r: LinkRow, e: Entry) {
    if (!selectable(e, r)) return
    r.selected[e.id] = !r.selected[e.id]
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

  function optsChanged(r: LinkRow) {
    rememberOpts(r)
  }

  function skipChanged(r: LinkRow) {
    rememberOpts(r)
    applyScope(r)
  }

  /** Where this link's files land, relative to the Download folder. */
  function destLabel(col: Collection, dir: string): string {
    // Mirrors collectionDir in app_download.go.
    const sub =
      settings.value?.downloadSubfolders &&
      (['playlist', 'channel', 'album', 'profile'].includes(col.type) || (col.type === 'post' && col.entries.length > 1))
    if (!sub) return L('Langsung ke folder download', 'Straight into the download folder')
    const name = dir.split(/[\\/]/).pop() ?? ''
    return `${L('Subfolder', 'Subfolder')}: ${name}`
  }

  const breakdown = $derived.by(() => {
    const parts: string[] = []
    for (const r of dl.rows) {
      if (r.status !== 'ready' || !r.col) continue
      const n = toQueue(r).length
      if (!n) continue
      const what = isSocial(r.col.source) ? `item ${sourceName[r.col.source]}` : r.col.source === 'spotify' ? L(`lagu ${r.col.type === 'album' ? 'album' : 'Spotify'}`, `${r.col.type === 'album' ? 'album' : 'Spotify'} ${n === 1 ? 'track' : 'tracks'}`) : r.col.type === 'channel' ? 'channel' : r.col.type === 'playlist' ? 'playlist' : L('video', n === 1 ? 'video' : 'videos')
      parts.push(`${n} ${what}`)
    }
    return parts.join(' + ')
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

  const totalDur = $derived(entries.reduce((s, e) => s + (e.duration || 0), 0))
</script>

<PageHeader title="Download" subtitle={L('YouTube, TikTok, Instagram, Facebook & Spotify — tempel link-nya, jenisnya terdeteksi otomatis.', 'YouTube, TikTok, Instagram, Facebook & Spotify — paste the link and the type is detected automatically.')} />
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

      {#if dl.rows.length > 0}
        <div class="link-list">
          {#each dl.rows as r (r.id)}
            <div class="link-row">
              <span class="badge {sourceClass[r.link.source] ?? 'yt'}">
                <Icon name={r.link.source === 'spotify' ? 'music' : r.link.source === 'instagram' ? 'image' : 'play'} size={10} stroke={3} />{sourceName[r.link.source] ?? 'Web'}
              </span>
              <span class="ltype">{countLabel(r)}</span>
              <span class="lurl ellipsis" title={r.link.url}>{shortUrl(r.link.url)}</span>
              {#if r.status === 'loading'}
                <span class="lst load"><Icon name="loader" size={12} stroke={3} class="spin" />{L('Membaca…', 'Reading…')}</span>
              {:else if r.status === 'ready'}
                <span class="lst ok"><Icon name="check" size={12} stroke={3} />{L('Terbaca', 'Ready')}</span>
              {:else}
                <button class="lst err" title={r.error} onclick={() => showDetail(r.link.url, r.error, '')}>{L('Gagal', 'Failed')}</button>
                <button class="mini" aria-label={L('Coba lagi', 'Try again')} title={L('Coba lagi', 'Try again')} onclick={() => analyze(r.id)}><Icon name="refresh" size={14} /></button>
              {/if}
              <button class="mini" aria-label={L('Hapus link', 'Remove link')} title={L('Hapus link', 'Remove link')} onclick={() => removeRow(r.id)}><Icon name="x" size={14} /></button>
            </div>
          {/each}
        </div>
      {/if}
    </section>

    <section class="card content" aria-label={L('Isi link', 'Link contents')}>
      {#if dl.rows.length === 0}
        <div class="empty">
          <div class="big-ic"><Icon name="download" size={34} stroke={1.8} /></div>
          <h2>{L('Tempel link untuk mulai', 'Paste a link to start')}</h2>
          <p>{L('Bisa beberapa link sekaligus, satu per baris.', 'Several links at once work too, one per line.')}</p>
          <div class="examples">
            <div><span class="badge yt"><Icon name="play" size={10} stroke={3} />YouTube</span> {L('video · Shorts · playlist · channel (@nama)', 'videos · Shorts · playlists · channels (@name)')}</div>
            <div><span class="badge sp"><Icon name="music" size={10} stroke={3} />Spotify</span> {L('lagu · album · playlist', 'tracks · albums · playlists')}</div>
            <div><span class="badge tt"><Icon name="play" size={10} stroke={3} />TikTok</span> {L('video · foto slide + musik · profil', 'videos · photo slides + sound · profiles')}</div>
            <div><span class="badge ig"><Icon name="image" size={10} stroke={3} />Instagram</span> {L('reel · post foto & video · carousel', 'reels · photo & video posts · carousels')}</div>
            <div><span class="badge fb"><Icon name="play" size={10} stroke={3} />Facebook</span> {L('video · reel · foto', 'videos · reels · photos')}</div>
          </div>
        </div>
      {:else}
        <div class="tabs" role="tablist" aria-label={L('Link yang dibaca', 'Checked links')}>
          {#each dl.rows as r (r.id)}
            <button role="tab" aria-selected={row?.id === r.id} class:on={row?.id === r.id} onclick={() => (dl.active = r.id)}>
              <span class="dot {sourceClass[r.link.source] ?? 'yt'}"></span>{tabText(r)}
            </button>
          {/each}
        </div>

        {#if row?.status === 'loading'}
          <div class="state"><Icon name="loader" size={28} class="spin" /><span>{L('Membaca isi link…', 'Reading the link…')} {row.link.type === 'channel' ? L('Channel besar bisa butuh beberapa menit.', 'Large channels can take a few minutes.') : ''}</span></div>
        {:else if row?.status === 'error'}
          <div class="state err">
            <Icon name="alert" size={28} />
            <span>{row.error}</span>
            <button class="btn" onclick={() => analyze(row.id)}><Icon name="refresh" size={16} />{L('Coba lagi', 'Try again')}</button>
          </div>
        {:else if row?.col}
          {@const col = row.col}
          <div class="col-head">
            {#if col.type === 'channel'}
              {@const t = tile(col.title)}
              <div class="avatar" style="background: {t.bg}; color: {t.fg}">{initials(col.title)}</div>
            {:else if col.thumbnail}
              <img class="cover" class:sq={col.source === 'spotify' || social} src={col.thumbnail} alt="" referrerpolicy="no-referrer" onerror={(e) => ((e.currentTarget as HTMLImageElement).style.visibility = 'hidden')} />
            {:else}
              <div class="cover blank"><Icon name={col.source === 'spotify' ? 'music' : 'play'} /></div>
            {/if}
            <div class="ch-text">
              <span class="ch-title ellipsis" title={col.title}>{col.title || L('Tanpa judul', 'Untitled')}</span>
              <span class="ch-sub ellipsis">
                {#if col.type === 'channel'}
                  {L('Channel YouTube', 'YouTube channel')}{col.subtitle ? ` · ${col.subtitle}` : ''} · {col.entries.length} {L('konten', 'items')} ({Object.entries(col.tabCounts ?? {}).filter(([, n]) => n > 0).map(([k, n]) => `${n} ${tabLabel[k] ?? k}`).join(' · ')})
                {:else if social}
                  {sourceName[col.source]} {col.type === 'profile' ? typeLabel.profile.toLowerCase() : 'post'}{col.subtitle ? ` · ${col.subtitle}` : ''}{col.type === 'post' ? ` · ${kindSummary(entries)}` : ` · ${entries.length} video${L('', entries.length === 1 ? '' : 's')}`}
                {:else}
                  {col.source === 'spotify' ? L(`${typeLabel[col.type]} Spotify`, `Spotify ${typeLabel[col.type].toLowerCase()}`) : L(`${typeLabel[col.type]} YouTube`, `YouTube ${typeLabel[col.type].toLowerCase()}`)}{col.subtitle ? ` · ${col.subtitle}` : ''}{entries.length > 1 ? ` · ${entries.length} ${col.source === 'spotify' ? L('lagu', 'tracks') : L('video', 'videos')}` : ''}{totalDur ? ` · ± ${longDuration(totalDur)}` : ''}
                {/if}
              </span>
            </div>
            {#if col.type === 'playlist' || col.type === 'album'}
              <form class="range" onsubmit={(e) => { e.preventDefault(); onRange(row) }}>
                <label for="rentang">{L('Rentang', 'Range')}</label>
                <input id="rentang" class="text-input" bind:value={row.range} onblur={() => onRange(row)} />
              </form>
            {/if}
          </div>

          <div class="selbar">
            <input
              id="semua"
              type="checkbox"
              checked={selCount > 0 && selCount === selectableCount}
              indeterminate={selCount > 0 && selCount < selectableCount}
              onchange={(e) => setAll(row, (e.currentTarget as HTMLInputElement).checked)}
            />
            <label for="semua">{L('Pilih semua', 'Select all')}</label>
            <span class="selinfo">{L(`${selCount} dari ${entries.length} dipilih`, `${selCount} of ${entries.length} selected`)}{col.type === 'channel' ? L(' · urut dari terbaru', ' · newest first') : ''}</span>
            {#if archivedCount > 0}<span class="arch">{archivedCount} {L('sudah pernah diunduh · dilewati', 'downloaded before · skipped')}</span>{/if}
          </div>

          <div class="entries">
            {#each entries as e (e.id)}
              {@const tid = row.taskOf[e.id]}
              {@const t = tid ? tasks[tid] : undefined}
              {@const can = selectable(e, row)}
              <div class="entry" class:dim={!can} class:active={t?.status === 'running'}>
                <input type="checkbox" aria-label={L(`Pilih ${e.title}`, `Select ${e.title}`)} checked={!!row.selected[e.id] && can} disabled={!can} onchange={() => toggle(row, e)} />
                <span class="idx">{String(e.index).padStart(2, '0')}</span>
                {#if e.thumbnail}
                  <img class="th" class:sq={col.source === 'spotify' || e.kind === 'image'} src={e.thumbnail} alt="" loading="lazy" referrerpolicy="no-referrer" onerror={(ev) => ((ev.currentTarget as HTMLImageElement).style.visibility = 'hidden')} />
                {:else}
                  {@const tt = tile(e.title)}
                  <div class="th" class:sq={col.source === 'spotify'} style="background: {tt.bg}"></div>
                {/if}
                <div class="et">
                  <span class="etitle ellipsis" title={e.title}>{e.title}</span>
                  {#if t && (t.status === 'running' || t.status === 'queued') && t.message}
                    <span class="esub ellipsis">{t.message}</span>
                  {:else if e.artist}
                    <span class="esub ellipsis">{e.artist}{e.album ? ` · ${e.album}` : ''}</span>
                  {:else if e.unavailable}
                    <span class="esub">{L('Tidak tersedia (privat/dihapus)', 'Unavailable (private/deleted)')}</span>
                  {:else if social && col.entries.length > 1}
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
                    {:else if t.status === 'failed'}
                      <button class="pill err as-btn" title={t.message} onclick={() => showDetail(e.title, t.message, t.detail)}>{L('Gagal · detail', 'Failed · details')}</button>
                    {:else if t.status === 'skipped'}
                      <span class="pill muted" title={t.message}>{t.message || L('Dilewati', 'Skipped')}</span>
                    {:else}
                      <span class="pill muted">{L('Dibatalkan', 'Canceled')}</span>
                    {/if}
                  {:else}
                    {#if e.date && col.type === 'channel'}<span class="edate">{e.archived && row.opts.skipExisting ? L('Sudah ada', 'Already have') : ymd(e.date)}</span>{:else if e.archived && row.opts.skipExisting}<span class="edate">{L('Sudah ada', 'Already have')}</span>{/if}
                    <span class="edur">{duration(e.duration)}</span>
                  {/if}
                </div>
                {#if t && isActive(t)}
                  <button class="mini" aria-label={L('Batalkan', 'Cancel')} title={L('Batalkan', 'Cancel')} onclick={() => api.cancelTask(t.id)}><Icon name="x" size={14} /></button>
                {:else}
                  <span></span>
                {/if}
              </div>
            {/each}
          </div>
        {/if}
      {/if}
    </section>
  </div>

  <aside class="card panel" aria-label={L('Pengaturan download', 'Download settings')}>
    {#if row?.col && row.status === 'ready'}
      {@const col = row.col}
      <div class="ph">
        <span class="ph-k">{L('PENGATURAN UNTUK', 'SETTINGS FOR')}</span>
        <span class="ph-v ellipsis" title={col.title}>{social ? `${sourceName[col.source]} · ${col.subtitle || col.title}` : col.type === 'channel' ? `Channel ${col.subtitle || col.title}` : L(`${typeLabel[col.type]} ${col.source === 'spotify' ? 'Spotify' : 'YouTube'}`, `${col.source === 'spotify' ? 'Spotify' : 'YouTube'} ${typeLabel[col.type].toLowerCase()}`)}</span>
      </div>
      <div class="scroll">
        {#if col.source === 'spotify'}
          <div class="info">
            <Icon name="info" size={16} />
            <span>{L('Spotify selalu diunduh sebagai audio. Lagunya dicocokkan dari YouTube, lalu diberi judul, artis, album & cover dari Spotify.', 'Spotify is always downloaded as audio. Songs are matched on YouTube, then tagged with the title, artist, album & cover from Spotify.')}</span>
          </div>
          <div class="sec">
            <span class="label">{L('Format audio', 'Audio format')}</span>
            <Chips bind:value={row.opts.audioFormat} columns={3} onchange={() => optsChanged(row)} options={[{ value: 'mp3', label: 'MP3' }, { value: 'm4a', label: 'M4A' }, { value: 'opus', label: 'OPUS' }]} />
          </div>
          <div class="sec">
            <span class="label">{L('Kualitas', 'Quality')}</span>
            <Segmented label={L('Kualitas', 'Quality')} bind:value={row.opts.audioQuality} onchange={() => optsChanged(row)} options={[{ value: 'auto', label: L('Otomatis', 'Auto') }, { value: '192', label: '192 kbps' }, { value: '320', label: '320 kbps' }]} />
            <p class="hint">{L('Otomatis mengikuti kualitas sumber — angka lebih tinggi tidak membuat suara lebih bagus.', 'Auto follows the source quality — a higher number does not make it sound better.')}</p>
          </div>
          {#if col.type !== 'track'}
            <Switch bind:checked={row.opts.numbering} onchange={() => optsChanged(row)} label={L('Nomor urut di nama file', 'Track numbers in file names')} hint={L('Urutan sama seperti di Spotify', 'Same order as on Spotify')} />
          {/if}
        {:else}
          {#if col.type === 'channel'}
            <div class="sec">
              <span class="label">{L('Jenis konten', 'Content types')}</span>
              <div class="multi">
                {#each ['videos', 'shorts', 'streams'] as k}
                  {@const n = col.tabCounts?.[k] ?? 0}
                  <button class="mchip" class:on={row.types[k]} disabled={n === 0} aria-pressed={row.types[k]} onclick={() => { row.types[k] = !row.types[k]; applyScope(row) }}>
                    {tabLabel[k]}<span>{n}</span>
                  </button>
                {/each}
              </div>
            </div>
            <div class="sec">
              <span class="label">{L('Ambil yang mana', 'Which ones')}</span>
              <Segmented label={L('Cakupan', 'Scope')} bind:value={row.scope} onchange={() => applyScope(row)} options={[{ value: 'all', label: L('Semua', 'All') }, { value: 'latest', label: L('N terbaru', 'Latest N') }, { value: 'since', label: L('Sejak tanggal', 'Since date') }]} />
              {#if row.scope === 'latest'}
                <div class="inline">
                  <input class="text-input" aria-label={L('Jumlah terbaru', 'How many latest')} inputmode="numeric" value={row.latestN} oninput={(e) => { const v = parseInt((e.currentTarget as HTMLInputElement).value, 10); row.latestN = isNaN(v) ? 1 : Math.max(1, Math.min(5000, v)); applyScope(row) }} />
                  <span class="t12">{L('terbaru per jenis', 'latest per type')}</span>
                </div>
              {:else if row.scope === 'since'}
                <input class="text-input" type="date" aria-label={L('Sejak tanggal', 'Since date')} bind:value={row.since} onchange={() => applyScope(row)} />
                <p class="hint">{L('Tanggal dari YouTube bersifat perkiraan.', 'Dates from YouTube are approximate.')}</p>
              {/if}
            </div>
          {/if}

          {#if social && kinds.image}
            <div class="sec">
              <span class="label">{L('Format foto', 'Photo format')}</span>
              <Segmented label={L('Format foto', 'Photo format')} bind:value={row.opts.imageFormat} onchange={() => optsChanged(row)} options={[{ value: 'original', label: L('Asli', 'Original') }, { value: 'jpg', label: 'JPG' }]} />
              <p class="hint">{row.opts.imageFormat === 'jpg' ? L('WEBP/PNG/HEIC diubah ke JPG agar bisa dibuka di mana saja.', 'WEBP/PNG/HEIC are converted to JPG so they open anywhere.') : L('Foto disimpan apa adanya dari sumbernya.', 'Photos are saved exactly as the source provides them.')}</p>
            </div>
          {/if}

          {#if !social || kinds.video}
          <div class="sec">
            <span class="label">{L('Unduh sebagai', 'Download as')}</span>
            <Segmented label={L('Unduh sebagai', 'Download as')} bind:value={row.opts.mode} onchange={() => optsChanged(row)} options={[{ value: 'video', label: 'Video', icon: 'video' }, { value: 'audio', label: 'Audio', icon: 'music' }]} />
          </div>

          {#if row.opts.mode === 'video'}
            <div class="sec">
              <span class="label">{L('Kualitas video', 'Video quality')}</span>
              <Chips bind:value={row.opts.quality} columns={4} small onchange={() => optsChanged(row)} options={[{ value: 'best', label: L('Terbaik', 'Best') }, { value: '1080', label: '1080p' }, { value: '720', label: '720p' }, { value: '480', label: '480p' }]} />
            </div>
            <div class="sec">
              <span class="label">{L('Format file', 'File format')}</span>
              <Chips bind:value={row.opts.container} columns={2} onchange={() => optsChanged(row)} options={[{ value: 'mp4', label: 'MP4' }, { value: 'mkv', label: 'MKV' }]} />
            </div>
          {:else}
            <div class="sec">
              <span class="label">{L('Format audio', 'Audio format')}</span>
              <Chips bind:value={row.opts.audioFormat} columns={4} small onchange={() => optsChanged(row)} options={[{ value: 'mp3', label: 'MP3' }, { value: 'm4a', label: 'M4A' }, { value: 'opus', label: 'OPUS' }, { value: 'flac', label: 'FLAC' }]} />
            </div>
          {/if}
          {:else if kinds.audio}
            <div class="sec">
              <span class="label">{L('Format musik', 'Sound format')}</span>
              <Chips bind:value={row.opts.audioFormat} columns={4} small onchange={() => optsChanged(row)} options={[{ value: 'mp3', label: 'MP3' }, { value: 'm4a', label: 'M4A' }, { value: 'opus', label: 'OPUS' }, { value: 'flac', label: 'FLAC' }]} />
            </div>
          {/if}

          {#if !social || kinds.video || kinds.audio}
          <Switch bind:checked={row.opts.embed} onchange={() => optsChanged(row)} label={L('Sematkan info & thumbnail', 'Embed info & thumbnail')} hint={L('Judul, channel, dan gambar sampul', 'Title, channel and cover image')} />
          {/if}
          <Switch bind:checked={row.opts.skipExisting} onchange={() => skipChanged(row)} label={L('Lewati yang sudah ada', 'Skip existing')} hint={L('Unduh ulang hanya ambil yang baru', 'Downloading again only fetches new items')} />
          {#if col.type === 'playlist'}
            <Switch bind:checked={row.opts.numbering} onchange={() => optsChanged(row)} label={L('Nomor urut di nama file', 'Numbers in file names')} hint={L('01 - judul, 02 - judul, …', '01 - title, 02 - title, …')} />
          {/if}
        {/if}

        <div class="sec">
          <span class="label">{L('Simpan ke', 'Save to')}</span>
          <OutputPicker kind="download" />
          {#if dirOf[col.key]}
            <div class="dest">
              <span class="ellipsis" title={dirOf[col.key]}>{destLabel(col, dirOf[col.key])}</span>
              <button class="link" onclick={() => api.openFolder(dirOf[col.key])}>{L('Buka folder', 'Open folder')}</button>
            </div>
          {/if}
        </div>
      </div>
    {:else}
      <div class="ph">
        <span class="ph-k">{L('PENGATURAN', 'SETTINGS')}</span>
        <span class="ph-v">Download</span>
      </div>
      <div class="scroll">
        <p class="hint">{L('Tempel link lalu klik', 'Paste a link, then click')} <b>{L('Periksa link', 'Check link')}</b>. {L('Pengaturan kualitas dan format akan muncul di sini untuk setiap link.', 'Quality and format settings for each link show up here.')}</p>
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
  .link-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-height: 184px;
    overflow-y: auto;
  }
  .link-row {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 40px;
    padding: 0 4px 0 10px;
    border-radius: 10px;
    background: var(--surface-2);
    flex-shrink: 0;
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
    color: #ff9c9c;
  }
  .badge.sp {
    background: var(--ok-soft);
    color: var(--ok);
  }
  .badge.tt {
    background: #10302f;
    color: #6ff2ec;
  }
  .badge.ig {
    background: #3a1a2b;
    color: #ff8fbf;
  }
  .badge.fb {
    background: #172a40;
    color: #8ab8ff;
  }
  .ltype {
    width: 150px;
    flex-shrink: 0;
    font-size: 13px;
    font-weight: 700;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .lurl {
    flex-grow: 1;
    font-size: 12px;
    color: var(--text-3);
  }
  .lst {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    font-weight: 700;
    white-space: nowrap;
  }
  .lst.ok {
    color: var(--ok);
  }
  .lst.load {
    color: var(--accent-text-2);
  }
  .lst.err {
    color: var(--err);
    border: 0;
    background: none;
    padding: 0;
    text-decoration: underline dotted;
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
    background: #2a303b;
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
  .state {
    flex-grow: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    color: var(--text-2);
    font-size: 13px;
    padding: 24px;
    text-align: center;
  }
  .state.err {
    color: var(--err);
  }
  .col-head {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 12px 16px;
    border-bottom: 1px solid var(--border-soft);
    flex-shrink: 0;
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
  .entries {
    flex-grow: 1;
    min-height: 0;
    overflow-y: auto;
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
    background: #1c2029;
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
  .inline {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .inline input {
    width: 100px;
  }
  .t12 {
    font-size: 12px;
    color: var(--text-3);
  }
  input[type='date'] {
    color-scheme: dark;
  }
  .dest {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    font-size: 12px;
    color: var(--text-3);
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
