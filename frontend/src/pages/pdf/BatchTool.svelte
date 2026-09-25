<script lang="ts">
  import { L } from '../../lib/i18n.svelte'
  import { onDestroy, onMount } from 'svelte'
  import FileList from '../../components/FileList.svelte'
  import EmptyDrop from '../../components/EmptyDrop.svelte'
  import OutputPicker from '../../components/OutputPicker.svelte'
  import RunFooter from '../../components/RunFooter.svelte'
  import Icon from '../../components/Icon.svelte'
  import ToolOptions from './ToolOptions.svelte'
  import { imageUrl, pageUrl } from '../../lib/api'
  import { hasTool, toast } from '../../lib/stores/app.svelte'
  import { convFor, ensurePasswords, pdfDrop, pdfEnv, pdfForm, pdfOpts, startBatch } from '../../lib/stores/pdf.svelte'
  import { askPassword } from '../../lib/stores/prompt.svelte'
  import { bytes, tile } from '../../lib/format'
  import type { PdfTool } from '../../lib/pdfTools'
  import type { FileItem } from '../../lib/types'

  let { tool }: { tool: PdfTool } = $props()
  const conv = $derived(convFor(tool.id))
  const isWeb = $derived(tool.pattern === 'web')
  const isPdf = $derived(tool.input === 'pdf')

  onMount(() => {
    pdfDrop.fn = (paths) => conv.addPaths(paths)
  })
  onDestroy(() => (pdfDrop.fn = null))

  // ---- web addresses (HTML → PDF) ----
  let urls = $state('')
  let seq = 0
  function blank(path: string): FileItem {
    return {
      id: `u${Date.now()}-${seq++}`, path, name: path, ext: 'url', size: 0, width: 0, height: 0, duration: 0, format: '', videoCodec: '', fps: 0,
      audioCodec: '', sampleRate: 0, bitsPerSample: 0, channels: 0, hasVideo: false, hasAudio: false, hasCover: false, pages: 0, encrypted: false, locked: false,
      subCodec: '', subFile: '', tags: null, error: '',
    }
  }
  function addUrls() {
    const list = urls.split(/[\s,]+/).map((s) => s.trim()).filter((s) => /^(https?:\/\/)?[\w-]+(\.[\w-]+)+/i.test(s))
    if (list.length === 0) {
      toast(L('Tempel alamat web, mis. https://contoh.com', 'Paste a web address, e.g. https://example.com'), 'info')
      return
    }
    conv.merge(list.map(blank))
    urls = ''
  }

  const meta = (it: FileItem) => {
    if (it.ext === 'url') return L('Halaman web', 'Web page')
    const parts: string[] = []
    if (it.pages) parts.push(`${it.pages} ${L('hal', it.pages === 1 ? 'page' : 'pages')}`)
    if (it.locked) parts.push(L('🔒 dikunci password', '🔒 password protected'))
    else if (it.encrypted) parts.push(L('dibatasi', 'restricted'))
    if (!isPdf && it.width) parts.push(`${it.width}×${it.height}`)
    parts.push(it.ext.toUpperCase())
    if (it.size) parts.push(bytes(it.size))
    return parts.join(' · ')
  }

  const target = $derived.by(() => {
    const o = pdfOpts
    switch (tool.id) {
      case 'compress':
        return `PDF · ${{ extreme: L('Ekstrem', 'Extreme'), recommended: L('Disarankan', 'Recommended'), low: L('Ringan', 'Light') }[o.compress.level]}`
      case 'pdf2img':
        return o.export.mode === 'extract' ? L('Gambar asli', 'Original pictures') : `${o.export.format.toUpperCase()} · ${o.export.dpi} DPI`
      case 'pdf2word':
        return 'DOCX'
      case 'pdf2ppt':
        return 'PPTX'
      case 'pdf2excel':
        return 'XLSX'
      case 'pdfa':
        return 'PDF/A-2b'
      case 'rotate':
        return `PDF · ${o.rotate}°`
      case 'protect':
        return L('PDF terkunci', 'Locked PDF')
      case 'unlock':
        return L('PDF terbuka', 'Unlocked PDF')
      case 'digisign':
        return L('PDF bertanda tangan digital', 'Digitally signed PDF')
      case 'pdfacheck':
        return o.pdfaCheck.flavour === '0' ? L('Cek PDF/A', 'PDF/A check') : `PDF/A-${o.pdfaCheck.flavour}`
    }
    return 'PDF'
  })

  async function setPassword(it: FileItem) {
    const pw = await askPassword(it.name)
    if (pw) conv.passwords[it.id] = pw
  }

  const pending = $derived(conv.pending())
  const env = $derived(pdfEnv.value)
  const blocked = $derived.by(() => {
    const o = pdfOpts
    if (tool.id === 'protect') {
      if (!o.protect.password) return L('Isi password dulu.', 'Enter a password first.')
      if (pdfForm.confirmPw !== o.protect.password) return L('Ulangi password yang sama.', 'Repeat the same password.')
    }
    if (tool.id === 'watermark') {
      if (o.watermark.type === 'text' && !o.watermark.text.trim()) return L('Isi teks watermark.', 'Enter the watermark text.')
      if (o.watermark.type === 'image' && !o.watermark.image) return L('Pilih gambar watermark.', 'Choose the watermark picture.')
    }
    if (['word2pdf', 'excel2pdf', 'ppt2pdf'].includes(tool.id) && env) {
      const ms = tool.id === 'word2pdf' ? env.office.word : tool.id === 'excel2pdf' ? env.office.excel : env.office.powerpoint
      if (!ms && !env.office.libreoffice) return L('Butuh Microsoft Office atau LibreOffice.', 'Needs Microsoft Office or LibreOffice.')
    }
    if (isWeb && env && !env.browser) return L('Butuh Microsoft Edge atau Chrome.', 'Needs Microsoft Edge or Chrome.')
    if (tool.id === 'digisign') {
      if (!o.digisign.certFile) return L('Pilih file sertifikat.', 'Choose a certificate file.')
      if (!pdfForm.certPassword) return L('Isi password sertifikat.', 'Enter the certificate password.')
    }
    if (tool.id === 'pdfacheck' && !hasTool('verapdf')) return L('Pasang veraPDF dulu di Pengaturan.', 'Install veraPDF in Settings first.')
    return ''
  })

  async function start() {
    let items = pending
    if (isPdf) items = await ensurePasswords(conv, items)
    if (tool.id === 'unlock') {
      const open = items.filter((it) => !it.encrypted)
      if (open.length === items.length && items.length > 0) {
        toast(L('File ini tidak dikunci', "These files aren't locked"), 'info')
        return
      }
    }
    if (items.length) startBatch(tool.id, conv, items)
  }

  const verb = $derived(
    {
      compress: L('Kompres', 'Compress'), repair: L('Perbaiki', 'Repair'), ocr: L('Jalankan OCR', 'Run OCR'), rotate: L('Putar', 'Rotate'),
      protect: L('Kunci', 'Protect'), unlock: L('Buka kunci', 'Unlock'), watermark: L('Tambah watermark', 'Add watermark'), numbers: L('Beri nomor', 'Add numbers'),
      pdfa: L('Ubah ke PDF/A', 'Convert to PDF/A'), digisign: L('Tandatangani', 'Sign'), pdfacheck: L('Validasi', 'Validate'),
    }[tool.id] ?? L('Mulai konversi', 'Start converting'),
  )
</script>

<div class="body">
  <div class="left">
    {#if isWeb}
      <section class="card urlbox" aria-label={L('Alamat web', 'Web addresses')}>
        <Icon name="globe" />
        <textarea
          bind:value={urls}
          rows="1"
          placeholder={L('Tempel alamat web di sini (bisa beberapa, satu per baris)…', 'Paste web addresses here (several allowed, one per line)…')}
          onkeydown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault()
              addUrls()
            }
          }}
        ></textarea>
        <button class="btn-accent" onclick={addUrls} disabled={!urls.trim()}><Icon name="plus" size={14} stroke={2.5} />{L('Tambah', 'Add')}</button>
      </section>
    {/if}
    {#if conv.items.length === 0}
      <EmptyDrop
        {conv}
        title={isWeb ? L('Atau tarik file HTML ke sini', 'Or drag HTML files here') : L(`Tarik & lepas file ${tool.formats[0]} ke sini`, `Drag & drop ${tool.formats[0]} files here`)}
        subtitle={L('Bisa banyak file sekaligus, atau satu folder penuh. Semua diproses di komputer ini.', 'Many files at once, or a whole folder. Everything is processed on this computer.')}
        pickLabel={isWeb ? L('Pilih file HTML', 'Choose HTML files') : L('Pilih file', 'Choose files')}
        formats={tool.formats}
        steps={[
          [isWeb ? L('Tambahkan alamat web', 'Add web addresses') : L('Tambahkan file', 'Add files'), L('Tarik ke sini atau klik tombol', 'Drag them here or click the button')],
          [L('Atur pilihan', 'Pick the options'), L('Di panel sebelah kanan', 'In the panel on the right')],
          [L('Klik Mulai', 'Click Start'), L('File asli tidak akan diubah', 'Your original files stay untouched')],
        ]}
      />
    {:else}
      <FileList {conv} noun={isWeb ? L('halaman', 'pages') : 'file'} dropText={L('Tarik & lepas file atau folder di sini', 'Drag & drop files or folders here')} formats={tool.formats.join(' · ')} {meta}>
        {#snippet thumb(it)}
          {#if it.ext === 'pdf' && !it.locked}
            <img class="pthumb" src={pageUrl(it.path, 0, 80)} alt="" loading="lazy" />
          {:else if tool.input === 'pdfimage'}
            <img class="pthumb" src={imageUrl(it.path, 80)} alt="" loading="lazy" />
          {:else}
            {@const t = tile(it.name)}
            <div class="kmb-tile" style="background: {t.bg}; color: {t.fg}"><Icon name={it.locked ? 'lock' : tool.icon} /></div>
          {/if}
        {/snippet}
        {#snippet result(it)}
          {@const task = conv.task(it)}
          {@const s = conv.state(it)}
          <span class="r1">{target}</span>
          {#if s === 'done' && task}
            {#if tool.id === 'compress' && it.size}
              {@const diff = Math.round(((task.outSize - it.size) / it.size) * 100)}
              <span class="r2">{bytes(task.outSize)} <span class={diff <= 0 ? 'saved' : 'grew'}>{diff <= 0 ? `−${Math.abs(diff)}%` : `+${diff}%`}</span></span>
            {:else}
              <span class="r2" title={task.message}>{task.message && !task.message.endsWith('…') ? task.message : bytes(task.outSize)}</span>
            {/if}
          {:else if s === 'running'}
            <span class="r2">{task?.message || L('Sedang diproses…', 'Processing…')}</span>
          {:else if it.locked && !conv.passwords[it.id]}
            <button class="pwlink" onclick={() => setPassword(it)}><Icon name="lock" size={12} />{L('Isi password', 'Enter password')}</button>
          {:else if it.locked}
            <span class="r2">{L('Password diisi', 'Password entered')}</span>
          {:else}
            <span class="r2">{it.size ? bytes(it.size) : ''}</span>
          {/if}
        {/snippet}
      </FileList>
    {/if}
  </div>

  <aside class="card panel" aria-label={L('Pengaturan', 'Settings')}>
    <div class="scroll">
      <ToolOptions {tool} />
      <div class="sec">
        <span class="label">{L('Simpan ke', 'Save to')}</span>
        <OutputPicker kind="pdf" />
      </div>
    </div>
    <RunFooter
      kind="pdf"
      running={conv.running}
      summary={conv.items.some((it) => !!conv.task(it))}
      busy={conv.starting}
      startLabel={`${verb}${pending.length ? ` (${pending.length})` : ''}`}
      disabled={pending.length === 0 || !!blocked}
      disabledHint={conv.items.length === 0 ? (isWeb ? L('Tambahkan alamat web dulu.', 'Add a web address first.') : L('Tambahkan file dulu untuk memulai.', 'Add files to get started.')) : blocked}
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
  .left {
    flex-grow: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 12px;
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
  .urlbox {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 12px 10px 16px;
    color: var(--accent);
    flex-shrink: 0;
  }
  .urlbox textarea {
    flex-grow: 1;
    min-height: 36px;
    max-height: 120px;
    resize: vertical;
    border: 0;
    background: transparent;
    font-size: 14px;
    padding: 8px 0;
    outline: none;
    color: var(--text);
  }
  .pthumb {
    width: 40px;
    height: 40px;
    flex-shrink: 0;
    border-radius: 6px;
    object-fit: contain;
    background: #fff;
  }
  .pwlink {
    align-self: flex-start;
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 0;
    border: 0;
    background: none;
    font-size: 12px;
    font-weight: 700;
    color: var(--warn);
  }
  .pwlink:hover {
    text-decoration: underline;
  }
</style>
