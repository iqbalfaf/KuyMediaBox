<script lang="ts">
  import Chips from './Chips.svelte'
  import Segmented from './Segmented.svelte'
  import { lossyAudio } from '../lib/media'
  import type { AudioOptions } from '../lib/types'

  let { o = $bindable() }: { o: AudioOptions } = $props()

  const bitrateHint: Record<number, string> = {
    128: '128 kbps · hemat, cukup untuk podcast & suara',
    192: '192 kbps · seimbang untuk musik',
    256: '256 kbps · kualitas tinggi',
    320: '320 kbps · kualitas terbaik untuk MP3',
  }
  const lossy = $derived(lossyAudio[o.format] ?? true)
  const bitrates = $derived(
    o.format === 'opus' ? [96, 128, 192, 256] : [128, 192, 256, 320],
  )
  $effect(() => {
    if (lossy && !bitrates.includes(o.bitrate)) o.bitrate = bitrates[Math.min(1, bitrates.length - 1)]
  })
</script>

<div class="sec">
  <span class="label">Format hasil</span>
  <Chips
    bind:value={o.format}
    columns={3}
    options={[
      { value: 'mp3', label: 'MP3' }, { value: 'm4a', label: 'M4A' }, { value: 'flac', label: 'FLAC' },
      { value: 'wav', label: 'WAV' }, { value: 'ogg', label: 'OGG' }, { value: 'opus', label: 'OPUS' },
    ]}
  />
</div>

{#if lossy}
  <div class="sec">
    <span class="label">Bitrate</span>
    <Segmented label="Bitrate" bind:value={o.bitrate} options={bitrates.map((b) => ({ value: b, label: String(b) }))} />
    <p class="hint">{bitrateHint[o.bitrate] ?? `${o.bitrate} kbps`}{o.format === 'opus' ? ' (Opus sudah jernih di bitrate rendah)' : ''}</p>
  </div>
{:else}
  <p class="hint">{o.format.toUpperCase()} tanpa kehilangan kualitas (lossless) — ukuran file lebih besar.</p>
{/if}

<style>
  .sec {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
</style>
