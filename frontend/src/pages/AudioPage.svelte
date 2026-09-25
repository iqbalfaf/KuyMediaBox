<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import PageHeader from '../components/PageHeader.svelte'
  import FileList from '../components/FileList.svelte'
  import EmptyDrop from '../components/EmptyDrop.svelte'
  import Segmented from '../components/Segmented.svelte'
  import Select from '../components/Select.svelte'
  import Switch from '../components/Switch.svelte'
  import OutputPicker from '../components/OutputPicker.svelte'
  import RunFooter from '../components/RunFooter.svelte'
  import ToolBanner from '../components/ToolBanner.svelte'
  import AudioFormatPanel from '../components/AudioFormatPanel.svelte'
  import PresetBar from '../components/PresetBar.svelte'
  import TrimFields, { trimError } from '../components/TrimFields.svelte'
  import { editTags, tagsFromFile } from '../components/TagDialog.svelte'
  import Icon from '../components/Icon.svelte'
  import { api } from '../lib/api'
  import { hasTool } from '../lib/stores/app.svelte'
  import { audioConv as conv } from '../lib/stores/converter.svelte'
  import { load, save } from '../lib/stores/persist'
  import { bytes, codec, duration, khz, tile } from '../lib/format'
  import { lossyAudio } from '../lib/media'
  import type { AudioOptions, AudioTags, FileItem } from '../lib/types'

  const defaults: { mode: 'convert' | 'merge'; a: AudioOptions } = {
    mode: 'convert',
    a: {
      format: 'mp3', bitrate: 320, vbr: false, vbrLevel: 'high', channels: 'source', sampleRate: 'source', keepMetadata: true,
      trimStart: '', trimEnd: '', fadeIn: 0, fadeOut: 0, normalize: false, loudness: -16, removeSilence: false, speed: 1, pitch: 0,
    },
  }
  const st = $state(load('kmb.audio', defaults))
  $effect(() => save('kmb.audio', $state.snapshot(st)))

  /** Tags edited in this session, by item id (not saved: they belong to specific files). */
  let tagsOf = $state<Record<string, AudioTags>>({})

  const builtins = $derived([
    { id: 'b-mp3', name: 'MP3 320 kbps', value: { mode: 'convert', a: { format: 'mp3', bitrate: 320, normalize: false, speed: 1, pitch: 0, fadeIn: 0, fadeOut: 0, removeSilence: false } } },
    { id: 'b-pod', name: L('Podcast (MP3 128 mono, suara rata)', 'Podcast (MP3 128 mono, even volume)'), value: { mode: 'convert', a: { format: 'mp3', bitrate: 128, channels: 'mono', normalize: true, loudness: -16, removeSilence: true } } },
    { id: 'b-ring', name: L('Nada dering (M4A 30 dtk + fade)', 'Ringtone (M4A 30 s + fade)'), value: { mode: 'convert', a: { format: 'm4a', bitrate: 192, trimStart: '0:00', trimEnd: '0:30', fadeIn: 1, fadeOut: 3 } } },
    { id: 'b-flac', name: L('Arsip lossless (FLAC)', 'Lossless archive (FLAC)'), value: { mode: 'convert', a: { format: 'flac', normalize: false, speed: 1, pitch: 0 } } },
    { id: 'b-opus', name: L('Hemat kuota (Opus 96)', 'Data saver (Opus 96)'), value: { mode: 'convert', a: { format: 'opus', bitrate: 96 } } },
  ])

  const isOriginal = $derived(st.a.format === 'original')
  const merge = $derived(st.mode === 'merge')

  function meta(it: FileItem): string {
    const p: string[] = []
    if (it.hasVideo) {
      p.push(L('Audio dari video', 'Audio from video'))
      if (it.audioCodec) p.push(codec(it.audioCodec))
    } else {
      p.push(it.ext.toUpperCase())
      if (it.sampleRate) p.push(khz(it.sampleRate))
      if (it.bitsPerSample && !lossyAudio[it.ext]) p.push(`${it.bitsPerSample}-bit`)
    }
    if (it.duration) p.push(duration(it.duration))
    p.push(bytes(it.size))
    if (it.hasCover) p.push(L('ada cover', 'has cover'))
    return p.join(' · ')
  }

  function outLabel(it: FileItem): string {
    const f = isOriginal ? it.ext.toUpperCase() : st.a.format.toUpperCase()
    if (isOriginal) return `${f} · ${L('salin', 'copy')}`
    if (!lossyAudio[st.a.format]) return `${f} · lossless`
    return st.a.vbr && st.a.format !== 'm4a' ? `${f} · VBR` : `${f} · ${st.a.bitrate} kbps`
  }

  function effects(): string {
    const e: string[] = []
    if (st.a.trimStart || st.a.trimEnd) e.push(L('dipotong', 'trimmed'))
    if (st.a.fadeIn || st.a.fadeOut) e.push('fade')
    if (st.a.normalize) e.push(`${st.a.loudness} LUFS`)
    if (st.a.removeSilence) e.push(L('tanpa hening', 'no silence'))
    if (st.a.speed !== 1) e.push(`${st.a.speed}×`)
    if (st.a.pitch) e.push(`${st.a.pitch > 0 ? '+' : ''}${st.a.pitch} ${L('nada', 'st')}`)
    return e.join(' · ')
  }

  function openTags(it: FileItem) {
    const current = tagsOf[it.id] ?? tagsFromFile(it)
    editTags(it, current, conv.items.length, (t, all) => {
      if (!all) {
        tagsOf[it.id] = t
        return
      }
      // Shared fields go to every file; titles and track numbers stay per file.
      tagsOf[it.id] = t
      for (const other of conv.items) {
        if (other.id === it.id) continue
        const base = tagsOf[other.id] ?? tagsFromFile(other)
        tagsOf[other.id] = { ...base, artist: t.artist, albumArtist: t.albumArtist, album: t.album, year: t.year, genre: t.genre, cover: t.cover, removeCover: t.removeCover }
      }
    })
  }

  const hasEffects = $derived(st.a.normalize || st.a.fadeIn > 0 || st.a.fadeOut > 0 || st.a.speed !== 1 || st.a.pitch !== 0 || st.a.channels !== 'source' || st.a.sampleRate !== 'source')
  const pending = $derived(conv.pending())
  const noFFmpeg = $derived(!hasTool('ffmpeg'))
  const trimBad = $derived(!merge && trimError(st.a.trimStart, st.a.trimEnd) !== '')
  const mergeFew = $derived(merge && conv.items.filter((it) => !it.error).length < 2)
  const originalFx = $derived(isOriginal && hasEffects)

  function start() {
    const a = $state.snapshot(st.a)
    if (merge) {
      const all = conv.items.filter((it) => !it.error)
      conv.start(all, (items) => api.startAudio(items, { mode: 'merge', options: a }))
      return
    }
    conv.start(pending, (items) =>
      api.startAudio(
        items.map((it) => ({ ...it, tags: tagsOf[it.id] ? $state.snapshot(tagsOf[it.id]) : null })),
        { mode: 'convert', options: a },
      ),
    )
  }

  $effect(() => {
    if (merge && isOriginal) st.a.format = 'mp3'
  })
</script>

<PageHeader title={L('Konversi Audio', 'Audio Converter')} subtitle={L('Ubah format & kualitas, potong, ratakan volume, edit tag, atau gabungkan audio.', 'Change format & quality, trim, even out volume, edit tags or join audio.')} />
<ToolBanner ids={['ffmpeg']} why={L('FFmpeg dibutuhkan untuk membaca dan mengonversi audio.', 'FFmpeg is needed to read and convert audio.')} />

<div class="body">
  {#if conv.items.length === 0}
    <EmptyDrop
      {conv}
      title={L('Tarik & lepas audio ke sini', 'Drag & drop audio here')}
      subtitle={L('File audio atau video — audionya yang diambil.', 'Audio or video files — the audio is extracted.')}
      pickLabel={L('Pilih audio', 'Choose audio')}
      formats={['MP3', 'WAV', 'FLAC', 'M4A', 'OGG', 'OPUS', 'WMA', 'AAC', 'MP4']}
      steps={[
        [L('Tambahkan audio', 'Add audio'), L('Tarik ke sini atau klik tombol', 'Drag it here or click the button')],
        [L('Pilih format & bitrate', 'Pick format & bitrate'), L('Di panel sebelah kanan', 'In the panel on the right')],
        [L('Klik Mulai', 'Click Start'), L('File asli tidak akan diubah', 'Your original files stay untouched')],
      ]}
    />
  {:else}
    <FileList
      {conv}
      noun={L('file', 'files')}
      dropText={L('Tarik & lepas audio atau video di sini', 'Drag & drop audio or video here')}
      formats="MP3 · WAV · FLAC · M4A · OGG · OPUS · WMA"
      resultWidth={170}
      reorder={merge}
      {meta}
    >
      {#snippet thumb(it)}
        {@const t = tile(it.name)}
        <div class="kmb-tile" style="background: {t.bg}; color: {t.fg}; border-radius: {it.hasVideo ? '8px' : '50%'}">
          <Icon name={it.hasVideo ? 'video' : 'music'} />
        </div>
      {/snippet}
      {#snippet result(it)}
        {@const task = conv.task(it)}
        {@const s = conv.state(it)}
        <span class="r1">{outLabel(it)}</span>
        {#if s === 'done' && task}
          <span class="r2">{bytes(task.outSize)}</span>
        {:else if s === 'running' && task?.message}
          <span class="r2">{task.message}</span>
        {:else if tagsOf[it.id]}
          <span class="r2 tagged"><Icon name="tag" size={11} />{tagsOf[it.id].artist || tagsOf[it.id].title ? `${tagsOf[it.id].artist}${tagsOf[it.id].artist && tagsOf[it.id].title ? ' – ' : ''}${tagsOf[it.id].title}` : L('tag diubah', 'tags edited')}</span>
        {:else if effects()}
          <span class="r2">{effects()}</span>
        {/if}
      {/snippet}
      {#snippet actions(it)}
        {#if !merge}
          <button class="mini" class:on={!!tagsOf[it.id]} aria-label={L('Edit tag', 'Edit tags')} title={L('Edit tag (judul, artis, album, cover)', 'Edit tags (title, artist, album, cover)')} onclick={() => openTags(it)}><Icon name="tag" size={15} /></button>
        {/if}
      {/snippet}
    </FileList>
  {/if}

  <aside class="card panel" aria-label={L('Pengaturan output', 'Output settings')}>
    <div class="scroll">
      <PresetBar module="audio" target={st} {builtins} />

      <Segmented
        label={L('Mode', 'Mode')}
        bind:value={st.mode}
        options={[
          { value: 'convert', label: L('Konversi', 'Convert') },
          { value: 'merge', label: L('Gabung jadi satu', 'Join into one') },
        ]}
      />
      {#if merge}<p class="hint">{L('Semua file digabung sesuai urutan daftar (atur dengan ↑↓).', 'All files are joined in list order (use ↑↓).')}</p>{/if}

      <AudioFormatPanel bind:o={st.a} allowOriginal={!merge} />

      {#if !merge}
        <div class="sec">
          <TrimFields bind:start={st.a.trimStart} bind:end={st.a.trimEnd} label={L('Potong', 'Trim')} />
        </div>
      {/if}

      {#if !isOriginal}
        <div class="sec">
          <span class="label">Fade</span>
          <div class="grid2">
            <label class="fld"><span>Fade in · {st.a.fadeIn} {L('dtk', 's')}</span><input type="range" min="0" max="10" step="0.5" bind:value={st.a.fadeIn} /></label>
            <label class="fld"><span>Fade out · {st.a.fadeOut} {L('dtk', 's')}</span><input type="range" min="0" max="10" step="0.5" bind:value={st.a.fadeOut} /></label>
          </div>
        </div>

        <div class="sec">
          <Switch bind:checked={st.a.normalize} label={L('Ratakan volume', 'Normalize volume')} hint={L('Standar EBU R128 (loudnorm): semua lagu sama kerasnya', 'EBU R128 (loudnorm): every track equally loud')} />
          {#if st.a.normalize}
            <Select
              label={L('Target kekerasan', 'Loudness target')}
              bind:value={st.a.loudness}
              options={[
                { value: -14, label: L('-14 LUFS · streaming (Spotify, YouTube)', '-14 LUFS · streaming (Spotify, YouTube)') },
                { value: -16, label: L('-16 LUFS · podcast (disarankan)', '-16 LUFS · podcast (recommended)') },
                { value: -18, label: '-18 LUFS' },
                { value: -23, label: L('-23 LUFS · siaran TV/radio', '-23 LUFS · TV/radio broadcast') },
              ]}
            />
          {/if}
        </div>

        <Switch bind:checked={st.a.removeSilence} label={L('Hapus hening awal & akhir', 'Remove silence at start & end')} hint={L('Bagian sunyi di ujung file dibuang', 'Quiet parts at the ends are cut off')} />

        <div class="sec">
          <span class="label">{L('Kecepatan & nada', 'Speed & pitch')}</span>
          <label class="fld">
            <span>{L('Kecepatan', 'Speed')} · {st.a.speed.toFixed(2)}×</span>
            <input type="range" min="0.5" max="2" step="0.05" bind:value={st.a.speed} />
          </label>
          <label class="fld">
            <span>{L('Nada', 'Pitch')} · {st.a.pitch > 0 ? '+' : ''}{st.a.pitch} {L('semitone', 'semitones')}</span>
            <input type="range" min="-12" max="12" step="1" bind:value={st.a.pitch} />
          </label>
          {#if st.a.speed !== 1 || st.a.pitch !== 0}
            <button class="link" onclick={() => { st.a.speed = 1; st.a.pitch = 0 }}>{L('Kembalikan normal', 'Back to normal')}</button>
          {/if}
          <p class="hint">{L('Kecepatan tidak mengubah nada, dan sebaliknya.', 'Speed does not change the pitch, and vice versa.')}</p>
        </div>

        <div class="sec">
          <span class="label">Channel</span>
          <Segmented
            label="Channel"
            bind:value={st.a.channels}
            options={[
              { value: 'source', label: L('Ikuti asli', 'Keep original') },
              { value: 'stereo', label: 'Stereo' },
              { value: 'mono', label: 'Mono' },
            ]}
          />
        </div>

        <div class="sec">
          <span class="label">Sample rate</span>
          <Select
            label="Sample rate"
            bind:value={st.a.sampleRate}
            options={[
              { value: 'source', label: L('Ikuti file asli', 'Same as source') },
              { value: '44100', label: '44,1 kHz (CD)' },
              { value: '48000', label: '48 kHz (video)' },
            ]}
          />
          {#if st.a.format === 'opus' && st.a.sampleRate === '44100'}<p class="hint">{L('Opus selalu memakai 48 kHz.', 'Opus always uses 48 kHz.')}</p>{/if}
        </div>
      {/if}

      {#if !merge}
        <Switch bind:checked={st.a.keepMetadata} label={L('Pertahankan info lagu', 'Keep song info')} hint={L('Judul, artis, album & cover. Klik ikon tag di daftar untuk mengeditnya.', 'Title, artist, album & cover. Click the tag icon in the list to edit them.')} />
      {/if}

      <div class="sec">
        <span class="label">{L('Simpan ke', 'Save to')}</span>
        <OutputPicker kind="audio" />
      </div>
    </div>

    <RunFooter
      kind="audio"
      running={conv.running}
      busy={conv.starting}
      startLabel={merge
        ? `${L('Gabungkan', 'Join')} ${conv.items.filter((it) => !it.error).length} file`
        : conv.hasUnprocessed() || pending.length === 0
          ? `${L('Mulai konversi', 'Start converting')}${pending.length ? ` (${pending.length})` : ''}`
          : `${L('Konversi ulang', 'Convert again')} (${pending.length})`}
      disabled={(merge ? mergeFew : pending.length === 0) || noFFmpeg || trimBad || originalFx}
      disabledHint={noFFmpeg
        ? L('Pasang FFmpeg dulu (lihat banner di atas).', 'Install FFmpeg first (see the banner above).')
        : conv.items.length === 0
          ? L('Tambahkan audio dulu untuk memulai.', 'Add audio to get started.')
          : mergeFew
            ? L('Tambahkan minimal 2 file untuk digabung.', 'Add at least 2 files to join.')
            : trimBad
              ? L('Perbaiki waktu potong.', 'Fix the trim times.')
              : originalFx
                ? L('Format asli hanya menyalin audio: matikan efek atau pilih format lain.', 'Original format only copies the audio: turn off effects or pick another format.')
                : ''}
      lastOutput={conv.lastOutput()}
      queueMore={conv.running && !merge ? conv.fresh().filter((it) => !it.error).length : 0}
      onstart={start}
      oncancel={() => conv.cancelAll()}
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
  .panel {
    width: 344px;
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
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
  .grid2 {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px;
  }
  .fld {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 12px;
    color: var(--text-3);
  }
  input[type='range'] {
    width: 100%;
    margin: 0;
    accent-color: var(--accent);
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
  .mini.on {
    color: var(--accent);
  }
  .tagged {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    color: var(--accent-text-2) !important;
  }
</style>
