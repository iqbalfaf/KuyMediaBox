<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import PageHeader from '../components/PageHeader.svelte'
  import FileList from '../components/FileList.svelte'
  import EmptyDrop from '../components/EmptyDrop.svelte'
  import Segmented from '../components/Segmented.svelte'
  import Select from '../components/Select.svelte'
  import Switch from '../components/Switch.svelte'
  import Chips from '../components/Chips.svelte'
  import OutputPicker from '../components/OutputPicker.svelte'
  import RunFooter from '../components/RunFooter.svelte'
  import ToolBanner from '../components/ToolBanner.svelte'
  import ModelPicker from '../components/ModelPicker.svelte'
  import PresetBar from '../components/PresetBar.svelte'
  import Icon from '../components/Icon.svelte'
  import { api } from '../lib/api'
  import { hasTool } from '../lib/stores/app.svelte'
  import { subtitleConv as conv } from '../lib/stores/converter.svelte'
  import { modelReady } from '../lib/stores/models.svelte'
  import { load, save } from '../lib/stores/persist'
  import { subtitleDefaults, subtitleBuiltins } from '../lib/modules'
  import { bytes, duration, tile } from '../lib/format'
  import type { FileItem, SubtitleJob } from '../lib/types'

  const defaults = subtitleDefaults
  const st = $state(load('kmb.subtitle', defaults))
  $effect(() => save('kmb.subtitle', $state.snapshot(st)))

  const builtins = $derived(subtitleBuiltins())

  const models: Record<string, [string, string]> = $derived({
    tiny: [L('Sangat cepat', 'Very fast'), L('akurasi rendah', 'low accuracy')],
    base: [L('Cepat', 'Fast'), L('cukup untuk suara jelas', 'fine for clear speech')],
    small: [L('Seimbang', 'Balanced'), L('disarankan untuk bahasa Indonesia', 'recommended for Indonesian')],
    turbo: [L('Terbaik (turbo)', 'Best (turbo)'), L('paling akurat, perlu PC kencang', 'most accurate, needs a fast PC')],
    medium: [L('Akurat', 'Accurate'), L('lambat', 'slow')],
  })

  const langName = $derived<Record<string, string>>({
    auto: L('Deteksi otomatis', 'Detect automatically'), id: 'Bahasa Indonesia', en: 'English', ms: 'Bahasa Melayu', jv: 'Basa Jawa', su: 'Basa Sunda',
    ar: 'العربية (Arab)', zh: '中文 (Mandarin)', ja: '日本語 (Jepang)', ko: '한국어 (Korea)', hi: 'हिन्दी (Hindi)', th: 'ไทย (Thai)', vi: 'Tiếng Việt',
    tl: 'Tagalog', es: 'Español', pt: 'Português', fr: 'Français', de: 'Deutsch', it: 'Italiano', nl: 'Nederlands', ru: 'Русский', tr: 'Türkçe',
  })

  function toggleFormat(f: string) {
    const set = new Set(st.job.formats)
    if (set.has(f)) set.delete(f)
    else set.add(f)
    if (set.size === 0) set.add('srt')
    st.job.formats = ['srt', 'vtt', 'txt', 'lrc'].filter((x) => set.has(x))
  }

  function meta(it: FileItem): string {
    const p: string[] = [it.hasVideo ? 'Video' : 'Audio', it.ext.toUpperCase()]
    if (it.duration) p.push(duration(it.duration))
    p.push(bytes(it.size))
    if (it.subFile || it.subCodec) p.push(L('sudah ada subtitle', 'has subtitles'))
    return p.join(' · ')
  }

  function outLabel(it: FileItem): string {
    const f = st.job.formats.map((x) => x.toUpperCase()).join(' + ')
    if (st.job.video !== 'none' && it.hasVideo) return `${f} + ${st.job.video === 'burn' ? L('video (dibakar)', 'video (burned)') : L('video (disematkan)', 'video (embedded)')}`
    return f
  }

  const pending = $derived(conv.pending())
  const noTools = $derived(!hasTool('ffmpeg') || !hasTool('whisper'))
  const noModel = $derived(!modelReady('whisper', st.job.model))
  const totalDuration = $derived(conv.items.reduce((s, it) => s + (it.duration || 0), 0))

  function start() {
    conv.start(pending, (items) => api.startSubtitle(items, $state.snapshot(st.job)))
  }
</script>

<PageHeader title={L('Subtitle Otomatis', 'Automatic Subtitles')} subtitle={L('Ucapan di video atau audio jadi subtitle SRT/VTT atau teks, offline di PC ini.', 'Speech in videos or audio becomes SRT/VTT subtitles or text, offline on this PC.')} />
<ToolBanner ids={['ffmpeg', 'whisper']} why={L('FFmpeg membaca suaranya, Whisper mengenali ucapan (±9 MB, lalu pilih model di panel kanan).', 'FFmpeg reads the sound, Whisper recognises the speech (±9 MB, then pick a model on the right).')} />

<div class="body">
  {#if conv.items.length === 0}
    <EmptyDrop
      {conv}
      title={L('Tarik & lepas video atau audio ke sini', 'Drag & drop videos or audio here')}
      subtitle={L('Film, vlog, rekaman kuliah, rapat, podcast — apa saja yang ada suaranya.', 'Films, vlogs, lectures, meetings, podcasts — anything with speech.')}
      pickLabel={L('Pilih file', 'Choose files')}
      formats={['MP4', 'MKV', 'MOV', 'WEBM', 'MP3', 'M4A', 'WAV', 'FLAC']}
      steps={[
        [L('Tambahkan file', 'Add files'), L('Tarik ke sini atau klik tombol', 'Drag them here or click the button')],
        [L('Pilih bahasa & model', 'Pick language & model'), L('Unduh model sekali saja', 'Download a model once')],
        [L('Klik Mulai', 'Click Start'), L('Subtitle disimpan di samping file', 'Subtitles are saved next to the file')],
      ]}
    />
  {:else}
    <FileList {conv} noun={L('file', 'files')} dropText={L('Tarik & lepas video atau audio di sini', 'Drag & drop videos or audio here')} formats="MP4 · MKV · MOV · MP3 · M4A · WAV" resultWidth={190} {meta}>
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
          <span class="r2 ellipsis" title={task.output}>{task.output.split(/[\\/]/).pop()}</span>
        {:else if s === 'running' && task?.message}
          <span class="r2">{task.message}</span>
        {:else}
          <span class="r2">{langName[st.job.language] ?? st.job.language}{st.job.translate ? ` → English` : ''}</span>
        {/if}
      {/snippet}
    </FileList>
  {/if}

  <aside class="card panel" aria-label={L('Pengaturan subtitle', 'Subtitle settings')}>
    <div class="scroll">
      <PresetBar module="subtitle" target={st} {builtins} />

      <div class="sec">
        <span class="label">{L('Bahasa yang diucapkan', 'Spoken language')}</span>
        <Select label={L('Bahasa', 'Language')} bind:value={st.job.language} options={Object.keys(langName).map((k) => ({ value: k, label: langName[k] }))} />
        <Switch bind:checked={st.job.translate} label={L('Terjemahkan ke bahasa Inggris', 'Translate into English')} hint={L('Subtitle ditulis dalam bahasa Inggris, apa pun bahasa aslinya', 'Subtitles are written in English, whatever the original language')} />
      </div>

      <div class="sec">
        <span class="label">{L('Model', 'Model')}</span>
        <ModelPicker kind="whisper" bind:value={st.job.model} labels={models} />
        <p class="hint">{L('Model lebih besar = lebih akurat tapi lebih lambat. Semua berjalan offline; model hanya diunduh sekali.', 'Bigger models = more accurate but slower. Everything runs offline; a model is downloaded once.')}</p>
      </div>

      <div class="sec">
        <span class="label">{L('Format hasil', 'Output formats')}</span>
        <div class="multi">
          {#each [['srt', 'SRT'], ['vtt', 'VTT'], ['txt', L('Teks', 'Text')], ['lrc', 'LRC']] as [f, name]}
            <button class="mchip" class:on={st.job.formats.includes(f)} aria-pressed={st.job.formats.includes(f)} onclick={() => toggleFormat(f)}>{name}</button>
          {/each}
        </div>
        <p class="hint">{L('SRT: paling umum (pemutar video, YouTube). VTT: web. Teks: transkrip polos. LRC: lirik lagu.', 'SRT: most common (players, YouTube). VTT: web. Text: plain transcript. LRC: song lyrics.')}</p>
      </div>

      <div class="sec">
        <span class="label">{L('Panjang baris', 'Line length')}</span>
        <Chips
          bind:value={st.job.maxLen}
          columns={4}
          small
          options={[
            { value: 0, label: L('Asli', 'As is') },
            { value: 32, label: '32' },
            { value: 42, label: '42' },
            { value: 60, label: '60' },
          ]}
        />
        <p class="hint">{L('Huruf per baris, maksimal 2 baris per subtitle — kalimat panjang dipecah dengan waktu yang pas. 42 adalah standar TV; "Asli" memakai potongan dari Whisper apa adanya.', 'Characters per line, at most 2 lines per subtitle — long sentences are split with matching timing. 42 is the TV standard; "As is" keeps Whisper’s segments unchanged.')}</p>
      </div>

      <div class="sec">
        <span class="label">{L('Video bersubtitle', 'Subtitled video')}</span>
        <Segmented
          label={L('Video bersubtitle', 'Subtitled video')}
          bind:value={st.job.video}
          options={[
            { value: 'none', label: L('Tidak', 'No') },
            { value: 'embed', label: L('Sematkan', 'Embed') },
            { value: 'burn', label: L('Bakar', 'Burn in') },
          ]}
        />
        <p class="hint">
          {st.job.video === 'none'
            ? L('Hanya file subtitle yang dibuat. Beri nama sama dengan video agar pemutar memuatnya otomatis.', 'Only subtitle files are made, named like the video so players load them automatically.')
            : st.job.video === 'embed'
              ? L('Juga menyimpan salinan video berisi subtitle yang bisa dinyalakan/dimatikan (tanpa encode ulang, cepat).', 'Also saves a copy of the video with switchable subtitles (no re-encode, fast).')
              : L('Juga menyimpan video dengan subtitle yang selalu tampil — cocok untuk WhatsApp/Instagram (encode ulang, lebih lama).', 'Also saves a video with always-visible subtitles — good for WhatsApp/Instagram (re-encoded, slower).')}
        </p>
      </div>

      <div class="sec">
        <span class="label">{L('Simpan ke', 'Save to')}</span>
        <OutputPicker kind="subtitle" />
      </div>
    </div>

    <RunFooter
      kind="subtitle"
      running={conv.running}
      busy={conv.starting}
      startLabel={`${L('Buat subtitle', 'Make subtitles')}${pending.length ? ` (${pending.length})` : ''}`}
      disabled={pending.length === 0 || noTools || noModel}
      disabledHint={noTools
        ? L('Pasang FFmpeg dan Whisper dulu (lihat banner di atas).', 'Install FFmpeg and Whisper first (see the banner above).')
        : conv.items.length === 0
          ? L('Tambahkan file dulu untuk memulai.', 'Add files to get started.')
          : noModel
            ? L('Unduh model yang dipilih dulu (tombol Unduh di panel).', 'Download the chosen model first (Get button in the panel).')
            : ''}
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
  .multi {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 6px;
  }
  .mchip {
    height: 34px;
    padding: 0 8px;
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
</style>
