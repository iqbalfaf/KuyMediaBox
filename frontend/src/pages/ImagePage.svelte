<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import PageHeader from '../components/PageHeader.svelte'
  import FileList from '../components/FileList.svelte'
  import EmptyDrop from '../components/EmptyDrop.svelte'
  import Chips from '../components/Chips.svelte'
  import Select from '../components/Select.svelte'
  import Switch from '../components/Switch.svelte'
  import OutputPicker from '../components/OutputPicker.svelte'
  import RunFooter from '../components/RunFooter.svelte'
  import Icon from '../components/Icon.svelte'
  import { api } from '../lib/api'
  import { imageConv as conv } from '../lib/stores/converter.svelte'
  import { load, save } from '../lib/stores/persist'
  import { bytes, tile } from '../lib/format'
  import type { FileItem, ImageOptions } from '../lib/types'

  const defaults: { o: ImageOptions } = {
    o: { format: 'jpg', quality: 85, resizeMode: 'longest', longest: 1920, percent: 50, width: 1920, height: 1080, background: '#ffffff', autoRotate: true },
  }
  const st = $state(load('kmb.image', defaults))
  $effect(() => save('kmb.image', $state.snapshot(st)))

  const formatHints = $derived<Record<string, string>>({
    jpg: L('JPG: paling kompatibel, cocok untuk foto.', 'JPG: most compatible, great for photos.'),
    png: L('PNG: tanpa kehilangan kualitas, mendukung transparan.', 'PNG: lossless, supports transparency.'),
    webp: L('WEBP: ukuran kecil, kualitas bagus, didukung semua browser modern.', 'WEBP: small files, good quality, supported by all modern browsers.'),
    avif: L('AVIF: paling kecil, tapi proses lebih lama.', 'AVIF: smallest files, but slower to encode.'),
    pdf: L('PDF: setiap gambar menjadi satu file PDF.', 'PDF: each image becomes its own PDF file.'),
    ico: L('ICO: ikon Windows, maksimal 256×256 px.', 'ICO: Windows icon, up to 256×256 px.'),
    bmp: L('BMP: tanpa kompresi, ukuran besar.', 'BMP: uncompressed, large files.'),
    tiff: L('TIFF: untuk cetak & arsip, tanpa kehilangan kualitas.', 'TIFF: for print & archiving, lossless.'),
  })
  const hasQuality = $derived(['jpg', 'webp', 'avif', 'pdf'].includes(st.o.format))
  const needsBg = $derived(['jpg', 'bmp', 'pdf'].includes(st.o.format))
  const swatches = ['#ffffff', '#000000']
  let colorInput = $state<HTMLInputElement>()

  function target(it: FileItem): string {
    const w = it.width
    const h = it.height
    if (!w || !h) return st.o.format.toUpperCase()
    let nw = w
    let nh = h
    const o = st.o
    if (o.resizeMode === 'longest' && o.longest > 0 && Math.max(w, h) > o.longest) {
      if (w >= h) [nw, nh] = [o.longest, Math.round((h * o.longest) / w)]
      else [nw, nh] = [Math.round((w * o.longest) / h), o.longest]
    } else if (o.resizeMode === 'percent' && o.percent > 0 && o.percent < 100) {
      ;[nw, nh] = [Math.round((w * o.percent) / 100), Math.round((h * o.percent) / 100)]
    } else if (o.resizeMode === 'box') {
      const bw = o.width > 0 ? o.width : w
      const bh = o.height > 0 ? o.height : h
      if (w > bw || h > bh) {
        const s = Math.min(bw / w, bh / h)
        ;[nw, nh] = [Math.round(w * s), Math.round(h * s)]
      }
    }
    if (o.format === 'ico' && Math.max(nw, nh) > 256) {
      if (nw >= nh) [nw, nh] = [256, Math.round((nh * 256) / nw)]
      else [nw, nh] = [Math.round((nw * 256) / nh), 256]
    }
    return `${st.o.format.toUpperCase()} · ${Math.max(1, nw)}×${Math.max(1, nh)}`
  }

  function meta(it: FileItem): string {
    const parts = []
    if (it.width) parts.push(`${it.width}×${it.height}`)
    parts.push(it.ext.toUpperCase())
    parts.push(bytes(it.size))
    return parts.join(' · ')
  }

  function numberInput(e: Event, key: 'longest' | 'percent' | 'width' | 'height', max: number) {
    const v = parseInt((e.currentTarget as HTMLInputElement).value.replace(/\D/g, ''), 10)
    st.o[key] = isNaN(v) ? 0 : Math.min(max, v)
  }

  const pending = $derived(conv.pending())
  const invalidResize = $derived(
    (st.o.resizeMode === 'longest' && !(st.o.longest >= 1)) ||
      (st.o.resizeMode === 'percent' && !(st.o.percent >= 1 && st.o.percent <= 100)) ||
      (st.o.resizeMode === 'box' && !(st.o.width >= 1 || st.o.height >= 1)),
  )

  function start() {
    conv.start(pending, (items) => api.startImage(items, $state.snapshot(st.o)))
  }
</script>

<PageHeader title={L('Konversi Gambar', 'Image Converter')} subtitle={L('Ubah format, ukuran, dan kualitas banyak gambar sekaligus.', 'Change the format, size and quality of many images at once.')} />

<div class="body">
  {#if conv.items.length === 0}
    <EmptyDrop
      {conv}
      title={L('Tarik & lepas gambar ke sini', 'Drag & drop images here')}
      subtitle={L('Bisa banyak file sekaligus, atau satu folder penuh.', 'Many files at once, or a whole folder.')}
      pickLabel={L('Pilih gambar', 'Choose images')}
      formats={['JPG', 'PNG', 'WEBP', 'HEIC', 'AVIF', 'BMP', 'TIFF', 'GIF', 'ICO']}
      steps={[
        [L('Tambahkan gambar', 'Add images'), L('Tarik ke sini atau klik tombol', 'Drag them here or click the button')],
        [L('Pilih format & ukuran', 'Pick format & size'), L('Di panel sebelah kanan', 'In the panel on the right')],
        [L('Klik Mulai', 'Click Start'), L('File asli tidak akan diubah', 'Your original files stay untouched')],
      ]}
    />
  {:else}
    <FileList
      {conv}
      noun={L('gambar', 'images')}
      dropText={L('Tarik & lepas gambar atau folder di sini', 'Drag & drop images or folders here')}
      formats="JPG · PNG · WEBP · HEIC · AVIF · BMP · TIFF · GIF"
      {meta}
    >
      {#snippet thumb(it)}
        {@const t = tile(it.name)}
        <div class="kmb-tile" style="background: {t.bg}; color: {t.fg}"><Icon name="image" /></div>
      {/snippet}
      {#snippet result(it)}
        {@const task = conv.task(it)}
        {@const s = conv.state(it)}
        <span class="r1">{target(it)}</span>
        {#if s === 'done' && task}
          {@const diff = it.size ? Math.round(((task.outSize - it.size) / it.size) * 100) : 0}
          <span class="r2">{bytes(task.outSize)} <span class={diff <= 0 ? 'saved' : 'grew'}>{diff <= 0 ? `−${Math.abs(diff)}%` : `+${diff}%`}</span></span>
        {:else if s === 'running'}
          <span class="r2">{L('Sedang diproses…', 'Processing…')}</span>
        {:else if s === 'canceled'}
          <span class="r2">{L('Dibatalkan', 'Canceled')}</span>
        {:else}
          <span class="r2">{bytes(it.size)} → ?</span>
        {/if}
      {/snippet}
    </FileList>
  {/if}

  <aside class="card panel" aria-label={L('Pengaturan output', 'Output settings')}>
    <div class="scroll">
      <div class="sec">
        <span class="label">{L('Format hasil', 'Output format')}</span>
        <Chips
          bind:value={st.o.format}
          columns={4}
          options={[
            { value: 'jpg', label: 'JPG' }, { value: 'png', label: 'PNG' }, { value: 'webp', label: 'WEBP' }, { value: 'avif', label: 'AVIF' },
            { value: 'pdf', label: 'PDF' }, { value: 'ico', label: 'ICO' }, { value: 'bmp', label: 'BMP' }, { value: 'tiff', label: 'TIFF' },
          ]}
        />
        <p class="hint">{formatHints[st.o.format]}</p>
      </div>

      {#if hasQuality}
        <div class="sec tight">
          <div class="row-between">
            <label class="label" for="img-q">{L('Kualitas', 'Quality')}</label>
            <span class="val">{st.o.quality}</span>
          </div>
          <input id="img-q" type="range" min="1" max="100" bind:value={st.o.quality} />
          <div class="row-between small"><span>{L('File lebih kecil', 'Smaller file')}</span><span>{L('Lebih tajam', 'Sharper')}</span></div>
        </div>
      {/if}

      <div class="sec">
        <span class="label">{L('Ukuran', 'Size')}</span>
        <div class="inline">
          <Select
            label={L('Cara mengubah ukuran', 'Resize mode')}
            bind:value={st.o.resizeMode}
            options={[
              { value: 'original', label: L('Ukuran asli', 'Original size') },
              { value: 'longest', label: L('Sisi terpanjang', 'Longest side') },
              { value: 'percent', label: L('Persentase', 'Percentage') },
              { value: 'box', label: L('Lebar × tinggi maks.', 'Max width × height') },
            ]}
          />
          {#if st.o.resizeMode === 'longest'}
            <div class="num"><input class="text-input" aria-label={L('Sisi terpanjang (piksel)', 'Longest side (pixels)')} inputmode="numeric" value={st.o.longest || ''} oninput={(e) => numberInput(e, 'longest', 20000)} /><span>px</span></div>
          {:else if st.o.resizeMode === 'percent'}
            <div class="num"><input class="text-input" aria-label={L('Persentase', 'Percentage')} inputmode="numeric" value={st.o.percent || ''} oninput={(e) => numberInput(e, 'percent', 100)} /><span>%</span></div>
          {/if}
        </div>
        {#if st.o.resizeMode === 'box'}
          <div class="inline">
            <div class="num wide"><input class="text-input" aria-label={L('Lebar maksimal', 'Max width')} placeholder={L('Lebar', 'Width')} inputmode="numeric" value={st.o.width || ''} oninput={(e) => numberInput(e, 'width', 20000)} /><span>px</span></div>
            <span class="x">×</span>
            <div class="num wide"><input class="text-input" aria-label={L('Tinggi maksimal', 'Max height')} placeholder={L('Tinggi', 'Height')} inputmode="numeric" value={st.o.height || ''} oninput={(e) => numberInput(e, 'height', 20000)} /><span>px</span></div>
          </div>
        {/if}
        <p class="hint">
          {#if st.o.resizeMode === 'original'}{L('Ukuran tidak diubah.', 'Size stays the same.')}{:else}{L('Rasio tetap terjaga. Gambar yang lebih kecil tidak diperbesar.', 'Aspect ratio is kept. Smaller images are never upscaled.')}{/if}
          {#if st.o.format === 'ico'} {L('ICO dibatasi 256 px.', 'ICO is limited to 256 px.')}{/if}
        </p>
      </div>

      <div class="sec">
        <span class="label">{L('Lainnya', 'Other')}</span>
        {#if needsBg}
          <div class="row-between">
            <div class="col">
              <span class="t13">{L('Latar area transparan', 'Background for transparency')}</span>
              <span class="t12">{st.o.format.toUpperCase()} {L('tanpa transparansi', 'has no transparency')}</span>
            </div>
            <div class="swatches">
              {#each swatches as c}
                <button class="sw" class:on={st.o.background.toLowerCase() === c} style="background: {c}" aria-label={c === '#ffffff' ? L('Latar putih', 'White background') : L('Latar hitam', 'Black background')} aria-pressed={st.o.background.toLowerCase() === c} onclick={() => (st.o.background = c)}></button>
              {/each}
              <button
                class="sw custom"
                class:on={!swatches.includes(st.o.background.toLowerCase())}
                style={!swatches.includes(st.o.background.toLowerCase()) ? `background: ${st.o.background}` : ''}
                aria-label={L('Pilih warna lain', 'Pick another color')}
                onclick={() => colorInput?.click()}
              >
                {#if swatches.includes(st.o.background.toLowerCase())}<Icon name="plus" size={14} stroke={2.5} />{/if}
              </button>
              <input class="hidden-color" type="color" bind:this={colorInput} bind:value={st.o.background} tabindex="-1" aria-hidden="true" />
            </div>
          </div>
        {/if}
        <Switch bind:checked={st.o.autoRotate} label={L('Putar otomatis', 'Auto-rotate')} hint={L('Ikuti orientasi kamera (EXIF)', 'Follow the camera orientation (EXIF)')} />
      </div>

      <div class="sec">
        <span class="label">{L('Simpan ke', 'Save to')}</span>
        <OutputPicker kind="image" />
      </div>
    </div>

    <RunFooter
      kind="image"
      running={conv.running}
      busy={conv.starting}
      startLabel={conv.hasUnprocessed() || pending.length === 0 ? `${L('Mulai konversi', 'Start converting')}${pending.length ? ` (${pending.length})` : ''}` : `${L('Konversi ulang', 'Convert again')} (${pending.length})`}
      disabled={pending.length === 0 || invalidResize}
      disabledHint={conv.items.length === 0 ? L('Tambahkan gambar dulu untuk memulai.', 'Add images to get started.') : invalidResize ? L('Isi ukuran yang valid.', 'Enter a valid size.') : ''}
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
    padding: 18px 20px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .sec {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .sec.tight {
    gap: 8px;
  }
  .row-between {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
  }
  .row-between.small {
    font-size: 11px;
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
  .inline {
    display: flex;
    gap: 8px;
    align-items: center;
  }
  .num {
    position: relative;
    width: 104px;
    flex-shrink: 0;
  }
  .num.wide {
    flex: 1;
    width: auto;
  }
  .num input {
    padding-right: 30px;
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
  .x {
    color: var(--text-3);
  }
  .col {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .t13 {
    font-size: 13px;
    font-weight: 600;
  }
  .t12 {
    font-size: 12px;
    color: var(--text-3);
  }
  .swatches {
    display: flex;
    gap: 6px;
    position: relative;
  }
  .sw {
    width: 30px;
    height: 30px;
    padding: 0;
    border-radius: 8px;
    border: 2px solid #3a414e;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-2);
  }
  .sw.custom {
    background: var(--surface-2);
  }
  .sw.on {
    border-color: var(--accent);
    box-shadow: inset 0 0 0 2px var(--surface);
  }
  .hidden-color {
    position: absolute;
    width: 0;
    height: 0;
    opacity: 0;
    pointer-events: none;
    right: 0;
    bottom: 0;
  }
</style>
