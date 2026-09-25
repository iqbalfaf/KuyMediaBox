<script lang="ts">
  import { L } from '../../lib/i18n.svelte'
  import Chips from '../../components/Chips.svelte'
  import Select from '../../components/Select.svelte'
  import Segmented from '../../components/Segmented.svelte'
  import Switch from '../../components/Switch.svelte'
  import PositionGrid from '../../components/PositionGrid.svelte'
  import Icon from '../../components/Icon.svelte'
  import DigiSignOptions from './DigiSignOptions.svelte'
  import { api, errText } from '../../lib/api'
  import { hasTool, nav, toast } from '../../lib/stores/app.svelte'
  import { initPdf, loadOcrLanguages, pdfEnv, pdfForm, pdfOpts } from '../../lib/stores/pdf.svelte'
  import type { PdfTool } from '../../lib/pdfTools'

  let { tool }: { tool: PdfTool } = $props()
  const o = pdfOpts

  $effect(() => {
    if (tool.id === 'ocr') loadOcrLanguages()
  })

  const env = $derived(pdfEnv.value)
  const officeFor = $derived.by(() => {
    if (!env) return ''
    const ms = tool.id === 'word2pdf' || tool.id === 'pdf2word' ? env.office.word : tool.id === 'excel2pdf' ? env.office.excel : env.office.powerpoint
    if (ms) return 'Microsoft Office'
    if (env.office.libreoffice) return 'LibreOffice'
    return ''
  })

  const confirm = pdfForm
  const toolFound = (id: string) => hasTool(id)
  let showPw = $state(false)
  const pwMismatch = $derived(o.protect.password !== '' && confirm.confirmPw !== '' && confirm.confirmPw !== o.protect.password)

  const numberFormats = $derived([
    { value: '{n}', label: '1, 2, 3' },
    { value: L('Halaman {n}', 'Page {n}'), label: L('Halaman 1', 'Page 1') },
    { value: '{n} / {total}', label: '1 / N' },
    { value: L('Halaman {n} dari {total}', 'Page {n} of {total}'), label: L('Halaman 1 dari N', 'Page 1 of N') },
    { value: '- {n} -', label: '- 1 -' },
  ])
  const presetFormat = $derived(numberFormats.some((f) => f.value === o.numbers.format))

  function num(e: Event, set: (v: number) => void, lo: number, hi: number) {
    const v = parseInt((e.currentTarget as HTMLInputElement).value.replace(/[^\d-]/g, ''), 10)
    if (!isNaN(v)) set(Math.max(lo, Math.min(hi, v)))
  }

  async function pickLogo() {
    try {
      const list = await api.pickFiles('pdfimage')
      if (list.length) o.watermark.image = list[0].path
    } catch (e) {
      toast(errText(e), 'err')
    }
  }
</script>

{#snippet pages(label: string, value: string, set: (v: string) => void)}
  <div class="sec tight">
    <label class="label" for="pg-{tool.id}">{label}</label>
    <input id="pg-{tool.id}" class="text-input" value={value} oninput={(e) => set((e.currentTarget as HTMLInputElement).value)} placeholder={L('Semua halaman — atau mis. 1-3, 5, 8-', 'Every page — or e.g. 1-3, 5, 8-')} />
  </div>
{/snippet}

{#snippet engineBox()}
  {#if !env}
    <p class="hint">{L('Memeriksa program Office…', 'Checking Office programs…')}</p>
  {:else if officeFor}
    <div class="note ok"><Icon name="check" size={14} stroke={3} /><span>{L('Diproses dengan', 'Converted with')} <b>{officeFor}</b> {L('yang terpasang di komputer ini.', 'installed on this computer.')}</span></div>
  {:else}
    <div class="note warn">
      <Icon name="alert" size={16} />
      <span>{L('Butuh Microsoft Office atau LibreOffice. LibreOffice gratis dan bisa diunduh dari Pengaturan.', 'Needs Microsoft Office or LibreOffice. LibreOffice is free and can be downloaded in Settings.')}</span>
    </div>
    <div class="row">
      <button class="btn" onclick={() => (nav.page = 'settings')}><Icon name="sliders" size={14} />{L('Buka Pengaturan', 'Open Settings')}</button>
      <button class="btn" onclick={() => initPdf(true)}><Icon name="refresh" size={14} />{L('Periksa ulang', 'Check again')}</button>
    </div>
  {/if}
{/snippet}

{#if tool.id === 'compress'}
  <div class="sec">
    <span class="label">{L('Tingkat kompresi', 'Compression level')}</span>
    <Chips
      columns={3}
      tall
      bind:value={o.compress.level}
      options={[
        { value: 'extreme', label: L('Ekstrem', 'Extreme'), sub: L('paling kecil', 'smallest') },
        { value: 'recommended', label: L('Disarankan', 'Recommended'), sub: L('seimbang', 'balanced') },
        { value: 'low', label: L('Ringan', 'Light'), sub: L('kualitas tinggi', 'high quality') },
      ]}
    />
    <p class="hint">
      {#if o.compress.level === 'extreme'}{L('Gambar diperkecil banyak. Cocok untuk dikirim lewat WhatsApp/email.', 'Pictures are reduced a lot. Good for sending by chat or email.')}
      {:else if o.compress.level === 'low'}{L('Gambar tetap tajam, penghematan lebih sedikit.', 'Pictures stay sharp; smaller savings.')}
      {:else}{L('Ukuran jauh lebih kecil dengan kualitas tetap baik.', 'Much smaller with good quality.')}{/if}
      {L('Teks dan grafik vektor tidak berubah.', 'Text and vector graphics are untouched.')}
    </p>
  </div>
  <Switch bind:checked={o.compress.gray} label={L('Ubah gambar jadi hitam-putih', 'Make pictures greyscale')} hint={L('Lebih kecil lagi, cocok untuk dokumen', 'Even smaller, fine for documents')} />
{:else if tool.id === 'repair'}
  <div class="note"><Icon name="info" size={16} /><span>{L('Struktur PDF dibangun ulang. Jika file sangat rusak, halaman yang masih terbaca diselamatkan.', 'The PDF structure is rebuilt. For badly damaged files, every page that can still be read is recovered.')}</span></div>
{:else if tool.id === 'ocr'}
  <div class="sec">
    <span class="label">{L('Bahasa dokumen', 'Document language')}</span>
    <Select
      label={L('Bahasa dokumen', 'Document language')}
      bind:value={o.ocr.lang}
      options={[{ value: '', label: L('Otomatis (bahasa Windows)', 'Automatic (Windows languages)') }, ...pdfEnv.langs.map((l) => ({ value: l.tag, label: l.name }))]}
    />
    <p class="hint">{L('Memakai OCR bawaan Windows. Huruf Latin (Indonesia, Inggris, dll.) terbaca dengan bahasa mana pun; bahasa lain bisa ditambah di Pengaturan Windows › Waktu & Bahasa.', 'Uses the OCR built into Windows. Latin text (English, Indonesian, …) reads with any language; add others in Windows Settings › Time & Language.')}</p>
  </div>
  {@render pages(L('Halaman', 'Pages'), o.ocr.pages, (v) => (o.ocr.pages = v))}
  <Switch bind:checked={o.ocr.skipText} label={L('Lewati halaman yang sudah berisi teks', 'Skip pages that already have text')} hint={L('Hanya halaman hasil scan yang diproses', 'Only scanned pages are processed')} />
{:else if tool.id === 'rotate'}
  <div class="sec">
    <span class="label">{L('Arah putar', 'Rotation')}</span>
    <Segmented
      label={L('Arah putar', 'Rotation')}
      bind:value={o.rotate}
      options={[
        { value: 90, label: L('90° kanan', '90° right'), icon: 'rotateCw' },
        { value: 180, label: '180°' },
        { value: 270, label: L('90° kiri', '90° left'), icon: 'rotateCcw' },
      ]}
    />
  </div>
  {@render pages(L('Halaman yang diputar', 'Pages to rotate'), o.pages, (v) => (o.pages = v))}
  <p class="hint">{L('Untuk memutar halaman satu per satu sambil melihat isinya, pakai Susun halaman.', 'To rotate pages one by one while seeing them, use Organize PDF.')}</p>
{:else if tool.id === 'protect'}
  <div class="sec tight">
    <label class="label" for="pw1">{L('Password', 'Password')}</label>
    <div class="pw">
      <input id="pw1" class="text-input" type={showPw ? 'text' : 'password'} bind:value={o.protect.password} autocomplete="new-password" placeholder={L('Password untuk membuka', 'Password to open')} />
      <button class="btn icon" aria-label={showPw ? L('Sembunyikan', 'Hide') : L('Tampilkan', 'Show')} onclick={() => (showPw = !showPw)}><Icon name={showPw ? 'eyeOff' : 'search'} size={16} /></button>
    </div>
    <input class="text-input" type={showPw ? 'text' : 'password'} bind:value={confirm.confirmPw} autocomplete="new-password" placeholder={L('Ulangi password', 'Repeat the password')} aria-label={L('Ulangi password', 'Repeat the password')} />
    {#if pwMismatch}<span class="bad">{L('Password tidak sama', "Passwords don't match")}</span>{/if}
    <p class="hint">{L('Simpan password baik-baik — tanpa password file tidak bisa dibuka.', "Keep the password safe — without it the file can't be opened.")}</p>
  </div>
  <div class="sec">
    <span class="label">{L('Izin setelah dibuka', 'Permissions once opened')}</span>
    <Switch bind:checked={o.protect.allowPrint} label={L('Boleh dicetak', 'Allow printing')} />
    <Switch bind:checked={o.protect.allowCopy} label={L('Boleh menyalin teks', 'Allow copying text')} />
    <Switch bind:checked={o.protect.allowEdit} label={L('Boleh diubah', 'Allow changes')} />
    <Switch bind:checked={o.protect.aes128} label={L('Mode kompatibel (AES-128)', 'Compatible mode (AES-128)')} hint={L('Untuk aplikasi PDF lama; bawaan AES-256', 'For old PDF readers; default is AES-256')} />
  </div>
{:else if tool.id === 'unlock'}
  <div class="note"><Icon name="info" size={16} /><span>{L('File yang hanya dibatasi (tidak bisa cetak/salin) langsung dibuka. File yang butuh password akan ditanyakan passwordnya saat mulai.', "Files that are only restricted (no print/copy) are unlocked right away. You'll be asked for the password of files that need one.")}</span></div>
{:else if tool.id === 'watermark'}
  <div class="sec">
    <Segmented
      label={L('Jenis watermark', 'Watermark type')}
      bind:value={o.watermark.type}
      options={[
        { value: 'text', label: L('Teks', 'Text'), icon: 'type' },
        { value: 'image', label: L('Gambar / logo', 'Image / logo'), icon: 'image' },
      ]}
    />
    {#if o.watermark.type === 'text'}
      <input class="text-input" bind:value={o.watermark.text} placeholder={L('Teks watermark', 'Watermark text')} aria-label={L('Teks watermark', 'Watermark text')} />
      <div class="row-between">
        <span class="t13">{L('Ukuran huruf', 'Font size')}</span>
        <div class="num"><input class="text-input" inputmode="numeric" value={o.watermark.size} oninput={(e) => num(e, (v) => (o.watermark.size = v), 6, 300)} aria-label={L('Ukuran huruf', 'Font size')} /><span>pt</span></div>
      </div>
      <div class="row-between">
        <span class="t13">{L('Warna', 'Colour')}</span>
        <div class="row">
          <label class="bold"><input type="checkbox" bind:checked={o.watermark.bold} /> {L('Tebal', 'Bold')}</label>
          <input class="color" type="color" bind:value={o.watermark.color} aria-label={L('Warna', 'Colour')} />
        </div>
      </div>
    {:else}
      <button class="btn pick" onclick={pickLogo}><Icon name="image" size={16} /><span class="ellipsis">{o.watermark.image ? o.watermark.image.split(/[\\/]/).pop() : L('Pilih gambar (PNG/JPG)…', 'Choose a picture (PNG/JPG)…')}</span></button>
      <div class="row-between"><span class="t13">{L('Lebar', 'Width')}</span><span class="val">{o.watermark.scale}% {L('halaman', 'of page')}</span></div>
      <input type="range" min="5" max="100" bind:value={o.watermark.scale} aria-label={L('Lebar gambar', 'Picture width')} />
    {/if}
  </div>
  <div class="sec tight">
    <div class="row-between"><span class="label">{L('Transparansi', 'Opacity')}</span><span class="val">{Math.round(o.watermark.opacity * 100)}%</span></div>
    <input type="range" min="0.05" max="1" step="0.05" bind:value={o.watermark.opacity} aria-label={L('Transparansi', 'Opacity')} />
  </div>
  <div class="sec">
    <span class="label">{L('Kemiringan', 'Angle')}</span>
    <Chips columns={4} small bind:value={o.watermark.angle} options={[{ value: 0, label: '0°' }, { value: 45, label: '45°' }, { value: 90, label: '90°' }, { value: -45, label: '-45°' }]} />
  </div>
  <div class="sec">
    <span class="label">{L('Posisi', 'Position')}</span>
    <div class="row">
      {#if o.watermark.position !== 'tile'}
        <PositionGrid label={L('Posisi watermark', 'Watermark position')} bind:value={o.watermark.position} />
      {/if}
      <div class="col">
        <Switch checked={o.watermark.position === 'tile'} onchange={(v) => (o.watermark.position = v ? 'tile' : 'mc')} label={L('Ulang di seluruh halaman', 'Repeat across the page')} />
      </div>
    </div>
  </div>
  {@render pages(L('Halaman', 'Pages'), o.watermark.pages, (v) => (o.watermark.pages = v))}
  <Switch bind:checked={o.watermark.under} label={L('Di bawah isi halaman', 'Below the page content')} hint={L('Teks dan gambar dokumen tetap di atas', 'Document text and pictures stay on top')} />
{:else if tool.id === 'numbers'}
  <div class="sec">
    <span class="label">{L('Posisi', 'Position')}</span>
    <PositionGrid label={L('Posisi nomor', 'Number position')} bind:value={o.numbers.position} rows={['t', 'b']} />
    <Switch bind:checked={o.numbers.mirror} label={L('Bolak-balik kiri/kanan', 'Mirror left/right')} hint={L('Untuk buku: halaman genap di sisi lain', 'For books: even pages on the other side')} />
  </div>
  <div class="sec">
    <span class="label">{L('Format', 'Format')}</span>
    <Select
      label={L('Format', 'Format')}
      value={presetFormat ? o.numbers.format : 'custom'}
      onchange={(v) => (o.numbers.format = v === 'custom' ? L('Hal. {n}', 'p. {n}') : v)}
      options={[...numberFormats, { value: 'custom', label: L('Tulis sendiri…', 'Custom…') }]}
    />
    {#if !presetFormat}
      <input class="text-input" bind:value={o.numbers.format} aria-label={L('Format sendiri', 'Custom format')} />
      <p class="hint">{L('{n} = nomor halaman, {total} = jumlah halaman', '{n} = page number, {total} = page count')}</p>
    {/if}
  </div>
  <div class="grid2">
    <div class="sec tight">
      <label class="label" for="nstart">{L('Mulai dari', 'Start at')}</label>
      <input id="nstart" class="text-input" inputmode="numeric" value={o.numbers.start} oninput={(e) => num(e, (v) => (o.numbers.start = v), 0, 99999)} />
    </div>
    <div class="sec tight">
      <label class="label" for="nsize">{L('Ukuran', 'Size')} (pt)</label>
      <input id="nsize" class="text-input" inputmode="numeric" value={o.numbers.size} oninput={(e) => num(e, (v) => (o.numbers.size = v), 5, 72)} />
    </div>
  </div>
  <div class="row-between">
    <span class="t13">{L('Warna', 'Colour')}</span>
    <div class="row">
      <label class="bold"><input type="checkbox" bind:checked={o.numbers.bold} /> {L('Tebal', 'Bold')}</label>
      <input class="color" type="color" bind:value={o.numbers.color} aria-label={L('Warna', 'Colour')} />
    </div>
  </div>
  <div class="sec tight">
    <span class="label">{L('Jarak dari tepi', 'Distance from the edge')}</span>
    <Chips columns={3} small bind:value={o.numbers.margin} options={[{ value: 18, label: L('Dekat', 'Close') }, { value: 28, label: L('Normal', 'Normal') }, { value: 48, label: L('Jauh', 'Far') }]} />
  </div>
  {@render pages(L('Halaman yang diberi nomor', 'Pages to number'), o.numbers.pages, (v) => (o.numbers.pages = v))}
  <p class="hint">{L('Contoh: isi "2-" agar sampul tidak diberi nomor.', 'Example: enter "2-" to leave the cover unnumbered.')}</p>
{:else if tool.id === 'pdf2img'}
  <div class="sec">
    <Segmented
      label={L('Cara', 'Mode')}
      bind:value={o.export.mode}
      options={[
        { value: 'pages', label: L('Halaman jadi gambar', 'Pages to images') },
        { value: 'extract', label: L('Ambil gambar di dalam', 'Pull out pictures') },
      ]}
    />
    <p class="hint">{o.export.mode === 'pages' ? L('Setiap halaman disimpan sebagai satu gambar.', 'Every page is saved as one picture.') : L('Foto dan gambar yang ada di dalam PDF disimpan dalam format aslinya.', 'Photos and pictures inside the PDF are saved in their original format.')}</p>
  </div>
  {#if o.export.mode === 'pages'}
    <div class="sec">
      <span class="label">{L('Format', 'Format')}</span>
      <Chips columns={2} bind:value={o.export.format} options={[{ value: 'jpg', label: 'JPG' }, { value: 'png', label: 'PNG' }]} />
    </div>
    <div class="sec">
      <span class="label">{L('Resolusi', 'Resolution')}</span>
      <Chips
        columns={3}
        tall
        bind:value={o.export.dpi}
        options={[
          { value: 72, label: '72 DPI', sub: L('layar', 'screen') },
          { value: 150, label: '150 DPI', sub: L('standar', 'standard') },
          { value: 300, label: '300 DPI', sub: L('cetak', 'print') },
        ]}
      />
    </div>
    {#if o.export.format === 'jpg'}
      <div class="sec tight">
        <div class="row-between"><span class="label">{L('Kualitas', 'Quality')}</span><span class="val">{o.export.quality}</span></div>
        <input type="range" min="30" max="100" bind:value={o.export.quality} aria-label={L('Kualitas', 'Quality')} />
      </div>
    {/if}
  {/if}
  {@render pages(L('Halaman', 'Pages'), o.export.pages, (v) => (o.export.pages = v))}
  <p class="hint">{L('Banyak gambar disimpan dalam satu folder bernama sama dengan PDF-nya.', 'Several pictures are saved in a folder named after the PDF.')}</p>
{:else if tool.id === 'pdf2word'}
  {#if env}
    <div class="note {env.office.word ? 'ok' : ''}">
      <Icon name={env.office.word ? 'check' : 'info'} size={16} />
      <span>
        {#if env.office.word}{L('Microsoft Word terpasang — tata letak, tabel, dan gambar ikut diubah.', 'Microsoft Word is installed — layout, tables and pictures are converted.')}
        {:else if env.office.libreoffice}{L('Diproses dengan LibreOffice; tata letak dipertahankan dengan kotak teks.', 'Converted with LibreOffice; layout is kept with text boxes.')}
        {:else}{L('Tanpa Microsoft Word: teks, paragraf, dan judul diambil (tata letak dan gambar tidak ikut). Pasang Word atau LibreOffice untuk hasil lebih lengkap.', 'Without Microsoft Word: text, paragraphs and headings are converted (not the layout or pictures). Install Word or LibreOffice for a fuller result.')}{/if}
      </span>
    </div>
  {/if}
  <p class="hint">{L('PDF hasil scan perlu di-OCR dulu agar teksnya terbaca.', 'Scanned PDFs need OCR first so the text can be read.')}</p>
{:else if tool.id === 'pdf2ppt'}
  <div class="note"><Icon name="info" size={16} /><span>{L('Setiap halaman jadi satu slide berisi gambar halaman yang tajam — tampilan persis sama dengan PDF.', 'Each page becomes a slide with a sharp picture of the page — it looks exactly like the PDF.')}</span></div>
{:else if tool.id === 'pdf2excel'}
  <div class="note"><Icon name="info" size={16} /><span>{L('Setiap halaman jadi satu sheet. Teks disusun ke baris dan kolom mengikuti posisinya, jadi tabel rapi paling akurat.', 'Every page becomes a sheet. Text is laid out in rows and columns by position, so neat tables convert best.')}</span></div>
  <p class="hint">{L('PDF hasil scan perlu di-OCR dulu.', 'Scanned PDFs need OCR first.')}</p>
{:else if tool.id === 'pdfa'}
  <div class="note"><Icon name="info" size={16} /><span>{L('Hasil PDF/A-2b: metadata arsip, profil warna sRGB, tanpa enkripsi/JavaScript/lampiran. Halaman dengan huruf yang tidak tertanam diubah jadi gambar agar tampil sama di mana pun.', 'Produces PDF/A-2b: archive metadata, sRGB colour profile, no encryption/JavaScript/attachments. Pages with fonts that are not embedded become images so they look the same everywhere.')}</span></div>
{:else if tool.id === 'pdfacheck'}
  <div class="note"><Icon name="shieldCheck" size={16} /><span>{L('veraPDF memeriksa setiap aturan ISO 19005 (PDF/A). Hasilnya "lolos" atau daftar aturan yang dilanggar (klik "Lihat detail").', 'veraPDF checks every ISO 19005 (PDF/A) rule. The result is "compliant" or the list of broken rules (click "View details").')}</span></div>
  <div class="sec">
    <span class="label">{L('Periksa sebagai', 'Check as')}</span>
    <Select
      label={L('Periksa sebagai', 'Check as')}
      bind:value={o.pdfaCheck.flavour}
      options={[
        { value: '0', label: L('Sesuai klaim file (otomatis)', 'What the file claims (automatic)') },
        { value: '1b', label: 'PDF/A-1b' },
        { value: '2b', label: 'PDF/A-2b' },
        { value: '2u', label: 'PDF/A-2u' },
        { value: '3b', label: 'PDF/A-3b' },
        { value: '1a', label: 'PDF/A-1a' },
        { value: '2a', label: 'PDF/A-2a' },
      ]}
    />
  </div>
  {#if !toolFound('verapdf')}
    <div class="note warn"><Icon name="alert" size={16} /><span>{L('veraPDF belum terpasang. Pasang di Pengaturan › Tools pendukung (Java ikut dipasang bila belum ada).', 'veraPDF is not installed. Install it in Settings › Helper tools (Java comes with it when missing).')} <button class="inl" onclick={() => (nav.page = 'settings')}>{L('Buka Pengaturan', 'Open Settings')}</button></span></div>
  {/if}
{:else if tool.id === 'digisign'}
  <DigiSignOptions />
{:else if tool.id === 'word2pdf' || tool.id === 'excel2pdf' || tool.id === 'ppt2pdf'}
  {@render engineBox()}
  <p class="hint">{L('File asli tidak diubah. Dokumen yang dikunci password harus dibuka proteksinya dulu.', 'The original file is not changed. Password-protected documents must be unprotected first.')}</p>
{:else if tool.id === 'html'}
  <div class="sec">
    <span class="label">{L('Lebar layar', 'Screen width')}</span>
    <Chips
      columns={4}
      tall
      bind:value={o.html.width}
      options={[
        { value: 390, label: 'HP', sub: '390' },
        { value: 768, label: 'Tablet', sub: '768' },
        { value: 1280, label: 'Laptop', sub: '1280' },
        { value: 1920, label: 'Desktop', sub: '1920' },
      ]}
    />
  </div>
  <Switch bind:checked={o.html.onePage} label={L('Satu halaman panjang', 'One long page')} hint={L('Seluruh halaman web tanpa terpotong', 'The whole web page without breaks')} />
  {#if !o.html.onePage}
    <div class="grid2">
      <div class="sec tight">
        <span class="label">{L('Kertas', 'Paper')}</span>
        <Select label={L('Kertas', 'Paper')} bind:value={o.html.pageSize} options={[{ value: 'a4', label: 'A4' }, { value: 'f4', label: 'F4 / Folio' }, { value: 'letter', label: 'Letter' }, { value: 'legal', label: 'Legal' }, { value: 'a3', label: 'A3' }]} />
      </div>
      <div class="sec tight">
        <span class="label">{L('Arah', 'Orientation')}</span>
        <Select label={L('Arah', 'Orientation')} bind:value={o.html.orientation} options={[{ value: 'portrait', label: L('Tegak', 'Portrait') }, { value: 'landscape', label: L('Mendatar', 'Landscape') }]} />
      </div>
    </div>
  {/if}
  <div class="sec tight">
    <span class="label">{L('Margin', 'Margin')}</span>
    <Chips columns={3} small bind:value={o.html.margin} options={[{ value: 'none', label: L('Tanpa', 'None') }, { value: 'small', label: L('Kecil', 'Small') }, { value: 'normal', label: L('Normal', 'Normal') }]} />
  </div>
  <Switch bind:checked={o.html.background} label={L('Cetak warna & gambar latar', 'Print background colours & images')} />
  {#if env && !env.browser}
    <div class="note warn"><Icon name="alert" size={16} /><span>{L('Microsoft Edge atau Google Chrome dibutuhkan.', 'Microsoft Edge or Google Chrome is required.')}</span></div>
  {/if}
{/if}

<style>
  .sec {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .sec.tight {
    gap: 8px;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .row-between {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
  }
  .col {
    display: flex;
    flex-direction: column;
    flex-grow: 1;
  }
  .grid2 {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px;
  }
  .t13 {
    font-size: 13px;
    font-weight: 600;
  }
  .val {
    font-size: 13px;
    font-weight: 800;
  }
  .bad {
    font-size: 12px;
    font-weight: 600;
    color: var(--err);
  }
  input[type='range'] {
    width: 100%;
    margin: 0;
    accent-color: var(--accent);
  }
  .num {
    position: relative;
    width: 96px;
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
  .color {
    width: 40px;
    height: 32px;
    padding: 2px;
    border-radius: 8px;
    border: 1px solid var(--border);
    background: var(--surface-2);
    cursor: pointer;
  }
  .bold {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    font-weight: 600;
    color: var(--text-2);
  }
  .bold input {
    accent-color: var(--accent);
  }
  .pw {
    display: flex;
    gap: 8px;
  }
  .pw .btn.icon {
    width: 40px;
    height: 40px;
  }
  .pick {
    justify-content: flex-start;
    height: 40px;
    min-width: 0;
  }
  .note {
    display: flex;
    gap: 10px;
    align-items: flex-start;
    padding: 12px;
    border-radius: 10px;
    background: var(--info-soft);
    color: var(--info-text);
    font-size: 12px;
    line-height: 1.5;
  }
  .note :global(svg) {
    flex-shrink: 0;
    margin-top: 1px;
  }
  .note.ok {
    background: var(--ok-soft);
    color: var(--note-text);
  }
  .note.warn {
    background: var(--note-bg);
    color: var(--warn);
  }
  .inl {
    padding: 0;
    border: 0;
    background: none;
    font: inherit;
    font-weight: 700;
    color: var(--accent-text-2);
    text-decoration: underline;
  }
</style>
