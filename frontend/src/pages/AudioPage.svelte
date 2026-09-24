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
  import Icon from '../components/Icon.svelte'
  import { api } from '../lib/api'
  import { hasTool } from '../lib/stores/app.svelte'
  import { audioConv as conv } from '../lib/stores/converter.svelte'
  import { load, save } from '../lib/stores/persist'
  import { bytes, codec, duration, khz, tile } from '../lib/format'
  import { lossyAudio } from '../lib/media'
  import type { AudioOptions, FileItem } from '../lib/types'

  const defaults: { a: AudioOptions } = {
    a: { format: 'mp3', bitrate: 320, channels: 'source', sampleRate: 'source', keepMetadata: true },
  }
  const st = $state(load('kmb.audio', defaults))
  $effect(() => save('kmb.audio', $state.snapshot(st)))

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

  const pending = $derived(conv.pending())
  const noFFmpeg = $derived(!hasTool('ffmpeg'))

  function start() {
    conv.start(pending, (items) => api.startAudio(items, $state.snapshot(st.a)))
  }
</script>

<PageHeader title={L('Konversi Audio', 'Audio Converter')} subtitle={L('Ubah format dan kualitas audio, termasuk audio dari file video.', 'Change audio format and quality, including audio from video files.')} />
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
      resultWidth={150}
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
        <span class="r1">{st.a.format.toUpperCase()} · {lossyAudio[st.a.format] ? `${st.a.bitrate} kbps` : 'lossless'}</span>
        {#if s === 'done' && task}
          <span class="r2">{bytes(task.outSize)}</span>
        {:else if s === 'running' && task?.message}
          <span class="r2">{task.message}</span>
        {/if}
      {/snippet}
    </FileList>
  {/if}

  <aside class="card panel" aria-label={L('Pengaturan output', 'Output settings')}>
    <div class="scroll">
      <AudioFormatPanel bind:o={st.a} />

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

      <Switch bind:checked={st.a.keepMetadata} label={L('Pertahankan info lagu', 'Keep song info')} hint={L('Judul, artis, album & cover', 'Title, artist, album & cover')} />

      <div class="sec">
        <span class="label">{L('Simpan ke', 'Save to')}</span>
        <OutputPicker kind="audio" />
      </div>
    </div>

    <RunFooter
      kind="audio"
      running={conv.running}
      busy={conv.starting}
      startLabel={conv.hasUnprocessed() || pending.length === 0 ? `${L('Mulai konversi', 'Start converting')}${pending.length ? ` (${pending.length})` : ''}` : `${L('Konversi ulang', 'Convert again')} (${pending.length})`}
      disabled={pending.length === 0 || noFFmpeg}
      disabledHint={noFFmpeg ? L('Pasang FFmpeg dulu (lihat banner di atas).', 'Install FFmpeg first (see the banner above).') : conv.items.length === 0 ? L('Tambahkan audio dulu untuk memulai.', 'Add audio to get started.') : ''}
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
</style>
