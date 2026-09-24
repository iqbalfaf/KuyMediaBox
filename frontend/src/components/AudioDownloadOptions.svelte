<script lang="ts">
  import { L, locale } from '../lib/i18n.svelte'
  import Chips from './Chips.svelte'
  import type { DownloadOptions } from '../lib/types'

  let {
    opts,
    label,
    spotify = false,
    onchange,
  }: { opts: DownloadOptions; label: string; spotify?: boolean; onchange: () => void } = $props()

  type Rate = DownloadOptions['audioQuality']

  // Bitrates offered per format with a recommendation; the middle one is the sweet spot.
  const rates: Record<string, { value: Rate; tag?: 'save' | 'rec' | 'best' }[]> = {
    mp3: [{ value: '128', tag: 'save' }, { value: '192', tag: 'rec' }, { value: '256' }, { value: '320', tag: 'best' }],
    m4a: [{ value: '128', tag: 'save' }, { value: '192', tag: 'rec' }, { value: '256', tag: 'best' }],
    opus: [{ value: '96', tag: 'save' }, { value: '128', tag: 'rec' }, { value: '160', tag: 'best' }],
  }
  const recommended: Record<string, Rate> = { mp3: '192', m4a: '192', opus: '128' }

  const lossless = $derived(opts.audioFormat === 'flac' || opts.audioFormat === 'wav')
  const list = $derived(rates[opts.audioFormat] ?? [])

  const tagText = (tag?: string) =>
    tag === 'rec' ? L('Disarankan', 'Recommended') : tag === 'best' ? L('Terbaik', 'Best') : tag === 'save' ? L('Hemat', 'Smaller') : ''

  function pickFormat(f: DownloadOptions['audioFormat']) {
    // Keep a bitrate that exists for the new format, else use its recommendation.
    if (f === 'flac' || f === 'wav') opts.audioQuality = 'auto'
    else if (opts.audioQuality !== 'auto' && !rates[f].some((r) => r.value === opts.audioQuality)) opts.audioQuality = recommended[f]
    onchange()
  }

  const mbPerMin = $derived.by(() => {
    const nf = new Intl.NumberFormat(locale(), { maximumFractionDigits: 1 })
    if (opts.audioFormat === 'wav') return nf.format(10.1)
    if (opts.audioFormat === 'flac') return `${nf.format(4)}–${nf.format(6)}`
    if (opts.audioQuality === 'auto') return ''
    return nf.format((Number(opts.audioQuality) * 60) / 8 / 1024)
  })

  const formats = $derived([
    { value: 'mp3' as const, label: 'MP3', sub: L('semua perangkat', 'plays anywhere') },
    { value: 'm4a' as const, label: 'M4A', sub: 'AAC' },
    { value: 'opus' as const, label: 'OPUS', sub: L('paling kecil', 'smallest') },
    ...(spotify ? [] : [{ value: 'flac' as const, label: 'FLAC', sub: 'lossless' }]),
    { value: 'wav' as const, label: 'WAV', sub: L('untuk editing', 'for editing') },
  ])
</script>

<div class="sec">
  <span class="label">{label}</span>
  <Chips bind:value={opts.audioFormat} columns={spotify ? 4 : 3} tall options={formats} onchange={pickFormat} />
</div>

{#if lossless}
  <p class="hint">
    {opts.audioFormat === 'wav'
      ? L('WAV tanpa kompresi, cocok untuk diedit di aplikasi musik/video. Ukurannya besar', 'WAV is uncompressed, good for editing in music/video apps. Files are large')
      : L('FLAC tanpa kehilangan kualitas dari sumbernya. Ukurannya besar', 'FLAC keeps the source quality without loss. Files are large')}
    (± {mbPerMin} MB {L('per menit', 'per minute')}).
    {L('Kualitasnya tetap mengikuti sumber — tidak lebih bagus dari aslinya.', "Quality still follows the source — it can't be better than the original.")}
  </p>
{:else}
  <div class="sec">
    <span class="label">{L('Bitrate', 'Bitrate')}</span>
    <Chips
      bind:value={opts.audioQuality}
      columns={list.length + 1}
      tall
      {onchange}
      options={[
        { value: 'auto' as Rate, label: L('Otomatis', 'Auto'), sub: L('ikut sumber', 'as source') },
        ...list.map((r) => ({ value: r.value, label: `${r.value}`, sub: tagText(r.tag) || 'kbps' })),
      ]}
    />
    <p class="hint">
      {#if opts.audioQuality === 'auto'}
        {L('Otomatis memakai kualitas terbaik sesuai sumber (VBR).', 'Auto uses the best quality the source allows (VBR).')}
      {:else}
        {opts.audioQuality} kbps ≈ {mbPerMin} MB {L('per menit', 'per minute')}.
      {/if}
      {L(`Rekomendasi: ${recommended[opts.audioFormat]} kbps.`, `Recommended: ${recommended[opts.audioFormat]} kbps.`)}
      {L('Audio YouTube aslinya sekitar 128–160 kbps, jadi bitrate lebih tinggi hanya menambah ukuran, bukan kualitas.', 'YouTube audio is about 128–160 kbps to begin with, so a higher bitrate only adds size, not quality.')}
    </p>
  </div>
{/if}

<style>
  .sec {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
</style>
