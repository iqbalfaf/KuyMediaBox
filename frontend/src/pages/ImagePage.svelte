<script lang="ts">
  import { L, locale } from '../lib/i18n.svelte'
  import PageHeader from '../components/PageHeader.svelte'
  import FileList from '../components/FileList.svelte'
  import EmptyDrop from '../components/EmptyDrop.svelte'
  import Chips from '../components/Chips.svelte'
  import Select from '../components/Select.svelte'
  import Switch from '../components/Switch.svelte'
  import Segmented from '../components/Segmented.svelte'
  import WatermarkFields from '../components/WatermarkFields.svelte'
  import ModelPicker from '../components/ModelPicker.svelte'
  import ToolBanner from '../components/ToolBanner.svelte'
  import { pickCrop } from '../components/CropDialog.svelte'
  import { modelReady } from '../lib/stores/models.svelte'
  import OutputPicker from '../components/OutputPicker.svelte'
  import RunFooter from '../components/RunFooter.svelte'
  import PresetBar from '../components/PresetBar.svelte'
  import Icon from '../components/Icon.svelte'
  import { showCompare } from '../components/CompareDialog.svelte'
  import { api, errText, imageUrl } from '../lib/api'
  import { hasTool, toast } from '../lib/stores/app.svelte'
  import { imageConv as conv } from '../lib/stores/converter.svelte'
  import { load, save } from '../lib/stores/persist'
  import { imageDefaults, imageBuiltins } from '../lib/modules'
  import { bytes, tile } from '../lib/format'
  import type { CollageJob, FileItem, ImageOptions, SlideOptions } from '../lib/types'

  const defaults = imageDefaults
  const st = $state(load('kmb.image', defaults))
  $effect(() => save('kmb.image', $state.snapshot(st)))

  const builtins = $derived(imageBuiltins())

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
    if (o.cropBox?.w > 0) [w, h] = [Math.max(1, Math.round(w * o.cropBox.w)), Math.max(1, Math.round(h * o.cropBox.h))]
    const r = ratio(o.crop)
    if (r) {
      if (w / h > r) w = Math.max(1, Math.round(h * r))
      else h = Math.max(1, Math.round(w / r))
    }
    if (o.aiUpscale > 0) [w, h] = [w * o.aiUpscale, h * o.aiUpscale]
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

  const previewItem = $derived(conv.items.find((it) => !it.error))
  function openCrop() {
    const it = previewItem
    if (!it) return
    const q = encodeURIComponent(it.path)
    pickCrop(
      { title: it.name, image: `/kmb/imgx?path=${q}&w=1400&rot=${st.o.rotate}&fh=${st.o.flipH ? 1 : 0}&fv=${st.o.flipV ? 1 : 0}`, box: $state.snapshot(st.o.cropBox) },
      (b) => {
        st.o.cropBox = b
        if (b.w > 0) st.o.crop = ''
      },
    )
  }

  // Tools the chosen options need (shown in the banner when missing).
  const needed = $derived(st.mode !== 'convert' ? [] : [
    ...(st.o.aiUpscale > 0 ? ['realesrgan'] : []),
    ...(st.o.removeBg ? ['onnxruntime'] : []),
    ...(st.o.format === 'png' && st.o.pngCompress === 'lossless' ? ['oxipng'] : []),
    ...(st.o.format === 'png' && st.o.pngCompress === 'small' ? ['pngquant', 'oxipng'] : []),
  ])
  const bgModels: Record<string, [string, string]> = $derived({
    general: [L('Umum (terbaik)', 'General (best)'), L('produk, hewan, benda, orang', 'products, animals, objects, people')],
    people: [L('Orang', 'People'), L('foto potret & seluruh badan', 'portraits & full body')],
    fast: [L('Cepat', 'Fast'), L('model kecil, tepi kurang halus', 'small model, rougher edges')],
  })
  const bgNotReady = $derived(st.o.removeBg && !modelReady('bgremove', st.o.bgModel))
  const toolsMissing = $derived(needed.some((id: string) => !hasTool(id)))

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

  const combined = $derived(st.mode !== 'convert')
  const usable = $derived(conv.items.filter((it) => !it.error))
  const fewForCombined = $derived(combined && usable.length < 2)

  function start() {
    if (st.mode === 'anim') {
      const o = $state.snapshot(st.anim)
      conv.start(usable, (items) => api.startSlideshow(items, o))
      return
    }
    if (st.mode === 'collage') {
      const job = $state.snapshot(st.collage)
      conv.start(usable, (items) => api.startCollage(items, job))
      return
    }
    const o = $state.snapshot(st.o)
    if (!canTarget) o.targetKB = 0
    conv.start(pending, (items) => api.startImage(items, o))
  }

  async function pickSlideMusic() {
    try {
      const p = await api.pickFile(L('Pilih musik', 'Choose music'), L('Audio', 'Audio'), '*.mp3;*.m4a;*.aac;*.wav;*.flac;*.ogg;*.opus')
      if (p) st.anim.music = p
    } catch (e) {
      toast(errText(e), 'err')
    }
  }

  function combinedLabel(it: FileItem): [string, string] {
    const pos = usable.indexOf(it) + 1
    if (st.mode === 'anim') {
      const a = st.anim
      return [`${a.format.toUpperCase()} · ${a.seconds} ${L('dtk', 's')}/${L('gambar', 'picture')}`, pos ? `${L('Urutan', 'Order')} ${pos}` : '']
    }
    return [`${L('Kolase', 'Collage')} ${st.collage.format.toUpperCase()}`, pos ? `${L('Urutan', 'Order')} ${pos}` : '']
  }
</script>

<PageHeader title={L('Konversi Gambar', 'Image Converter')} subtitle={L('Ubah format, ukuran, dan kualitas banyak gambar sekaligus.', 'Change the format, size and quality of many images at once.')} />
<ToolBanner ids={needed} why={L('Dibutuhkan untuk pilihan AI / kompres PNG yang Anda nyalakan. Sekali unduh, lalu bekerja offline.', 'Needed for the AI / PNG compression options you turned on. Download once, then it works offline.')} />

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
      reorder={combined}
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
        {#if combined}
          {@const cl = combinedLabel(it)}
          <span class="r1">{cl[0]}</span>
          <span class="r2">{s === 'done' && task ? bytes(task.outSize) : s === 'running' ? L('Sedang diproses…', 'Processing…') : cl[1]}</span>
        {:else}
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
        {/if}
      {/snippet}
    </FileList>
  {/if}

  <aside class="card panel" aria-label={L('Pengaturan output', 'Output settings')}>
    <div class="scroll">
      <Segmented
        label={L('Mode', 'Mode')}
        bind:value={st.mode}
        options={[
          { value: 'convert', label: L('Konversi', 'Convert') },
          { value: 'anim', label: L('Animasi', 'Animation') },
          { value: 'collage', label: L('Kolase', 'Collage') },
        ]}
      />

      {#if st.mode === 'convert'}
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
        <div class="inline">
          <button class="btn grow" onclick={openCrop} disabled={!previewItem} title={previewItem ? `${L('Pratinjau', 'Preview')}: ${previewItem.name}` : L('Tambahkan gambar dulu', 'Add a picture first')}>
            <Icon name="crop" size={16} />{st.o.cropBox.w > 0 ? L('Ubah area crop', 'Change the crop area') : L('Crop manual…', 'Manual crop…')}
          </button>
          {#if st.o.cropBox.w > 0}
            <button class="btn icon" aria-label={L('Hapus crop', 'Remove crop')} title={L('Hapus crop', 'Remove crop')} onclick={() => (st.o.cropBox = { x: 0, y: 0, w: 0, h: 0 })}><Icon name="x" size={16} /></button>
          {/if}
        </div>
        {#if st.o.cropBox.w > 0}<p class="hint">{L('Area yang sama dipakai untuk semua gambar (dalam persen), jadi paling pas untuk foto berukuran sama.', 'The same area (in percent) is used for every picture, so it suits photos of the same size best.')}</p>{/if}
      </div>

      <div class="sec">
        <span class="label">{L('Perbesar dengan AI', 'Enlarge with AI')}</span>
        <Chips
          bind:value={st.o.aiUpscale}
          columns={4}
          small
          options={[
            { value: 0, label: L('Tidak', 'Off') },
            { value: 2, label: '2×' },
            { value: 3, label: '3×' },
            { value: 4, label: '4×' },
          ]}
        />
        {#if st.o.aiUpscale > 0}
          <Segmented
            label={L('Jenis gambar', 'Picture type')}
            bind:value={st.o.aiModel}
            options={[
              { value: 'photo', label: L('Foto', 'Photo') },
              { value: 'anime', label: L('Anime / ilustrasi', 'Anime / art') },
            ]}
          />
          <p class="hint">{L('Real-ESRGAN menambah detail saat memperbesar (memakai kartu grafis). Cocok untuk foto kecil/buram; hasilnya bisa diperkecil lagi dengan pengaturan Ukuran.', 'Real-ESRGAN adds detail while enlarging (uses the graphics card). Good for small/blurry photos; the Size settings can shrink it again.')}</p>
        {/if}
      </div>

      <div class="sec">
        <Switch bind:checked={st.o.removeBg} label={L('Hapus latar belakang', 'Remove background')} hint={L('Objek utama dipotong otomatis dengan AI, offline', 'The main subject is cut out automatically with AI, offline')} />
        {#if st.o.removeBg}
          <ModelPicker kind="bgremove" bind:value={st.o.bgModel} labels={bgModels} />
          <Switch bind:checked={st.o.bgMask} label={L('Simpan masker saja', 'Save the mask only')} hint={L('Gambar hitam-putih: putih = objek (untuk editor foto)', 'Black and white: white = subject (for photo editors)')} />
          {#if needsBg && !st.o.bgMask}<p class="hint">{L(`${st.o.format.toUpperCase()} tidak mendukung transparan: latar diganti warna di bagian Lainnya. Pilih PNG atau WEBP untuk latar transparan.`, `${st.o.format.toUpperCase()} has no transparency: the background becomes the colour under Other. Pick PNG or WEBP for a transparent background.`)}</p>{/if}
        {/if}
      </div>

      <WatermarkFields bind:wm={st.o.watermark} hint={L('Teks atau logo di setiap gambar', 'Text or a logo on every picture')} />

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
        {#if st.o.format === 'png'}
          <Select
            label={L('Kompres PNG', 'PNG compression')}
            bind:value={st.o.pngCompress}
            options={[
              { value: '', label: L('Kompres PNG: standar', 'PNG compression: standard') },
              { value: 'lossless', label: L('Kompres PNG: maksimal tanpa turun kualitas (oxipng)', 'PNG compression: maximum lossless (oxipng)') },
              { value: 'small', label: L('Kompres PNG: jauh lebih kecil (pngquant)', 'PNG compression: much smaller (pngquant)') },
            ]}
          />
          {#if st.o.pngCompress === 'small'}<p class="hint">{L('Warna dikurangi ke palet 256 warna — biasanya 60–80% lebih kecil dan hampir tak terlihat bedanya. Kurang cocok untuk foto dengan gradasi halus.', 'Colours are reduced to a 256-colour palette — usually 60–80% smaller and hardly visible. Less suited to photos with smooth gradients.')}</p>{/if}
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

      {:else if st.mode === 'anim'}
        <p class="hint">{L('Semua gambar di daftar jadi satu animasi atau video slideshow, sesuai urutan (atur dengan ↑↓).', 'Every picture in the list becomes one animation or slideshow video, in list order (use ↑↓).')}</p>
        <div class="sec">
          <span class="label">Format</span>
          <Chips
            bind:value={st.anim.format}
            columns={3}
            options={[
              { value: 'mp4', label: 'MP4', sub: L('video', 'video') },
              { value: 'gif', label: 'GIF', sub: L('animasi', 'animation') },
              { value: 'webp', label: 'WEBP', sub: L('animasi kecil', 'small animation') },
            ]}
          />
        </div>
        <div class="sec">
          <span class="label">{L('Durasi tiap gambar (detik)', 'Time per picture (seconds)')}</span>
          <Chips bind:value={st.anim.seconds} columns={5} small options={[0.3, 0.5, 1, 2, 3].map((v) => ({ value: v, label: v.toLocaleString(locale()) }))} />
          <span class="label">{L('Transisi', 'Transition')}</span>
          <Chips
            bind:value={st.anim.fade}
            columns={3}
            small
            options={[
              { value: 0, label: L('Langsung', 'Cut') },
              { value: 0.25, label: L('Pudar cepat', 'Quick fade') },
              { value: 0.5, label: L('Pudar', 'Fade') },
            ].map((x) => ({ ...x, disabled: x.value >= st.anim.seconds }))}
          />
        </div>
        <div class="sec">
          <span class="label">{L('Bingkai', 'Frame')}</span>
          <Chips
            bind:value={st.anim.ratio}
            columns={5}
            small
            options={[
              { value: '', label: L('Asli', 'Original'), title: L('Rasio gambar pertama', 'Ratio of the first picture') }, { value: '1:1', label: '1:1' }, { value: '16:9', label: '16:9' },
              { value: '9:16', label: '9:16' }, { value: '4:5', label: '4:5' },
            ]}
          />
          <Segmented
            label={L('Cara mengisi bingkai', 'How to fill the frame')}
            bind:value={st.anim.fit}
            options={[
              { value: 'blur', label: L('Latar blur', 'Blurred') },
              { value: 'contain', label: L('Bar', 'Bars') },
              { value: 'cover', label: L('Penuh', 'Fill') },
            ]}
          />
          <span class="label">{L('Sisi terpanjang', 'Longest side')}</span>
          <Chips bind:value={st.anim.size} columns={4} small options={[480, 720, 1080, 1920].map((v) => ({ value: v, label: `${v} px` }))} />
          {#if st.anim.format !== 'mp4' && st.anim.size > 720}<p class="hint">{L('GIF besar cepat membengkak ukurannya; 480–720 px biasanya cukup.', 'Big GIFs grow quickly; 480–720 px is usually enough.')}</p>{/if}
        </div>
        {#if st.anim.format === 'mp4'}
          <div class="sec">
            <span class="label">{L('Musik', 'Music')}</span>
            <div class="inline">
              <button class="btn grow" onclick={pickSlideMusic}><Icon name="music" size={16} />{st.anim.music ? L('Ganti musik', 'Change music') : L('Tambah musik…', 'Add music…')}</button>
              {#if st.anim.music}<button class="btn icon" aria-label={L('Hapus musik', 'Remove music')} onclick={() => (st.anim.music = '')}><Icon name="x" size={16} /></button>{/if}
            </div>
            {#if st.anim.music}<span class="hint ellipsis" title={st.anim.music}>{st.anim.music.split(/[\\/]/).pop()}</span>{/if}
          </div>
        {/if}
        {#if usable.length > 1}<p class="hint">{L('Durasi hasil', 'Result length')}: ±{Math.round(usable.length * st.anim.seconds * 10) / 10} {L('dtk', 's')}</p>{/if}
      {:else}
        <p class="hint">{L('Semua gambar di daftar disusun jadi satu gambar grid, sesuai urutan (atur dengan ↑↓).', 'Every picture in the list is laid out in one grid picture, in list order (use ↑↓).')}</p>
        <div class="sec">
          <span class="label">{L('Kolom', 'Columns')}</span>
          <Chips bind:value={st.collage.layout.cols} columns={5} small options={[0, 2, 3, 4, 5].map((v) => ({ value: v, label: v ? String(v) : L('Otomatis', 'Auto') }))} />
          <span class="label">{L('Bentuk kotak', 'Cell shape')}</span>
          <Chips bind:value={st.collage.layout.cell} columns={5} small options={['1:1', '4:5', '3:4', '16:9', '9:16'].map((v) => ({ value: v, label: v }))} />
          <Segmented
            label={L('Isi kotak', 'Cell fill')}
            bind:value={st.collage.layout.fit}
            options={[
              { value: 'cover', label: L('Penuh (dipotong)', 'Fill (cropped)') },
              { value: 'contain', label: L('Utuh', 'Whole') },
            ]}
          />
        </div>
        <div class="sec">
          <span class="label">{L('Jarak & sudut', 'Spacing & corners')}</span>
          <label class="t12" for="col-gap">{L('Jarak', 'Spacing')} · {st.collage.layout.gap} px</label>
          <input id="col-gap" type="range" min="0" max="80" step="2" bind:value={st.collage.layout.gap} />
          <label class="t12" for="col-rad">{L('Sudut membulat', 'Rounded corners')} · {st.collage.layout.radius} px</label>
          <input id="col-rad" type="range" min="0" max="80" step="2" bind:value={st.collage.layout.radius} />
          <div class="row-between">
            <span class="t13">{L('Warna latar', 'Background colour')}</span>
            <input class="color" type="color" aria-label={L('Warna latar', 'Background colour')} bind:value={st.collage.layout.background} />
          </div>
        </div>
        <div class="sec">
          <span class="label">{L('Hasil', 'Result')}</span>
          <Chips bind:value={st.collage.layout.width} columns={4} small options={[1080, 2000, 3000, 4000].map((v) => ({ value: v, label: `${v} px` }))} />
          <Chips bind:value={st.collage.format} columns={3} small options={[{ value: 'jpg', label: 'JPG' }, { value: 'png', label: 'PNG' }, { value: 'webp', label: 'WEBP' }]} />
        </div>
      {/if}

      <div class="sec">
        <span class="label">{L('Simpan ke', 'Save to')}</span>
        <OutputPicker kind="image" />
      </div>
    </div>

    <RunFooter
      kind="image"
      running={conv.running}
      busy={conv.starting}
      startLabel={st.mode === 'anim'
        ? `${L('Buat animasi', 'Make the animation')} (${usable.length})`
        : st.mode === 'collage'
          ? `${L('Buat kolase', 'Make the collage')} (${usable.length})`
          : conv.hasUnprocessed() || pending.length === 0 ? `${L('Mulai konversi', 'Start converting')}${pending.length ? ` (${pending.length})` : ''}` : `${L('Konversi ulang', 'Convert again')} (${pending.length})`}
      disabled={combined ? fewForCombined : pending.length === 0 || invalidResize || invalidIco || invalidWm || toolsMissing || bgNotReady}
      disabledHint={conv.items.length === 0
        ? L('Tambahkan gambar dulu untuk memulai.', 'Add images to get started.')
        : fewForCombined
          ? L('Tambahkan minimal 2 gambar.', 'Add at least 2 pictures.')
        : toolsMissing
          ? L('Pasang tool yang dibutuhkan dulu (lihat banner di atas).', 'Install the needed tools first (see the banner above).')
        : bgNotReady
          ? L('Unduh model hapus latar yang dipilih dulu.', 'Download the chosen background model first.')
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
  .grow {
    flex: 1;
  }
  .color {
    width: 40px;
    height: 32px;
    padding: 2px;
    border-radius: 8px;
    border: 1px solid var(--border);
    background: var(--surface-2);
    flex-shrink: 0;
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
