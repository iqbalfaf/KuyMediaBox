<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import Chips from './Chips.svelte'
  import Segmented from './Segmented.svelte'
  import { lossyAudio } from '../lib/media'
  import type { AudioOptions } from '../lib/types'

  let { o = $bindable() }: { o: AudioOptions } = $props()

  const bitrateHint = $derived<Record<number, string>>({
    128: L('128 kbps · hemat, cukup untuk podcast & suara', '128 kbps · small, fine for podcasts & voice'),
    192: L('192 kbps · seimbang untuk musik', '192 kbps · balanced for music'),
    256: L('256 kbps · kualitas tinggi', '256 kbps · high quality'),
    320: L('320 kbps · kualitas terbaik untuk MP3', '320 kbps · best quality for MP3'),
  })
  const lossy = $derived(lossyAudio[o.format] ?? true)
  const bitrates = $derived(
    o.format === 'opus' ? [96, 128, 192, 256] : [128, 192, 256, 320],
  )
  $effect(() => {
    if (lossy && !bitrates.includes(o.bitrate)) o.bitrate = bitrates[Math.min(1, bitrates.length - 1)]
  })
</script>

<div class="sec">
  <span class="label">{L('Format hasil', 'Output format')}</span>
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
    <p class="hint">{bitrateHint[o.bitrate] ?? `${o.bitrate} kbps`}{o.format === 'opus' ? L(' (Opus sudah jernih di bitrate rendah)', ' (Opus sounds clear even at low bitrates)') : ''}</p>
  </div>
{:else}
  <p class="hint">{o.format.toUpperCase()} {L('tanpa kehilangan kualitas (lossless) — ukuran file lebih besar.', 'keeps full quality (lossless) — larger files.')}</p>
{/if}

<style>
  .sec {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
</style>
