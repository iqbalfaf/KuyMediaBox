<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import PageHeader from '../components/PageHeader.svelte'
  import FileList from '../components/FileList.svelte'
  import EmptyDrop from '../components/EmptyDrop.svelte'
  import Chips from '../components/Chips.svelte'
  import Segmented from '../components/Segmented.svelte'
  import Select from '../components/Select.svelte'
  import OutputPicker from '../components/OutputPicker.svelte'
  import RunFooter from '../components/RunFooter.svelte'
  import ToolBanner from '../components/ToolBanner.svelte'
  import AudioFormatPanel from '../components/AudioFormatPanel.svelte'
  import Icon from '../components/Icon.svelte'
  import { api } from '../lib/api'
  import { caps, hasTool } from '../lib/stores/app.svelte'
  import { videoConv as conv } from '../lib/stores/converter.svelte'
  import { load, save } from '../lib/stores/persist'
  import { bytes, codec, duration, fps, tile } from '../lib/format'
  import { codecOptionLabel, crfFor, lossyAudio, maxCrf, outputSize, videoFormats } from '../lib/media'
  import type { AudioOptions, FileItem, VideoOptions } from '../lib/types'

  const defaults: { mode: 'video' | 'audio'; v: VideoOptions; a: AudioOptions } = {
    mode: 'video',
    v: { format: 'mp4', codec: 'h264', quality: 'seimbang', manual: false, crf: 23, preset: 'medium', resolution: '720', custom: 540 },
    a: { format: 'mp3', bitrate: 192, channels: 'source', sampleRate: 'source', keepMetadata: true },
  }
  const st = $state(load('kmb.video', defaults))
  $effect(() => save('kmb.video', $state.snapshot(st)))

  const available = (c: string) => c === 'copy' || !caps.ffmpeg || !!caps.encoders[c]
  const codecs = $derived(videoFormats[st.v.format] ?? [])

  // Keep the codec valid for the chosen container and installed encoders.
  $effect(() => {
    const list = videoFormats[st.v.format] ?? []
    if (st.v.format === 'gif') return
    if (!list.includes(st.v.codec) || !available(st.v.codec)) {
      st.v.codec = list.find((c) => c !== 'copy' && available(c)) ?? 'copy'
    }
  })
  $effect(() => {
    const m = maxCrf(st.v.codec)
    if (st.v.crf > m) st.v.crf = m
  })

  const isCopy = $derived(st.v.format !== 'gif' && st.v.codec === 'copy')
  const isGif = $derived(st.v.format === 'gif')
  const speed = $derived<Record<string, string>>({ fast: L('cepat', 'fast'), medium: L('sedang', 'medium'), slow: L('lambat', 'slow') })
  const qualityName = $derived<Record<string, string>>({ hemat: L('hemat', 'small'), seimbang: L('seimbang', 'balanced'), tinggi: L('tinggi', 'high') })

  function meta(it: FileItem): string {
    const p: string[] = []
    if (it.width) p.push(`${it.width}×${it.height}`)
    if (it.videoCodec) p.push(codec(it.videoCodec))
    if (it.fps) p.push(fps(it.fps))
    p.push(bytes(it.size))
    if (it.height > it.width) p.push(L('vertikal', 'vertical'))
    return p.join(' · ')
  }

  function resultLine(it: FileItem): [string, string] {
    if (st.mode === 'audio') {
      const f = st.a.format.toUpperCase()
      return [lossyAudio[st.a.format] ? `${f} · ${st.a.bitrate} kbps` : `${f} · lossless`, L('Audio saja', 'Audio only')]
    }
    if (isGif) {
      const [w, h] = outputSize(it.width, it.height, st.v)
      return [`GIF · ${w}×${h}`, '12 fps']
    }
    if (isCopy) return [`${st.v.format.toUpperCase()} · ${L('salin', 'copy')}`, L('Tanpa encode ulang', 'No re-encode')]
    const [w, h] = outputSize(it.width, it.height, st.v)
    const res = w && h ? `${Math.min(w, h)}p` : ''
    const q = st.v.manual ? `CRF ${crfFor(st.v)}` : `${L('Kualitas', 'Quality')} ${qualityName[st.v.quality] ?? st.v.quality}`
    return [`${st.v.format.toUpperCase()} · ${codec(st.v.codec)}${res ? ` · ${res}` : ''}`, w && h && (w !== it.width || h !== it.height) ? `${L('Menjadi', 'Becomes')} ${w}×${h}` : q]
  }

  const pending = $derived(conv.pending())
  const invalidCustom = $derived(st.mode === 'video' && !isCopy && st.v.resolution === 'custom' && !(st.v.custom >= 16))
  const noFFmpeg = $derived(!hasTool('ffmpeg'))

  function start() {
    conv.start(pending, (items) =>
      api.startVideo(items, { mode: st.mode, video: $state.snapshot(st.v), audio: $state.snapshot(st.a) }),
    )
  }

  const totalDuration = $derived(conv.items.reduce((s, it) => s + (it.duration || 0), 0))
</script>

<PageHeader title={L('Konversi Video', 'Video Converter')} subtitle={L('Ubah format, kecilkan ukuran, atau ambil audionya saja.', 'Change the format, shrink the size, or extract just the audio.')} />
<ToolBanner ids={['ffmpeg']} why={L('FFmpeg dibutuhkan untuk membaca dan mengonversi video.', 'FFmpeg is needed to read and convert video.')} />

<div class="body">
  {#if conv.items.length === 0}
    <EmptyDrop
      {conv}
      title={L('Tarik & lepas video ke sini', 'Drag & drop videos here')}
      subtitle={L('Bisa banyak file sekaligus, atau satu folder penuh.', 'Many files at once, or a whole folder.')}
      pickLabel={L('Pilih video', 'Choose videos')}
      formats={['MP4', 'MKV', 'MOV', 'AVI', 'WEBM', 'FLV', 'WMV', '3GP', 'TS']}
      steps={[
        [L('Tambahkan video', 'Add videos'), L('Tarik ke sini atau klik tombol', 'Drag them here or click the button')],
        [L('Pilih format & kualitas', 'Pick format & quality'), L('Di panel sebelah kanan', 'In the panel on the right')],
        [L('Klik Mulai', 'Click Start'), L('File asli tidak akan diubah', 'Your original files stay untouched')],
      ]}
    />
  {:else}
    <FileList
      {conv}
      noun={L('video', 'videos')}
      dropText={L('Tarik & lepas video di sini', 'Drag & drop videos here')}
      formats="MP4 · MKV · MOV · AVI · WEBM · FLV · WMV · 3GP"
      resultWidth={190}
      rowHeight={76}
      {meta}
    >
      {#snippet thumb(it)}
        {@const t = tile(it.name)}
        <div class="vthumb" style="background: {t.bg}; color: {t.fg}">
          <Icon name="play" size={18} />
          {#if it.duration}<span class="dur">{duration(it.duration)}</span>{/if}
        </div>
      {/snippet}
      {#snippet result(it)}
        {@const task = conv.task(it)}
        {@const s = conv.state(it)}
        {@const r = resultLine(it)}
        <span class="r1">{r[0]}</span>
        {#if s === 'done' && task}
          {@const diff = it.size ? Math.round(((task.outSize - it.size) / it.size) * 100) : 0}
          <span class="r2">{bytes(task.outSize)} <span class={diff <= 0 ? 'saved' : 'grew'}>{diff <= 0 ? `−${Math.abs(diff)}%` : `+${diff}%`}</span></span>
        {:else if s === 'running' && task?.message}
          <span class="r2">{task.message}</span>
        {:else}
          <span class="r2">{r[1]}</span>
        {/if}
      {/snippet}
    </FileList>
  {/if}

  <aside class="card panel" aria-label={L('Pengaturan output', 'Output settings')}>
    <div class="scroll">
      <Segmented
        label={L('Jenis hasil', 'Output type')}
        bind:value={st.mode}
        options={[
          { value: 'video', label: L('Jadi video', 'Video') },
          { value: 'audio', label: L('Ambil audio saja', 'Audio only') },
        ]}
      />

      {#if st.mode === 'video'}
        <div class="sec">
          <span class="label">Format</span>
          <Chips
            bind:value={st.v.format}
            columns={3}
            options={[
              { value: 'mp4', label: 'MP4' }, { value: 'mkv', label: 'MKV' }, { value: 'webm', label: 'WEBM' },
              { value: 'mov', label: 'MOV' }, { value: 'avi', label: 'AVI' }, { value: 'gif', label: 'GIF' },
            ]}
          />
        </div>

        {#if !isGif}
          <div class="sec">
            <span class="label">Codec</span>
            <Select
              label="Codec video"
              bind:value={st.v.codec}
              options={codecs.map((c) => ({ value: c, label: codecOptionLabel(c) + (available(c) ? '' : L(' — tidak tersedia', ' — not available')), disabled: !available(c) }))}
            />
            {#if isCopy}<p class="hint">{L('Sangat cepat & tanpa turun kualitas, tapi resolusi dan ukuran tetap. Hanya bisa jika codec sumber cocok dengan format tujuan.', 'Very fast with no quality loss, but resolution and size stay the same. Only works when the source codec fits the target format.')}</p>{/if}
          </div>
        {:else}
          <p class="hint">{L('GIF dibuat 12 fps dengan palet warna optimal. Cocok untuk klip pendek.', 'GIFs are made at 12 fps with an optimized palette. Best for short clips.')}</p>
        {/if}

        {#if !isCopy && !isGif}
          <div class="sec">
            <span class="label">{L('Kualitas', 'Quality')}</span>
            {#if !st.v.manual}
              <Segmented
                label={L('Kualitas', 'Quality')}
                bind:value={st.v.quality}
                options={[
                  { value: 'hemat', label: L('Hemat', 'Small') },
                  { value: 'seimbang', label: L('Seimbang', 'Balanced') },
                  { value: 'tinggi', label: L('Tinggi', 'High') },
                ]}
              />
            {:else}
              <div class="row-between">
                <label class="t12" for="crf">{L('CRF (angka kecil = lebih bagus & besar)', 'CRF (lower = better & bigger)')}</label>
                <span class="val">{st.v.crf}</span>
              </div>
              <input id="crf" type="range" min="10" max={maxCrf(st.v.codec)} bind:value={st.v.crf} />
              <Select
                label={L('Kecepatan encode', 'Encoding speed')}
                bind:value={st.v.preset}
                options={[
                  { value: 'fast', label: L('Cepat (file sedikit lebih besar)', 'Fast (slightly bigger file)') },
                  { value: 'medium', label: L('Sedang (disarankan)', 'Medium (recommended)') },
                  { value: 'slow', label: L('Lambat (file lebih kecil)', 'Slow (smaller file)') },
                ]}
              />
            {/if}
            <div class="row-between t12">
              <span>CRF {crfFor(st.v)} · {L('kecepatan', 'speed')} {speed[st.v.preset]}</span>
              <button
                class="link"
                onclick={() => {
                  if (!st.v.manual) st.v.crf = crfFor(st.v)
                  st.v.manual = !st.v.manual
                }}>{st.v.manual ? L('Pakai pilihan mudah', 'Use simple choices') : L('Atur manual', 'Set manually')}</button
              >
            </div>
          </div>
        {/if}

        {#if !isCopy}
          <div class="sec">
            <span class="label">{L('Resolusi', 'Resolution')}</span>
            <Chips
              bind:value={st.v.resolution}
              columns={5}
              small
              options={[
                { value: 'original', label: L('Asli', 'Original') }, { value: '1080', label: '1080p' }, { value: '720', label: '720p' },
                { value: '480', label: '480p' }, { value: 'custom', label: L('Lain', 'Other') },
              ]}
            />
            {#if st.v.resolution === 'custom'}
              <div class="num">
                <input
                  class="text-input"
                  aria-label={L('Sisi pendek dalam piksel', 'Short side in pixels')}
                  inputmode="numeric"
                  value={st.v.custom || ''}
                  oninput={(e) => {
                    const v = parseInt((e.currentTarget as HTMLInputElement).value.replace(/\D/g, ''), 10)
                    st.v.custom = isNaN(v) ? 0 : Math.min(4320, v)
                  }}
                /><span>{L('px sisi pendek', 'px short side')}</span>
              </div>
            {/if}
            <p class="hint">{L('Video tidak pernah diperbesar. Video vertikal ikut disesuaikan.', 'Videos are never upscaled. Vertical videos are handled too.')}</p>
          </div>
        {/if}
      {:else}
        <AudioFormatPanel bind:o={st.a} />
      {/if}

      <div class="sec">
        <span class="label">{L('Simpan ke', 'Save to')}</span>
        <OutputPicker kind="video" />
      </div>
    </div>

    <RunFooter
      kind="video"
      running={conv.running}
      busy={conv.starting}
      startLabel={conv.hasUnprocessed() || pending.length === 0 ? `${L('Mulai konversi', 'Start converting')}${pending.length ? ` (${pending.length})` : ''}` : `${L('Konversi ulang', 'Convert again')} (${pending.length})`}
      disabled={pending.length === 0 || invalidCustom || noFFmpeg}
      disabledHint={noFFmpeg ? L('Pasang FFmpeg dulu (lihat banner di atas).', 'Install FFmpeg first (see the banner above).') : conv.items.length === 0 ? L('Tambahkan video dulu untuk memulai.', 'Add videos to get started.') : invalidCustom ? L('Isi resolusi yang valid (minimal 16 px).', 'Enter a valid resolution (at least 16 px).') : ''}
      note={conv.items.length && !conv.running && totalDuration ? `${L('Total durasi', 'Total duration')} ${duration(totalDuration)}` : ''}
      lastOutput={conv.lastOutput()}
      queueMore={conv.running ? conv.fresh().filter((it) => !it.error).length : 0}
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
  .row-between {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
  }
  .t12 {
    font-size: 12px;
    color: var(--text-3);
  }
  .val {
    font-size: 14px;
    font-weight: 800;
  }
  input[type='range'] {
    width: 100%;
    margin: 0;
    accent-color: var(--accent);
  }
  .link {
    padding: 0;
    border: 0;
    background: none;
    font-size: 12px;
    font-weight: 700;
    color: var(--accent-text-2);
    white-space: nowrap;
  }
  .link:hover {
    color: var(--accent-text);
  }
  .num {
    position: relative;
  }
  .num input {
    padding-right: 110px;
    font-weight: 700;
  }
  .num span {
    position: absolute;
    right: 12px;
    top: 12px;
    font-size: 12px;
    color: var(--text-3);
    pointer-events: none;
  }
  .vthumb {
    width: 88px;
    height: 52px;
    flex-shrink: 0;
    border-radius: 8px;
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .dur {
    position: absolute;
    right: 4px;
    bottom: 4px;
    padding: 1px 5px;
    border-radius: 4px;
    background: var(--bg);
    color: var(--text);
    font-size: 10px;
    font-weight: 700;
    font-variant-numeric: tabular-nums;
  }
</style>
