<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import PageHeader from '../components/PageHeader.svelte'
  import FileList from '../components/FileList.svelte'
  import EmptyDrop from '../components/EmptyDrop.svelte'
  import Chips from '../components/Chips.svelte'
  import Select from '../components/Select.svelte'
  import Switch from '../components/Switch.svelte'
  import Segmented from '../components/Segmented.svelte'
  import PositionGrid from '../components/PositionGrid.svelte'
  import OutputPicker from '../components/OutputPicker.svelte'
  import RunFooter from '../components/RunFooter.svelte'
  import PresetBar from '../components/PresetBar.svelte'
  import Icon from '../components/Icon.svelte'
  import { showCompare } from '../components/CompareDialog.svelte'
  import { api, errText, imageUrl } from '../lib/api'
  import { toast } from '../lib/stores/app.svelte'
  import { imageConv as conv } from '../lib/stores/converter.svelte'
  import { load, save } from '../lib/stores/persist'
  import { bytes, tile } from '../lib/format'
  import type { FileItem, ImageOptions } from '../lib/types'

  const defaults: { o: ImageOptions } = {
    o: {
      format: 'jpg', quality: 85, resizeMode: 'longest', longest: 1920, percent: 50, width: 1920, height: 1080, background: '#ffffff', autoRotate: true,
      keepMetadata: false, targetKB: 0, icoSizes: [16, 32, 48, 256], rotate: 0, flipH: false, flipV: false, crop: '',
      watermark: { enabled: false, type: 'text', text: '© ', bold: true, color: '#ffffff', image: '', size: 5, opacity: 0.6, angle: 0, position: 'br', margin: 3 },
    },
  }
  const st = $state(load('kmb.image', defaults))
  $effect(() => save('kmb.image', $state.snapshot(st)))

  const builtins = $derived([
    { id: 'b-web', name: L('Web ringan (WEBP, maks. 200 KB)', 'Light web (WEBP, max 200 KB)'), value: { format: 'webp', quality: 80, resizeMode: 'longest', longest: 1600, targetKB: 200, keepMetadata: false } },
    { id: 'b-wa', name: L('Foto WhatsApp (JPG 1600 px)', 'WhatsApp photo (JPG 1600 px)'), value: { format: 'jpg', quality: 82, resizeMode: 'longest', longest: 1600, targetKB: 0 } },
    { id: 'b-ig', name: L('Instagram 4:5 (1080 px)', 'Instagram 4:5 (1080 px)'), value: { format: 'jpg', quality: 90, crop: '4:5', resizeMode: 'box', width: 1080, height: 1350, targetKB: 0 } },
    { id: 'b-ico', name: L('Ikon aplikasi (ICO semua ukuran)', 'App icon (ICO, all sizes)'), value: { format: 'ico', icoSizes: [16, 24, 32, 48, 64, 128, 256], crop: '1:1', targetKB: 0 } },
    { id: 'b-500', name: L('Dokumen / formulir (≤ 500 KB)', 'Documents / forms (≤ 500 KB)'), value: { format: 'jpg', quality: 90, resizeMode: 'original', targetKB: 500 } },
  ])

  const formatHints = $derived<Record<string, string>>({
    jpg: L('JPG: paling kompatibel, cocok untuk foto.', 'JPG: most compatible, great for photos.'),
    png: L('PNG: tanpa kehilangan kualitas, mendukung transparan.', 'PNG: lossless, supports transparency.'),
    webp: L('WEBP: ukuran kecil, kualitas bagus, didukung semua browser modern.', 'WEBP: small files, good quality, supported by all modern browsers.'),
    avif: L('AVIF: paling kecil, tapi proses lebih lama.', 'AVIF: smallest files, but slower to encode.'),
    pdf: L('PDF: setiap gambar menjadi satu file PDF.', 'PDF: each image becomes its own PDF file.'),
    ico: L('ICO: ikon Windows berisi beberapa ukuran sekaligus.', 'ICO: a Windows icon holding several sizes.'),
    bmp: L('BMP: tanpa kompresi, ukuran besar.', 'BMP: uncompressed, large files.'),
    tiff: L('TIFF: untuk cetak & arsip, tanpa kehilangan kualitas.', 'TIFF: for print & archiving, lossless.'),
  })
  const hasQuality = $derived(['jpg', 'webp', 'avif', 'pdf'].includes(st.o.format))
  const needsBg = $derived(['jpg', 'bmp', 'pdf'].includes(st.o.format))
  const canKeepExif = $derived(st.o.format === 'jpg' || st.o.format === 'png')
  const canTarget = $derived(st.o.format !== 'ico')
  const swatches = ['#ffffff', '#000000']
  const icoAll = [16, 24, 32, 48, 64, 128, 256]
  let colorInput = $state<HTMLInputElement>()

  function ratio(r: string): number {
    const [a, b] = r.split(':').map(Number)
    return a && b ? a / b : 0
  }

  function target(it: FileItem): string {
    let w = it.width
    let h = it.height
    if (!w || !h) return st.o.format.toUpperCase()
    const o = st.o
    if (o.rotate === 90 || o.rotate === 270) [w, h] = [h, w]
    const r = ratio(o.crop)
    if (r) {
      if (w / h > r) w = Math.max(1, Math.round(h * r))
      else h = Math.max(1, Math.round(w / r))
    }
    let nw = w
    let nh = h
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
    if (o.format === 'ico') return `ICO · ${o.icoSizes.length ? o.icoSizes.slice().sort((a, b) => a - b).join('/') : Math.min(256, Math.max(nw, nh))}`
    return `${st.o.format.toUpperCase()} · ${Math.max(1, nw)}×${Math.max(1, nh)}${canTarget && o.targetKB > 0 ? ` · ≤ ${o.targetKB} KB` : ''}`
  }

  function meta(it: FileItem): string {
    const parts = []
    if (it.width) parts.push(`${it.width}×${it.height}`)
    parts.push(it.ext.toUpperCase())
    parts.push(bytes(it.size))
    return parts.join(' · ')
  }

  function numberInput(e: Event, key: 'longest' | 'percent' | 'width' | 'height' | 'targetKB', max: number) {
    const v = parseInt((e.currentTarget as HTMLInputElement).value.replace(/\D/g, ''), 10)
    st.o[key] = isNaN(v) ? 0 : Math.min(max, v)
  }

  function toggleIco(size: number) {
    const set = new Set(st.o.icoSizes)
    if (set.has(size)) set.delete(size)
    else set.add(size)
    st.o.icoSizes = [...set].sort((a, b) => a - b)
  }

  async function pickLogo() {
    try {
      const p = await api.pickFile(L('Pilih gambar logo', 'Choose the logo picture'), L('Gambar', 'Images'), '*.png;*.jpg;*.jpeg;*.webp;*.bmp;*.gif')
      if (p) st.o.watermark.image = p
    } catch (e) {
      toast(errText(e), 'err')
    }
  }

  const previewable = new Set(['jpg', 'png', 'webp', 'avif', 'bmp', 'tiff', 'ico'])
  function openCompare(it: FileItem) {
    const t = conv.task(it)
    if (!t?.output) return
    const ext = t.output.split('.').pop()?.toLowerCase() ?? ''
    if (!previewable.has(ext)) {
      api.revealFile(t.output)
      return
    }
    showCompare(it.name, it.path, t.output, it.size, t.outSize)
  }

  const pending = $derived(conv.pending())
  const invalidResize = $derived(
    (st.o.resizeMode === 'longest' && !(st.o.longest >= 1)) ||
      (st.o.resizeMode === 'percent' && !(st.o.percent >= 1 && st.o.percent <= 100)) ||
      (st.o.resizeMode === 'box' && !(st.o.width >= 1 || st.o.height >= 1)),
  )
  const invalidIco = $derived(st.o.format === 'ico' && st.o.icoSizes.length === 0)
  const invalidWm = $derived(st.o.watermark.enabled && (st.o.watermark.type === 'text' ? !st.o.watermark.text.trim() : !st.o.watermark.image))

  function start() {
    const o = $state.snapshot(st.o)
    if (!canTarget) o.targetKB = 0
    conv.start(pending, (items) => api.startImage(items, o))
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
        {@const done = conv.state(it) === 'done'}
        <button class="thumb" class:done title={done ? L('Bandingkan sebelum & sesudah', 'Compare before & after') : it.name} onclick={() => done && openCompare(it)}>
          <div class="kmb-tile fallback" style="background: {t.bg}; color: {t.fg}"><Icon name="image" /></div>
          <img src={imageUrl(it.path, 96)} alt="" loading="lazy" onerror={(e) => ((e.currentTarget as HTMLImageElement).style.display = 'none')} />
          {#if done}<span class="cmp"><Icon name="columns" size={11} /></span>{/if}
        </button>
      {/snippet}
      {#snippet result(it)}
        {@const task = conv.task(it)}
        {@const s = conv.state(it)}
        <span class="r1">{target(it)}</span>
        {#if s === 'done' && task}
          {@const diff = it.size ? Math.round(((task.outSize - it.size) / it.size) * 100) : 0}
          <span class="r2">{bytes(task.outSize)} <span class={diff <= 0 ? 'saved' : 'grew'}>{diff <= 0 ? `−${Math.abs(diff)}%` : `+${diff}%`}</span>
            <button class="cmp-link" onclick={() => openCompare(it)}>{L('Bandingkan', 'Compare')}</button></span>
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
      <PresetBar module="image" target={st.o} {builtins} />

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

      {#if st.o.format === 'ico'}
        <div class="sec">
          <span class="label">{L('Ukuran ikon', 'Icon sizes')}</span>
          <div class="multi">
            {#each icoAll as size}
              <button class="mchip" class:on={st.o.icoSizes.includes(size)} aria-pressed={st.o.icoSizes.includes(size)} onclick={() => toggleIco(size)}>{size}</button>
            {/each}
          </div>
          <p class="hint">{L('Semua ukuran disimpan dalam satu file .ico. Gambar yang tidak persegi diberi latar transparan.', 'All sizes go into one .ico file. Non-square pictures get a transparent border.')}</p>
        </div>
      {/if}

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

      {#if canTarget}
        <div class="sec">
          <Switch
            checked={st.o.targetKB > 0}
            onchange={(v) => (st.o.targetKB = v ? 500 : 0)}
            label={L('Batasi ukuran file', 'Limit the file size')}
            hint={L('Kualitas lalu ukuran gambar diturunkan sampai muat', 'Quality, then picture size, is lowered until it fits')}
          />
          {#if st.o.targetKB > 0}
            <div class="inline">
              <div class="num wide"><input class="text-input" aria-label={L('Ukuran maksimal (KB)', 'Max size (KB)')} inputmode="numeric" value={st.o.targetKB || ''} oninput={(e) => numberInput(e, 'targetKB', 100000)} /><span>KB</span></div>
              {#each [100, 200, 500, 1000] as kb}
                <button class="mchip small" class:on={st.o.targetKB === kb} onclick={() => (st.o.targetKB = kb)}>{kb >= 1000 ? '1 MB' : kb}</button>
              {/each}
            </div>
          {/if}
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
        </p>
      </div>

      <div class="sec">
        <span class="label">{L('Putar & potong', 'Rotate & crop')}</span>
        <div class="tools">
          <button class="tbtn" title={L('Putar ke kiri', 'Rotate left')} aria-label={L('Putar ke kiri', 'Rotate left')} onclick={() => (st.o.rotate = (st.o.rotate + 270) % 360)}><Icon name="rotateCcw" size={16} /></button>
          <button class="tbtn" title={L('Putar ke kanan', 'Rotate right')} aria-label={L('Putar ke kanan', 'Rotate right')} onclick={() => (st.o.rotate = (st.o.rotate + 90) % 360)}><Icon name="rotateCw" size={16} /></button>
          <button class="tbtn" class:on={st.o.flipH} aria-pressed={st.o.flipH} title={L('Balik horizontal', 'Flip horizontally')} aria-label={L('Balik horizontal', 'Flip horizontally')} onclick={() => (st.o.flipH = !st.o.flipH)}><Icon name="flipH" size={16} /></button>
          <button class="tbtn" class:on={st.o.flipV} aria-pressed={st.o.flipV} title={L('Balik vertikal', 'Flip vertically')} aria-label={L('Balik vertikal', 'Flip vertically')} onclick={() => (st.o.flipV = !st.o.flipV)}><Icon name="flipV" size={16} /></button>
          <span class="rot">{st.o.rotate}°</span>
          {#if st.o.rotate || st.o.flipH || st.o.flipV}
            <button class="link" onclick={() => { st.o.rotate = 0; st.o.flipH = false; st.o.flipV = false }}>Reset</button>
          {/if}
        </div>
        <Chips
          bind:value={st.o.crop}
          columns={4}
          small
          options={[
            { value: '', label: L('Asli', 'Original') }, { value: '1:1', label: '1:1' }, { value: '4:5', label: '4:5' }, { value: '16:9', label: '16:9' },
            { value: '9:16', label: '9:16' }, { value: '4:3', label: '4:3' }, { value: '3:2', label: '3:2' }, { value: '2:3', label: '2:3' },
          ]}
        />
        {#if st.o.crop}<p class="hint">{L('Dipotong di bagian tengah gambar.', 'Cropped around the centre of the picture.')}</p>{/if}
      </div>

      <div class="sec">
        <Switch bind:checked={st.o.watermark.enabled} label="Watermark" hint={L('Teks atau logo di setiap gambar', 'Text or a logo on every picture')} />
        {#if st.o.watermark.enabled}
          <Segmented
            label={L('Jenis watermark', 'Watermark type')}
            bind:value={st.o.watermark.type}
            options={[
              { value: 'text', label: L('Teks', 'Text'), icon: 'type' },
              { value: 'image', label: 'Logo', icon: 'image' },
            ]}
          />
          {#if st.o.watermark.type === 'text'}
            <div class="inline">
              <input class="text-input" aria-label={L('Teks watermark', 'Watermark text')} bind:value={st.o.watermark.text} placeholder="© Nama" />
              <input class="color" type="color" aria-label={L('Warna teks', 'Text colour')} bind:value={st.o.watermark.color} />
              <button class="tbtn" class:on={st.o.watermark.bold} aria-pressed={st.o.watermark.bold} title={L('Tebal', 'Bold')} onclick={() => (st.o.watermark.bold = !st.o.watermark.bold)}><b>B</b></button>
            </div>
          {:else}
            <div class="inline">
              <button class="btn grow" onclick={pickLogo}><Icon name="image" size={16} />{st.o.watermark.image ? L('Ganti logo', 'Change logo') : L('Pilih logo…', 'Choose logo…')}</button>
            </div>
            {#if st.o.watermark.image}<span class="hint ellipsis" title={st.o.watermark.image}>{st.o.watermark.image.split(/[\\/]/).pop()}</span>{/if}
          {/if}
          <div class="wm">
            <div class="wm-pos">
              <PositionGrid label={L('Posisi', 'Position')} bind:value={st.o.watermark.position} />
              <button class="mchip small" class:on={st.o.watermark.position === 'tile'} onclick={() => (st.o.watermark.position = st.o.watermark.position === 'tile' ? 'br' : 'tile')}>{L('Berulang', 'Tiled')}</button>
            </div>
            <div class="wm-sliders">
              <label class="t12" for="wm-size">{st.o.watermark.type === 'text' ? L('Ukuran huruf', 'Text size') : L('Lebar logo', 'Logo width')} · {st.o.watermark.size}%</label>
              <input id="wm-size" type="range" min="1" max={st.o.watermark.type === 'text' ? 30 : 100} bind:value={st.o.watermark.size} />
              <label class="t12" for="wm-op">{L('Transparansi', 'Opacity')} · {Math.round(st.o.watermark.opacity * 100)}%</label>
              <input id="wm-op" type="range" min="0.05" max="1" step="0.05" bind:value={st.o.watermark.opacity} />
              <label class="t12" for="wm-ang">{L('Kemiringan', 'Angle')} · {st.o.watermark.angle}°</label>
              <input id="wm-ang" type="range" min="-90" max="90" step="5" bind:value={st.o.watermark.angle} />
            </div>
          </div>
        {/if}
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
        <Switch
          checked={!st.o.keepMetadata}
          onchange={(v) => (st.o.keepMetadata = !v)}
          label={L('Hapus metadata EXIF', 'Remove EXIF metadata')}
          hint={canKeepExif || !st.o.keepMetadata
            ? L('Lokasi GPS, kamera & tanggal tidak ikut tersimpan (lebih privat)', 'GPS location, camera & date are not kept (more private)')
            : L('Metadata hanya bisa dipertahankan untuk hasil JPG/PNG', 'Metadata can only be kept for JPG/PNG results')}
        />
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
      disabled={pending.length === 0 || invalidResize || invalidIco || invalidWm}
      disabledHint={conv.items.length === 0
        ? L('Tambahkan gambar dulu untuk memulai.', 'Add images to get started.')
        : invalidResize
          ? L('Isi ukuran yang valid.', 'Enter a valid size.')
          : invalidIco
            ? L('Pilih minimal satu ukuran ikon.', 'Pick at least one icon size.')
            : invalidWm
              ? L('Isi teks atau pilih logo watermark.', 'Enter the watermark text or choose a logo.')
              : ''}
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
    padding-right: 34px;
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
    border: 2px solid var(--swatch-border);
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
    white-space: nowrap;
  }
  .mchip.small {
    height: 32px;
  }
  .mchip.on {
    border-color: var(--accent);
    background: var(--accent-soft);
    color: var(--accent-text);
    font-weight: 700;
  }
  .tools {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .tbtn {
    width: 36px;
    height: 34px;
    flex-shrink: 0;
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
  .link {
    padding: 0;
    border: 0;
    background: none;
    font-size: 12px;
    font-weight: 700;
    color: var(--accent-text-2);
  }
  .color {
    width: 40px;
    height: 40px;
    padding: 2px;
    border-radius: 10px;
    border: 1px solid var(--border);
    background: var(--surface-2);
    flex-shrink: 0;
  }
  .grow {
    flex: 1;
  }
  .wm {
    display: flex;
    gap: 12px;
  }
  .wm-pos {
    display: flex;
    flex-direction: column;
    gap: 6px;
    align-items: stretch;
  }
  .wm-sliders {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }
  .thumb {
    position: relative;
    width: 40px;
    height: 40px;
    flex-shrink: 0;
    padding: 0;
    border: 0;
    border-radius: 8px;
    overflow: hidden;
    background: transparent;
    cursor: default;
  }
  .thumb.done {
    cursor: zoom-in;
  }
  .thumb .fallback {
    position: absolute;
    inset: 0;
  }
  .thumb img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .cmp {
    position: absolute;
    right: 2px;
    bottom: 2px;
    width: 16px;
    height: 16px;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--accent);
    color: var(--accent-ink);
  }
  .cmp-link {
    padding: 0;
    margin-left: 6px;
    border: 0;
    background: none;
    font-size: 12px;
    font-weight: 700;
    color: var(--accent-text-2);
  }
  .cmp-link:hover {
    text-decoration: underline;
  }
</style>
