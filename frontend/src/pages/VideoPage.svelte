<script lang="ts">
  import { L, locale } from '../lib/i18n.svelte'
  import { onMount } from 'svelte'
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
  import PresetBar from '../components/PresetBar.svelte'
  import TrimFields, { trimError } from '../components/TrimFields.svelte'
  import Icon from '../components/Icon.svelte'
  import { api } from '../lib/api'
  import { caps, hasTool } from '../lib/stores/app.svelte'
  import { videoConv as conv } from '../lib/stores/converter.svelte'
  import { load, save } from '../lib/stores/persist'
  import { bytes, codec, duration, fps, tile } from '../lib/format'
  import { codecOptionLabel, crfFor, lossyAudio, maxCrf, outputSize, videoFormats } from '../lib/media'
  import type { AudioOptions, FileItem, VideoOptions } from '../lib/types'

  type Mode = 'video' | 'audio' | 'merge' | 'frames'
  const defaults: { mode: Mode; v: VideoOptions; a: AudioOptions; frameEvery: number; frameFormat: 'jpg' | 'png' } = {
    mode: 'video',
    v: {
      format: 'mp4', codec: 'h264', quality: 'seimbang', manual: false, crf: 23, preset: 'medium', resolution: '720', custom: 540,
      targetMB: 0, bitrateK: 0, hw: '', trimStart: '', trimEnd: '', fps: 'original', fpsCustom: 25, audioMode: 'auto', audioBitrate: 128,
      rotate: 0, flipH: false, flipV: false, subtitles: 'none',
    },
    a: {
      format: 'mp3', bitrate: 192, vbr: false, vbrLevel: 'high', channels: 'source', sampleRate: 'source', keepMetadata: true,
      trimStart: '', trimEnd: '', fadeIn: 0, fadeOut: 0, normalize: false, loudness: -16, removeSilence: false, speed: 1, pitch: 0,
    },
    frameEvery: 5,
    frameFormat: 'jpg',
  }
  const st = $state(load('kmb.video', defaults))
  $effect(() => save('kmb.video', $state.snapshot(st)))

  const builtins = $derived([
    { id: 'b-wa', name: L('WhatsApp (maks. 16 MB, 720p)', 'WhatsApp (max 16 MB, 720p)'), value: { mode: 'video', v: { format: 'mp4', codec: 'h264', resolution: '720', targetMB: 15, fps: '30', audioMode: 'aac', audioBitrate: 96 } } },
    { id: 'b-discord', name: L('Discord (maks. 10 MB)', 'Discord (max 10 MB)'), value: { mode: 'video', v: { format: 'mp4', codec: 'h264', resolution: '720', targetMB: 9.5, audioMode: 'aac', audioBitrate: 96 } } },
    { id: 'b-email', name: L('Email (maks. 8 MB, 480p)', 'Email (max 8 MB, 480p)'), value: { mode: 'video', v: { format: 'mp4', codec: 'h264', resolution: '480', targetMB: 7.5, audioMode: 'aac', audioBitrate: 64 } } },
    { id: 'b-yt', name: L('Upload YouTube 1080p', 'YouTube upload 1080p'), value: { mode: 'video', v: { format: 'mp4', codec: 'h264', resolution: '1080', targetMB: 0, quality: 'tinggi', manual: false, audioMode: 'aac', audioBitrate: 192 } } },
    { id: 'b-gif', name: L('GIF pendek (480p, 12 fps)', 'Short GIF (480p, 12 fps)'), value: { mode: 'video', v: { format: 'gif', resolution: '480', fps: '12' } } },
    { id: 'b-mp3', name: L('Ambil lagu (MP3 320)', 'Grab the song (MP3 320)'), value: { mode: 'audio', a: { format: 'mp3', bitrate: 320 } } },
  ])

  // GPU encoders are tested once per session (a few seconds), when the page opens.
  let hw = $state<Record<string, string[]>>({})
  let hwChecking = $state(false)
  let hwChecked = $state(false)
  async function checkHW() {
    if (hwChecking || !caps.ffmpeg) return
    hwChecking = true
    try {
      hw = (await api.getHWEncoders()) ?? {}
    } catch {
      hw = {}
    } finally {
      hwChecking = false
      hwChecked = true
    }
  }
  onMount(() => {
    if (st.v.hw) checkHW()
  })
  const hwName: Record<string, string> = { nvenc: 'NVIDIA NVENC', qsv: 'Intel Quick Sync', amf: 'AMD AMF' }
  const hwFor = (codecName: string) => Object.keys(hw).filter((k) => hw[k]?.includes(codecName))

  const available = (c: string) => c === 'copy' || !caps.ffmpeg || !!caps.encoders[c] || (!!st.v.hw && !!hw[st.v.hw]?.includes(c))
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
  $effect(() => {
    // A GPU choice that doesn't support the codec falls back to the processor.
    if (st.v.hw && hwChecked && !hw[st.v.hw]?.includes(st.v.codec)) st.v.hw = ''
  })

  const isCopy = $derived(st.v.format !== 'gif' && st.v.codec === 'copy')
  const isGif = $derived(st.v.format === 'gif')
  const encodes = $derived(st.mode === 'merge' || (st.mode === 'video' && !isCopy && !isGif))
  const speed = $derived<Record<string, string>>({ fast: L('cepat', 'fast'), medium: L('sedang', 'medium'), slow: L('lambat', 'slow') })
  const qualityName = $derived<Record<string, string>>({ hemat: L('hemat', 'small'), seimbang: L('seimbang', 'balanced'), tinggi: L('tinggi', 'high') })

  function meta(it: FileItem): string {
    const p: string[] = []
    if (it.width) p.push(`${it.width}×${it.height}`)
    if (it.videoCodec) p.push(codec(it.videoCodec))
    if (it.fps) p.push(fps(it.fps))
    p.push(bytes(it.size))
    if (it.height > it.width) p.push(L('vertikal', 'vertical'))
    if (it.subFile || it.subCodec) p.push(L('ada subtitle', 'has subtitles'))
    return p.join(' · ')
  }

  function fpsLabel(): string {
    if (st.v.fps === 'custom') return `${st.v.fpsCustom} fps`
    if (st.v.fps !== 'original') return `${st.v.fps} fps`
    return isGif ? '12 fps' : ''
  }

  function resultLine(it: FileItem): [string, string] {
    if (st.mode === 'audio') {
      const f = st.a.format.toUpperCase()
      return [lossyAudio[st.a.format] ? (st.a.vbr && st.a.format !== 'm4a' ? `${f} · VBR` : `${f} · ${st.a.bitrate} kbps`) : `${f} · lossless`, L('Audio saja', 'Audio only')]
    }
    if (st.mode === 'frames') return [`${st.frameFormat.toUpperCase()} · ${L('tiap', 'every')} ${st.frameEvery} ${L('dtk', 's')}`, L('Folder berisi gambar', 'A folder of pictures')]
    if (isGif) {
      const [w, h] = outputSize(it.width, it.height, st.v)
      return [`GIF · ${w}×${h}`, fpsLabel()]
    }
    if (isCopy) return [`${st.v.format.toUpperCase()} · ${L('salin', 'copy')}`, L('Tanpa encode ulang', 'No re-encode')]
    const [w, h] = outputSize(it.width, it.height, st.v)
    const res = w && h ? `${Math.min(w, h)}p` : ''
    const q = st.v.targetMB > 0 ? `≤ ${st.v.targetMB} MB` : st.v.bitrateK > 0 ? `${st.v.bitrateK} kbps` : st.v.manual ? `CRF ${crfFor(st.v)}` : `${L('Kualitas', 'Quality')} ${qualityName[st.v.quality] ?? st.v.quality}`
    const gpu = st.v.hw ? ` · GPU` : ''
    return [`${st.v.format.toUpperCase()} · ${codec(st.v.codec)}${res ? ` · ${res}` : ''}${gpu}`, w && h && (w !== it.width || h !== it.height) ? `${L('Menjadi', 'Becomes')} ${w}×${h} · ${q}` : q]
  }

  const pending = $derived(conv.pending())
  const invalidCustom = $derived(encodes && st.v.resolution === 'custom' && !(st.v.custom >= 16))
  const trimBad = $derived(st.mode !== 'merge' && trimError(st.v.trimStart, st.v.trimEnd) !== '')
  const noFFmpeg = $derived(!hasTool('ffmpeg'))
  const mergeFew = $derived(st.mode === 'merge' && conv.items.filter((it) => !it.error).length < 2)

  function start() {
    const job = {
      mode: st.mode,
      video: $state.snapshot(st.v),
      audio: $state.snapshot(st.a),
      frameEvery: st.frameEvery,
      frameFormat: st.frameFormat,
    }
    if (st.mode === 'merge') {
      // Joining uses every file in list order.
      const all = conv.items.filter((it) => !it.error)
      conv.start(all, (items) => api.startVideo(items, job))
      return
    }
    conv.start(pending, (items) => api.startVideo(items, job))
  }

  const totalDuration = $derived(conv.items.reduce((s, it) => s + (it.duration || 0), 0))
  const startLabel = $derived.by(() => {
    if (st.mode === 'merge') return `${L('Gabungkan', 'Join')} ${conv.items.filter((it) => !it.error).length} video`
    if (st.mode === 'frames') return `${L('Ambil frame', 'Export frames')}${pending.length ? ` (${pending.length})` : ''}`
    return conv.hasUnprocessed() || pending.length === 0 ? `${L('Mulai konversi', 'Start converting')}${pending.length ? ` (${pending.length})` : ''}` : `${L('Konversi ulang', 'Convert again')} (${pending.length})`
  })
</script>

<PageHeader title={L('Konversi Video', 'Video Converter')} subtitle={L('Ubah format, kecilkan ukuran, potong, gabung, atau ambil audionya saja.', 'Change the format, shrink, trim, join, or extract just the audio.')} />
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
      resultWidth={200}
      rowHeight={76}
      reorder={st.mode === 'merge'}
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
          {#if st.mode === 'merge' || st.mode === 'frames'}
            <span class="r2">{bytes(task.outSize)}{task.message ? ` · ${task.message}` : ''}</span>
          {:else}
            {@const diff = it.size ? Math.round(((task.outSize - it.size) / it.size) * 100) : 0}
            <span class="r2">{bytes(task.outSize)} <span class={diff <= 0 ? 'saved' : 'grew'}>{diff <= 0 ? `−${Math.abs(diff)}%` : `+${diff}%`}</span></span>
          {/if}
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
      <PresetBar module="video" target={st} {builtins} />

      <Segmented
        label={L('Jenis hasil', 'Output type')}
        bind:value={st.mode}
        options={[
          { value: 'video', label: 'Video' },
          { value: 'audio', label: 'Audio' },
          { value: 'merge', label: L('Gabung', 'Join') },
          { value: 'frames', label: 'Frame' },
        ]}
      />
      {#if st.mode === 'merge'}
        <p class="hint">{L('Semua video di daftar digabung jadi satu, sesuai urutan (atur dengan tombol ↑↓). Ukuran & fps mengikuti video pertama.', 'All videos in the list are joined into one, in list order (use ↑↓). Size & fps follow the first video.')}</p>
      {:else if st.mode === 'frames'}
        <p class="hint">{L('Setiap video menghasilkan folder berisi gambar.', 'Each video produces a folder of pictures.')}</p>
      {/if}

      {#if st.mode === 'video' || st.mode === 'merge'}
        <div class="sec">
          <span class="label">Format</span>
          <Chips
            bind:value={st.v.format}
            columns={3}
            options={[
              { value: 'mp4', label: 'MP4' }, { value: 'mkv', label: 'MKV' }, { value: 'webm', label: 'WEBM' },
              { value: 'mov', label: 'MOV' }, { value: 'avi', label: 'AVI' }, { value: 'gif', label: 'GIF', disabled: st.mode === 'merge' },
            ]}
          />
        </div>

        {#if !isGif}
          <div class="sec">
            <span class="label">Codec</span>
            <Select
              label="Codec video"
              bind:value={st.v.codec}
              options={codecs
                .filter((c) => st.mode !== 'merge' || c !== 'copy')
                .map((c) => ({ value: c, label: codecOptionLabel(c) + (available(c) ? '' : L(' — tidak tersedia', ' — not available')), disabled: !available(c) }))}
            />
            {#if isCopy}<p class="hint">{L('Sangat cepat & tanpa turun kualitas, tapi resolusi dan ukuran tetap. Hanya bisa jika codec sumber cocok dengan format tujuan.', 'Very fast with no quality loss, but resolution and size stay the same. Only works when the source codec fits the target format.')}</p>{/if}
          </div>
        {/if}

        {#if encodes}
          <div class="sec">
            <div class="row-between">
              <span class="label">{L('Akselerasi GPU', 'GPU acceleration')}</span>
              {#if !hwChecked}
                <button class="link" onclick={checkHW} disabled={hwChecking}>{hwChecking ? L('Memeriksa…', 'Checking…') : L('Periksa GPU', 'Check GPU')}</button>
              {/if}
            </div>
            {#if hwChecked}
              <Select
                label={L('Akselerasi GPU', 'GPU acceleration')}
                bind:value={st.v.hw}
                options={[
                  { value: '', label: L('Tidak — pakai prosesor (kualitas terbaik)', 'No — use the processor (best quality)') },
                  ...(['nvenc', 'qsv', 'amf'] as const).map((k) => ({
                    value: k,
                    label: hwName[k] + (hwFor(st.v.codec).includes(k) ? '' : hw[k]?.length ? L(` — tidak untuk ${codec(st.v.codec)}`, ` — not for ${codec(st.v.codec)}`) : L(' — tidak ada', ' — not found')),
                    disabled: !hwFor(st.v.codec).includes(k),
                  })),
                ]}
              />
              <p class="hint">{st.v.hw ? L('Jauh lebih cepat; file sedikit lebih besar pada kualitas yang sama.', 'Much faster; files are a little bigger at the same quality.') : Object.keys(hw).length ? L('GPU tersedia: ', 'GPU available: ') + Object.keys(hw).map((k) => hwName[k]).join(', ') : L('Tidak ada encoder GPU yang bisa dipakai di PC ini.', 'No usable GPU encoder on this PC.')}</p>
            {:else}
              <p class="hint">{L('Pakai kartu grafis (NVIDIA/Intel/AMD) untuk encode lebih cepat.', 'Use the graphics card (NVIDIA/Intel/AMD) to encode faster.')}</p>
            {/if}
          </div>

          <div class="sec">
            <span class="label">{L('Kualitas', 'Quality')}</span>
            <Segmented
              label={L('Cara menentukan kualitas', 'How quality is set')}
              value={st.v.targetMB > 0 ? 'size' : st.v.bitrateK > 0 ? 'bitrate' : 'quality'}
              onchange={(v) => {
                st.v.targetMB = v === 'size' ? 16 : 0
                st.v.bitrateK = v === 'bitrate' ? 2500 : 0
              }}
              options={[
                { value: 'quality', label: L('Kualitas', 'Quality') },
                { value: 'size', label: L('Ukuran', 'Size') },
                { value: 'bitrate', label: 'Bitrate' },
              ]}
            />
            {#if st.v.bitrateK > 0}
              <div class="inline">
                <div class="num kbps">
                  <input
                    class="text-input"
                    aria-label={L('Bitrate video (kbps)', 'Video bitrate (kbps)')}
                    inputmode="numeric"
                    value={st.v.bitrateK}
                    oninput={(e) => {
                      const v = parseInt((e.currentTarget as HTMLInputElement).value.replace(/\D/g, ''), 10)
                      st.v.bitrateK = isNaN(v) || v < 50 ? 50 : Math.min(200000, v)
                    }}
                  /><span>kbps</span>
                </div>
                {#each [1000, 2500, 5000, 8000] as k}
                  <button class="mchip" class:on={st.v.bitrateK === k} onclick={() => (st.v.bitrateK = k)}>{k >= 1000 ? `${k / 1000}M` : k}</button>
                {/each}
              </div>
              <p class="hint">
                ≈ {((st.v.bitrateK * 60) / 8 / 1000).toLocaleString(locale(), { maximumFractionDigits: 1 })} MB {L('video per menit', 'of video per minute')}.
                {L('Kira-kira: 720p 2.500 · 1080p 5.000 · 4K 15.000 kbps. Satu tahap, rata-rata bitrate dijaga.', 'Roughly: 720p 2,500 · 1080p 5,000 · 4K 15,000 kbps. One pass, the average bitrate is kept.')}
              </p>
            {:else if st.v.targetMB > 0}
              <div class="inline">
                <div class="num">
                  <input
                    class="text-input"
                    aria-label={L('Ukuran target (MB)', 'Target size (MB)')}
                    inputmode="decimal"
                    value={st.v.targetMB}
                    oninput={(e) => {
                      const v = parseFloat((e.currentTarget as HTMLInputElement).value.replace(',', '.'))
                      st.v.targetMB = isNaN(v) || v <= 0 ? 0.1 : Math.min(100000, v)
                    }}
                  /><span>MB</span>
                </div>
                {#each [8, 16, 25, 50] as mb}
                  <button class="mchip" class:on={st.v.targetMB === mb} onclick={() => (st.v.targetMB = mb)}>{mb}</button>
                {/each}
              </div>
              <p class="hint">{L('Bitrate dihitung dari durasi; encode 2 tahap agar ukurannya pas (WhatsApp: 16 MB, Discord: 10 MB).', 'The bitrate is worked out from the duration; two passes keep the size on target (WhatsApp: 16 MB, Discord: 10 MB).')}</p>
            {:else}
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
              {/if}
            {/if}
            {#if st.v.manual || st.v.targetMB > 0 || st.v.bitrateK > 0}
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
            {#if st.v.targetMB === 0 && st.v.bitrateK === 0}
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
            {/if}
          </div>
        {/if}

        {#if !isCopy}
          <div class="sec">
            <span class="label">{L('Resolusi', 'Resolution')}</span>
            <Chips
              bind:value={st.v.resolution}
              columns={4}
              small
              options={[
                { value: 'original', label: L('Asli', 'Original') }, { value: '2160', label: '4K' }, { value: '1440', label: '1440p' }, { value: '1080', label: '1080p' },
                { value: '720', label: '720p' }, { value: '480', label: '480p' }, { value: 'custom', label: L('Lain', 'Other') },
              ]}
            />
            {#if st.v.resolution === 'custom'}
              <div class="num wide">
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
          </div>

          <div class="sec">
            <span class="label">{L('Frame per detik', 'Frames per second')}</span>
            <Chips
              bind:value={st.v.fps}
              columns={5}
              small
              options={[
                { value: 'original', label: isGif ? '12' : L('Asli', 'Original') }, { value: '60', label: '60' }, { value: '30', label: '30' },
                { value: '24', label: '24' }, { value: 'custom', label: L('Lain', 'Other') },
              ]}
            />
            {#if st.v.fps === 'custom'}
              <div class="num wide">
                <input
                  class="text-input"
                  aria-label="FPS"
                  inputmode="decimal"
                  value={st.v.fpsCustom || ''}
                  oninput={(e) => {
                    const v = parseFloat((e.currentTarget as HTMLInputElement).value.replace(',', '.'))
                    st.v.fpsCustom = isNaN(v) ? 0 : Math.min(240, v)
                  }}
                /><span>fps</span>
              </div>
            {/if}
          </div>

          <div class="sec">
            <span class="label">{L('Putar & balik', 'Rotate & flip')}</span>
            <div class="tools">
              <button class="tbtn" title={L('Putar ke kiri', 'Rotate left')} aria-label={L('Putar ke kiri', 'Rotate left')} onclick={() => (st.v.rotate = (st.v.rotate + 270) % 360)}><Icon name="rotateCcw" size={16} /></button>
              <button class="tbtn" title={L('Putar ke kanan', 'Rotate right')} aria-label={L('Putar ke kanan', 'Rotate right')} onclick={() => (st.v.rotate = (st.v.rotate + 90) % 360)}><Icon name="rotateCw" size={16} /></button>
              <button class="tbtn" class:on={st.v.flipH} aria-pressed={st.v.flipH} title={L('Balik horizontal (cermin)', 'Flip horizontally (mirror)')} aria-label={L('Balik horizontal', 'Flip horizontally')} onclick={() => (st.v.flipH = !st.v.flipH)}><Icon name="flipH" size={16} /></button>
              <button class="tbtn" class:on={st.v.flipV} aria-pressed={st.v.flipV} title={L('Balik vertikal', 'Flip vertically')} aria-label={L('Balik vertikal', 'Flip vertically')} onclick={() => (st.v.flipV = !st.v.flipV)}><Icon name="flipV" size={16} /></button>
              <span class="rot">{st.v.rotate}°</span>
              {#if st.v.rotate || st.v.flipH || st.v.flipV}<button class="link" onclick={() => { st.v.rotate = 0; st.v.flipH = false; st.v.flipV = false }}>Reset</button>{/if}
            </div>
          </div>
        {/if}

        {#if !isGif}
          <div class="sec">
            <span class="label">{L('Suara', 'Audio track')}</span>
            <div class="inline">
              <Select
                label={L('Suara', 'Audio track')}
                bind:value={st.v.audioMode}
                options={[
                  { value: 'auto', label: L('Otomatis', 'Automatic') },
                  { value: 'copy', label: L('Salin asli', 'Copy original') },
                  { value: 'aac', label: 'AAC', disabled: st.v.format === 'webm' || st.v.format === 'avi' },
                  { value: 'mp3', label: 'MP3', disabled: st.v.format === 'webm' },
                  { value: 'opus', label: 'Opus', disabled: st.v.format === 'mov' || st.v.format === 'avi' },
                  { value: 'mute', label: L('Tanpa suara', 'No sound') },
                ]}
              />
              {#if ['aac', 'mp3', 'opus'].includes(st.v.audioMode)}
                <div class="br">
                  <Select
                    label="Bitrate"
                    bind:value={st.v.audioBitrate}
                    options={[64, 96, 128, 160, 192, 256, 320].map((b) => ({ value: b, label: `${b} kbps` }))}
                  />
                </div>
              {/if}
            </div>
          </div>

          {#if st.mode === 'video'}
            <div class="sec">
              <span class="label">Subtitle</span>
              <Segmented
                label="Subtitle"
                bind:value={st.v.subtitles}
                options={[
                  { value: 'none', label: L('Tanpa', 'None') },
                  { value: 'embed', label: L('Sematkan', 'Embed'), disabled: st.v.format === 'avi' },
                  { value: 'burn', label: L('Bakar', 'Burn in'), disabled: isCopy },
                ]}
              />
              {#if st.v.subtitles !== 'none'}
                <p class="hint">
                  {L('Dipakai file .srt/.ass/.vtt bernama sama di samping video (mis. film.srt, film.id.srt), atau subtitle di dalam file.', 'Uses a .srt/.ass/.vtt file with the same name next to the video (e.g. movie.srt, movie.en.srt), or subtitles inside the file.')}
                  {st.v.subtitles === 'burn' ? L(' Dibakar = selalu tampil, tidak bisa dimatikan.', ' Burned in = always shown, can\'t be turned off.') : L(' Disematkan = bisa dinyalakan/dimatikan di pemutar.', ' Embedded = can be switched on/off in the player.')}
                </p>
              {/if}
            </div>
          {/if}
        {/if}
      {:else if st.mode === 'audio'}
        <AudioFormatPanel bind:o={st.a} />
      {:else}
        <div class="sec">
          <span class="label">{L('Ambil gambar setiap', 'Take a picture every')}</span>
          <Chips
            bind:value={st.frameEvery}
            columns={5}
            small
            options={[1, 5, 10, 30, 60].map((n) => ({ value: n, label: n === 60 ? L('1 mnt', '1 min') : `${n} ${L('dtk', 's')}` }))}
          />
          <span class="label">{L('Format gambar', 'Picture format')}</span>
          <Segmented label={L('Format gambar', 'Picture format')} bind:value={st.frameFormat} options={[{ value: 'jpg', label: 'JPG' }, { value: 'png', label: 'PNG' }]} />
        </div>
        <div class="sec">
          <span class="label">{L('Resolusi', 'Resolution')}</span>
          <Chips
            bind:value={st.v.resolution}
            columns={4}
            small
            options={[
              { value: 'original', label: L('Asli', 'Original') }, { value: '1080', label: '1080p' }, { value: '720', label: '720p' }, { value: '480', label: '480p' },
            ]}
          />
        </div>
      {/if}

      {#if st.mode !== 'merge'}
        <div class="sec">
          <TrimFields bind:start={st.v.trimStart} bind:end={st.v.trimEnd} label={L('Potong (opsional)', 'Trim (optional)')} />
          {#if isCopy && st.mode === 'video' && st.v.trimStart}<p class="hint">{L('Mode salin memotong di keyframe terdekat, jadi awalnya bisa bergeser sedikit.', 'Copy mode cuts at the nearest keyframe, so the start may shift slightly.')}</p>{/if}
        </div>
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
      {startLabel}
      disabled={(st.mode === 'merge' ? mergeFew : pending.length === 0) || invalidCustom || noFFmpeg || trimBad}
      disabledHint={noFFmpeg
        ? L('Pasang FFmpeg dulu (lihat banner di atas).', 'Install FFmpeg first (see the banner above).')
        : conv.items.length === 0
          ? L('Tambahkan video dulu untuk memulai.', 'Add videos to get started.')
          : mergeFew
            ? L('Tambahkan minimal 2 video untuk digabung.', 'Add at least 2 videos to join.')
            : invalidCustom
              ? L('Isi resolusi yang valid (minimal 16 px).', 'Enter a valid resolution (at least 16 px).')
              : trimBad
                ? L('Perbaiki waktu potong.', 'Fix the trim times.')
                : ''}
      note={conv.items.length && !conv.running && totalDuration ? `${L('Total durasi', 'Total duration')} ${duration(totalDuration)}` : ''}
      lastOutput={conv.lastOutput()}
      queueMore={conv.running && st.mode !== 'merge' ? conv.fresh().filter((it) => !it.error).length : 0}
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
  .link:hover:not(:disabled) {
    color: var(--accent-text);
  }
  .inline {
    display: flex;
    gap: 6px;
    align-items: center;
  }
  .num {
    position: relative;
    width: 110px;
    flex-shrink: 0;
  }
  .num.wide {
    width: auto;
  }
  .num.kbps {
    width: 120px;
  }
  .num.kbps input {
    padding-right: 44px;
  }
  .num input {
    padding-right: 40px;
    font-weight: 700;
  }
  .num.wide input {
    padding-right: 110px;
  }
  .num span {
    position: absolute;
    right: 12px;
    top: 12px;
    font-size: 12px;
    color: var(--text-3);
    pointer-events: none;
  }
  .mchip {
    height: 40px;
    flex: 1;
    border-radius: 9px;
    border: 1px solid var(--border);
    background: var(--surface-2);
    color: var(--text-4);
    font-size: 12px;
    font-weight: 600;
  }
  .mchip.on {
    border-color: var(--accent);
    background: var(--accent-soft);
    color: var(--accent-text);
    font-weight: 700;
  }
  .br {
    width: 120px;
    flex-shrink: 0;
  }
  .tools {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .tbtn {
    width: 36px;
    height: 34px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 9px;
    border: 1px solid var(--border);
    background: var(--surface-2);
    color: var(--text-2);
  }
  .tbtn.on {
    border-color: var(--accent);
    background: var(--accent-soft);
    color: var(--accent-text);
  }
  .rot {
    font-size: 13px;
    font-weight: 700;
    color: var(--text-2);
    margin-left: 4px;
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
