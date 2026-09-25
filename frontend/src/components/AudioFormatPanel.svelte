<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import Chips from './Chips.svelte'
  import Segmented from './Segmented.svelte'
  import { lossyAudio } from '../lib/media'
  import type { AudioOptions } from '../lib/types'

  let { o = $bindable(), allowOriginal = false }: { o: AudioOptions; allowOriginal?: boolean } = $props()

  const bitrateHint = $derived<Record<number, string>>({
    64: L('64 kbps · sangat hemat, untuk rekaman suara (paling bagus dengan Opus)', '64 kbps · very small, for voice recordings (best with Opus)'),
    96: L('96 kbps · hemat, jernih untuk Opus', '96 kbps · small, clear with Opus'),
    128: L('128 kbps · hemat, cukup untuk podcast & suara', '128 kbps · small, fine for podcasts & voice'),
    192: L('192 kbps · seimbang untuk musik', '192 kbps · balanced for music'),
    256: L('256 kbps · kualitas tinggi', '256 kbps · high quality'),
    320: L('320 kbps · kualitas terbaik untuk MP3', '320 kbps · best quality for MP3'),
  })
  const lossy = $derived(o.format !== 'original' && (lossyAudio[o.format] ?? true))
  const bitrates = $derived(o.format === 'opus' ? [64, 96, 128, 192, 256] : [64, 128, 192, 256, 320])
  $effect(() => {
    if (lossy && !bitrates.includes(o.bitrate)) o.bitrate = bitrates[2]
  })
  // Variable bitrate: MP3 (LAME -V), OGG (Vorbis -q) and Opus. AAC stays constant.
  const canVBR = $derived(o.format === 'mp3' || o.format === 'ogg' || o.format === 'opus')
  const vbrAvg: Record<string, Record<string, number>> = {
    mp3: { best: 245, high: 190, medium: 165, small: 130 },
    ogg: { best: 256, high: 192, medium: 128, small: 96 },
    opus: { best: 192, high: 160, medium: 128, small: 96 },
  }
  const vbrName = $derived<Record<string, string>>({ best: L('Terbaik', 'Best'), high: L('Tinggi', 'High'), medium: L('Sedang', 'Medium'), small: L('Hemat', 'Small') })
</script>

<div class="sec">
  <span class="label">{L('Format hasil', 'Output format')}</span>
  <Chips
    bind:value={o.format}
    columns={3}
    options={[
      { value: 'mp3', label: 'MP3' }, { value: 'm4a', label: 'M4A' }, { value: 'flac', label: 'FLAC' },
      { value: 'wav', label: 'WAV' }, { value: 'ogg', label: 'OGG' }, { value: 'opus', label: 'OPUS' },
      ...(allowOriginal ? [{ value: 'original', label: L('Asli (salin)', 'Original (copy)') }] : []),
    ]}
  />
</div>

{#if lossy}
  <div class="sec">
    <span class="label">Bitrate</span>
    {#if canVBR}
      <Segmented
        label={L('Jenis bitrate', 'Bitrate type')}
        value={o.vbr ? 'vbr' : 'cbr'}
        onchange={(v) => (o.vbr = v === 'vbr')}
        options={[
          { value: 'cbr', label: L('Tetap (CBR)', 'Constant (CBR)') },
          { value: 'vbr', label: L('Variabel (VBR)', 'Variable (VBR)') },
        ]}
      />
    {/if}
    {#if o.vbr && canVBR}
      <Chips
        bind:value={o.vbrLevel}
        columns={4}
        tall
        options={(['best', 'high', 'medium', 'small'] as const).map((lv) => ({ value: lv, label: vbrName[lv], sub: `±${vbrAvg[o.format][lv]}` }))}
      />
      <p class="hint">{L('VBR memberi bit lebih banyak di bagian yang rumit dan lebih sedikit di bagian sederhana — kualitas sama, file biasanya lebih kecil. Angka = rata-rata kbps.', 'VBR spends more bits on complex parts and fewer on simple ones — same quality, usually smaller files. Numbers are average kbps.')}</p>
    {:else}
      <Chips bind:value={o.bitrate} columns={5} small options={bitrates.map((b) => ({ value: b, label: String(b) }))} />
      <p class="hint">{bitrateHint[o.bitrate] ?? `${o.bitrate} kbps`}{o.format === 'opus' ? L(' (Opus sudah jernih di bitrate rendah)', ' (Opus sounds clear even at low bitrates)') : ''}{o.format === 'm4a' ? L(' · M4A (AAC) selalu bitrate tetap.', ' · M4A (AAC) always uses a constant bitrate.') : ''}</p>
    {/if}
  </div>
{:else if o.format === 'original'}
  <p class="hint">{L('Audio disalin apa adanya tanpa encode ulang — cocok untuk mengubah tag atau memotong. Efek suara tidak bisa dipakai.', 'The audio is copied as is without re-encoding — good for editing tags or trimming. Sound effects are unavailable.')}</p>
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
