<script lang="ts">
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
      p.push('Audio dari video')
      if (it.audioCodec) p.push(codec(it.audioCodec))
    } else {
      p.push(it.ext.toUpperCase())
      if (it.sampleRate) p.push(khz(it.sampleRate))
      if (it.bitsPerSample && !lossyAudio[it.ext]) p.push(`${it.bitsPerSample}-bit`)
    }
    if (it.duration) p.push(duration(it.duration))
    p.push(bytes(it.size))
    if (it.hasCover) p.push('ada cover')
    return p.join(' · ')
  }

  const pending = $derived(conv.pending())
  const noFFmpeg = $derived(!hasTool('ffmpeg'))

  function start() {
    conv.start(pending, (items) => api.startAudio(items, $state.snapshot(st.a)))
  }
</script>

<PageHeader title="Konversi Audio" subtitle="Ubah format dan kualitas audio, termasuk audio dari file video." />
<ToolBanner ids={['ffmpeg']} why="FFmpeg dibutuhkan untuk membaca dan mengonversi audio." />

<div class="body">
  {#if conv.items.length === 0}
    <EmptyDrop
      {conv}
      title="Tarik & lepas audio ke sini"
      subtitle="File audio atau video — audionya yang diambil."
      pickLabel="Pilih audio"
      formats={['MP3', 'WAV', 'FLAC', 'M4A', 'OGG', 'OPUS', 'WMA', 'AAC', 'MP4']}
      steps={[
        ['Tambahkan audio', 'Tarik ke sini atau klik tombol'],
        ['Pilih format & bitrate', 'Di panel sebelah kanan'],
        ['Klik Mulai', 'File asli tidak akan diubah'],
      ]}
    />
  {:else}
    <FileList
      {conv}
      noun="file"
      dropText="Tarik & lepas audio atau video di sini"
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

  <aside class="card panel" aria-label="Pengaturan output">
    <div class="scroll">
      <AudioFormatPanel bind:o={st.a} />

      <div class="sec">
        <span class="label">Channel</span>
        <Segmented
          label="Channel"
          bind:value={st.a.channels}
          options={[
            { value: 'source', label: 'Ikuti asli' },
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
            { value: 'source', label: 'Ikuti file asli' },
            { value: '44100', label: '44,1 kHz (CD)' },
            { value: '48000', label: '48 kHz (video)' },
          ]}
        />
        {#if st.a.format === 'opus' && st.a.sampleRate === '44100'}<p class="hint">Opus selalu memakai 48 kHz.</p>{/if}
      </div>

      <Switch bind:checked={st.a.keepMetadata} label="Pertahankan info lagu" hint="Judul, artis, album & cover" />

      <div class="sec">
        <span class="label">Simpan ke</span>
        <OutputPicker kind="audio" />
      </div>
    </div>

    <RunFooter
      kind="audio"
      running={conv.running}
      busy={conv.starting}
      startLabel={conv.hasUnprocessed() || pending.length === 0 ? `Mulai konversi${pending.length ? ` (${pending.length})` : ''}` : `Konversi ulang (${pending.length})`}
      disabled={pending.length === 0 || noFFmpeg}
      disabledHint={noFFmpeg ? 'Pasang FFmpeg dulu (lihat banner di atas).' : conv.items.length === 0 ? 'Tambahkan audio dulu untuk memulai.' : ''}
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
