<script lang="ts">
  import { L } from '../../lib/i18n.svelte'
  import { onDestroy, onMount } from 'svelte'
  import EmptyDrop from '../../components/EmptyDrop.svelte'
  import OutputPicker from '../../components/OutputPicker.svelte'
  import RunFooter from '../../components/RunFooter.svelte'
  import Chips from '../../components/Chips.svelte'
  import Select from '../../components/Select.svelte'
  import Switch from '../../components/Switch.svelte'
  import Icon from '../../components/Icon.svelte'
  import StatusCell from '../../components/StatusCell.svelte'
  import CameraDialog from './CameraDialog.svelte'
  import { api, errText, imageUrl, pageUrl } from '../../lib/api'
  import { showDetail, toast } from '../../lib/stores/app.svelte'
  import { convFor, ensurePasswords, pdfDrop, pdfOpts, single, startBatch, startCombine } from '../../lib/stores/pdf.svelte'
  import { isActive, tasks } from '../../lib/stores/tasks.svelte'
  import { bytes } from '../../lib/format'
  import type { PdfTool } from '../../lib/pdfTools'
  import type { FileItem } from '../../lib/types'

  let { tool }: { tool: PdfTool } = $props()
  const conv = $derived(convFor(tool.id))
  const isPdf = $derived(tool.input === 'pdf')
  const o = pdfOpts

  onMount(() => {
    pdfDrop.fn = (paths) => conv.addPaths(paths)
  })
  onDestroy(() => (pdfDrop.fn = null))

  const task = $derived(single[tool.id] ? tasks[single[tool.id]] : undefined)
  const combine = $derived(tool.id !== 'img2pdf' || o.images.combine)
  const valid = $derived(conv.items.filter((it) => !it.error))
  const running = $derived(combine ? isActive(task) || conv.starting : conv.running)

  // ---- drag to reorder ----
  let dragFrom = $state(-1)
  let dragOver = $state(-1)
  function onDragStart(e: DragEvent, i: number) {
    dragFrom = i
    e.dataTransfer?.setData('text/plain', String(i))
    if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
  }
  function onDrop(e: DragEvent, i: number) {
    e.preventDefault()
    if (dragFrom >= 0) conv.move(dragFrom, i)
    dragFrom = dragOver = -1
  }
  function sortByName() {
    conv.items = [...conv.items].sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' }))
  }

  let scanning = $state(false)
  let camera = $state(false)
  async function scan() {
    scanning = true
    try {
      const it = await api.pdfScan()
      if (it) conv.merge([it])
    } catch (e) {
      toast(errText(e), 'err')
    } finally {
      scanning = false
    }
  }

  async function start() {
    if (combine) {
      let items = valid
      if (isPdf) {
        items = await ensurePasswords(conv, items)
        if (items.length !== valid.length) {
          toast(L('Semua PDF yang dikunci perlu password untuk digabung', 'Every locked PDF needs its password to be merged'), 'info')
          return
        }
      }
      startCombine(tool.id, conv, items)
    } else {
      startBatch('img2pdf', conv, conv.pending())
    }
  }

  const minItems = $derived(tool.id === 'merge' ? 2 : 1)
  const startLabel = $derived(
    tool.id === 'merge'
      ? `${L('Gabungkan', 'Merge')} ${valid.length} PDF`
      : combine
        ? `${L('Buat PDF', 'Create PDF')}${valid.length ? ` (${valid.length} ${L('halaman', valid.length === 1 ? 'page' : 'pages')})` : ''}`
        : `${L('Buat PDF', 'Create PDFs')} (${conv.pending().length})`,
  )
  const lastOutput = $derived(combine ? (task?.output ?? '') : conv.lastOutput())
</script>

<div class="body">
  <div class="left">
    {#if tool.id === 'scan'}
      <section class="card acquire">
        <button class="btn-accent big" onclick={scan} disabled={scanning}>
          <Icon name={scanning ? 'loader' : 'scan'} size={16} class={scanning ? 'spin' : ''} />{scanning ? L('Menunggu scanner…', 'Waiting for the scanner…') : L('Pindai dari scanner', 'Scan from scanner')}
        </button>
        <button class="btn big" onclick={() => (camera = true)}><Icon name="camera" size={16} />{L('Foto dengan kamera', 'Use the camera')}</button>
        <span class="hint">{L('Atau tambahkan foto dokumen yang sudah ada.', 'Or add photos of documents you already have.')}</span>
      </section>
    {/if}
    {#if conv.items.length === 0}
      <EmptyDrop
        {conv}
        title={isPdf ? L('Tarik & lepas PDF ke sini', 'Drag & drop PDFs here') : L('Tarik & lepas gambar ke sini', 'Drag & drop images here')}
        subtitle={isPdf ? L('Tambahkan minimal 2 PDF. Urutannya bisa diatur setelah ditambahkan.', 'Add at least 2 PDFs. You can change the order afterwards.') : L('Setiap gambar jadi satu halaman. Urutannya bisa diatur.', 'Every picture becomes a page. You can change the order.')}
        pickLabel={isPdf ? L('Pilih PDF', 'Choose PDFs') : L('Pilih gambar', 'Choose images')}
        formats={tool.formats}
        steps={[
          [L('Tambahkan file', 'Add files'), L('Tarik ke sini atau klik tombol', 'Drag them here or click the button')],
          [L('Atur urutan', 'Set the order'), L('Geser kartu ke posisi yang diinginkan', 'Drag the cards into place')],
          [L('Klik Mulai', 'Click Start'), L('Hasilnya satu file PDF', 'You get one PDF file')],
        ]}
      />
    {:else}
      <section class="card list">
        <div class="toolbar">
          <div class="count">
            <span class="n">{conv.items.length} file</span>
            <span class="size">{bytes(conv.totalSize)} total</span>
            {#if conv.adding}<span class="loading"><Icon name="loader" size={14} class="spin" /> {L('Membaca file…', 'Reading files…')}</span>{/if}
          </div>
          <button class="btn" onclick={() => conv.pickFiles()} disabled={conv.adding}><Icon name="plus" size={16} />{L('Tambah file', 'Add files')}</button>
          <button class="btn" onclick={sortByName} title={L('Urutkan menurut nama', 'Sort by name')}><Icon name="chevronDown" size={16} />{L('Urut A–Z', 'Sort A–Z')}</button>
          <button class="btn icon" aria-label={L('Kosongkan daftar', 'Clear list')} title={L('Kosongkan daftar', 'Clear list')} onclick={() => conv.clear()}><Icon name="trash" size={16} /></button>
        </div>
        <p class="tip">{L('Geser kartu untuk mengubah urutan. Hasil mengikuti urutan dari kiri ke kanan.', 'Drag the cards to reorder. The result follows the order from left to right.')}</p>
        <div class="cards">
          {#each conv.items as it, i (it.id)}
            {@const st = conv.state(it)}
            <div
              class="cardi"
              class:over={dragOver === i && dragFrom !== i}
              class:bad={!!it.error}
              draggable="true"
              role="listitem"
              ondragstart={(e) => onDragStart(e, i)}
              ondragover={(e) => {
                e.preventDefault()
                dragOver = i
              }}
              ondragleave={() => dragOver === i && (dragOver = -1)}
              ondrop={(e) => onDrop(e, i)}
              ondragend={() => (dragFrom = dragOver = -1)}
            >
              <span class="idx">{i + 1}</span>
              <div class="thumb">
                {#if it.error}
                  <Icon name="alert" size={28} />
                {:else if it.locked}
                  <Icon name="lock" size={28} />
                {:else if it.ext === 'pdf'}
                  <img src={pageUrl(it.path, 0, 220)} alt="" loading="lazy" draggable="false" />
                {:else}
                  <img src={imageUrl(it.path, 220)} alt="" loading="lazy" draggable="false" />
                {/if}
              </div>
              <span class="nm ellipsis" title={it.path}>{it.name}</span>
              <span class="mt ellipsis">{it.error || (it.pages ? `${it.pages} ${L('hal', it.pages === 1 ? 'page' : 'pages')} · ` : '') + bytes(it.size)}</span>
              {#if !combine && st !== 'ready'}<div class="st"><StatusCell state={st} task={conv.task(it)} /></div>{/if}
              <div class="acts">
                <button class="mini" aria-label={L('Geser ke kiri', 'Move left')} disabled={i === 0} onclick={() => conv.move(i, i - 1)}><Icon name="chevronLeft" size={14} /></button>
                <button class="mini" aria-label={L('Geser ke kanan', 'Move right')} disabled={i === conv.items.length - 1} onclick={() => conv.move(i, i + 1)}><Icon name="chevronRight" size={14} /></button>
                <button class="mini" aria-label={L(`Hapus ${it.name}`, `Remove ${it.name}`)} onclick={() => conv.remove(it)}><Icon name="x" size={14} /></button>
              </div>
            </div>
          {/each}
          <button class="cardi add" onclick={() => conv.pickFiles()} disabled={conv.adding}><Icon name="plus" size={26} /><span>{L('Tambah', 'Add')}</span></button>
        </div>
      </section>
    {/if}
  </div>

  <aside class="card panel" aria-label={L('Pengaturan', 'Settings')}>
    <div class="scroll">
      {#if tool.id === 'merge'}
        <div class="note"><Icon name="info" size={16} /><span>{L('Semua halaman digabung sesuai urutan kartu. PDF yang dikunci password akan ditanyakan passwordnya.', 'All pages are joined in card order. You will be asked for the password of locked PDFs.')}</span></div>
      {:else}
        {#if tool.id === 'img2pdf'}
          <Switch bind:checked={o.images.combine} label={L('Gabung jadi satu PDF', 'Combine into one PDF')} hint={o.images.combine ? L('Satu gambar = satu halaman', 'One picture = one page') : L('Setiap gambar jadi PDF sendiri', 'Every picture becomes its own PDF')} />
        {/if}
        <div class="sec">
          <span class="label">{L('Ukuran kertas', 'Paper size')}</span>
          <Select
            label={L('Ukuran kertas', 'Paper size')}
            bind:value={o.images.pageSize}
            options={[
              { value: 'a4', label: 'A4 (210 × 297 mm)' },
              { value: 'f4', label: 'F4 / Folio (215 × 330 mm)' },
              { value: 'letter', label: 'Letter (8.5 × 11 in)' },
              { value: 'legal', label: 'Legal (8.5 × 14 in)' },
              { value: 'a5', label: 'A5 (148 × 210 mm)' },
              { value: 'fit', label: L('Sesuai ukuran gambar', 'Same as the picture') },
            ]}
          />
        </div>
        {#if o.images.pageSize !== 'fit'}
          <div class="sec">
            <span class="label">{L('Arah halaman', 'Orientation')}</span>
            <Chips columns={3} small bind:value={o.images.orientation} options={[{ value: 'auto', label: L('Otomatis', 'Automatic') }, { value: 'portrait', label: L('Tegak', 'Portrait') }, { value: 'landscape', label: L('Mendatar', 'Landscape') }]} />
          </div>
        {/if}
        <div class="sec">
          <span class="label">Margin</span>
          <Chips columns={3} small bind:value={o.images.margin} options={[{ value: 'none', label: L('Tanpa', 'None') }, { value: 'small', label: L('Kecil', 'Small') }, { value: 'big', label: L('Besar', 'Big') }]} />
        </div>
        <div class="sec tight">
          <div class="row-between"><span class="label">{L('Kualitas gambar', 'Picture quality')}</span><span class="val">{o.images.quality}</span></div>
          <input type="range" min="40" max="100" bind:value={o.images.quality} aria-label={L('Kualitas gambar', 'Picture quality')} />
        </div>
      {/if}
      <div class="sec">
        <span class="label">{L('Simpan ke', 'Save to')}</span>
        <OutputPicker kind="pdf" />
      </div>
      {#if combine && task && !isActive(task)}
        <div class="res {task.status}">
          {#if task.status === 'done'}
            <Icon name="check" size={16} stroke={3} /><span class="ellipsis" title={task.output}>{task.output.split(/[\\/]/).pop()} · {bytes(task.outSize)}</span>
            <button class="link" onclick={() => api.revealFile(task.output)}>{L('Lihat', 'Show')}</button>
          {:else if task.status === 'failed'}
            <Icon name="alert" size={16} /><span class="ellipsis">{task.message}</span>
            <button class="link" onclick={() => showDetail(tool.name(), task.message, task.detail)}>{L('Detail', 'Details')}</button>
          {:else}
            <span>{task.message}</span>
          {/if}
        </div>
      {/if}
    </div>
    <RunFooter
      kind="pdf"
      {running}
      summary={combine ? !!task : conv.items.some((it) => !!conv.task(it))}
      busy={conv.starting}
      {startLabel}
      disabled={valid.length < minItems || conv.adding}
      disabledHint={valid.length < minItems ? (tool.id === 'merge' ? L('Tambahkan minimal 2 PDF.', 'Add at least 2 PDFs.') : L('Tambahkan gambar dulu.', 'Add pictures first.')) : ''}
      {lastOutput}
      onstart={start}
      oncancel={() => (combine && task ? api.cancelTask(task.id) : conv.cancelAll())}
    />
  </aside>
</div>

<CameraDialog bind:open={camera} onshot={(it: FileItem) => conv.merge([it])} />

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
  .acquire {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 14px 16px;
    flex-shrink: 0;
  }
  .big {
    height: 42px;
    padding: 0 16px;
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
  }
  .val {
    font-size: 13px;
    font-weight: 800;
  }
  input[type='range'] {
    width: 100%;
    margin: 0;
    accent-color: var(--accent);
  }
  .list {
    flex-grow: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .toolbar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 14px 16px;
    border-bottom: 1px solid var(--border);
  }
  .count {
    flex-grow: 1;
    display: flex;
    align-items: baseline;
    gap: 10px;
    min-width: 0;
  }
  .n {
    font-size: 15px;
    font-weight: 700;
  }
  .size {
    font-size: 13px;
    color: var(--text-3);
  }
  .loading {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--accent-text-2);
  }
  .tip {
    margin: 10px 16px 0;
    font-size: 12px;
    color: var(--text-3);
  }
  .cards {
    flex-grow: 1;
    min-height: 0;
    overflow-y: auto;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
    gap: 14px;
    padding: 14px 16px 16px;
    align-content: start;
  }
  .cardi {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 10px;
    border-radius: 12px;
    border: 1px solid var(--border);
    background: var(--surface-2);
    cursor: grab;
    min-width: 0;
  }
  .cardi.over {
    border-color: var(--accent);
    box-shadow: -4px 0 0 var(--accent);
  }
  .cardi.bad {
    border-color: var(--err-border);
  }
  .idx {
    position: absolute;
    top: 6px;
    left: 6px;
    z-index: 1;
    min-width: 22px;
    height: 22px;
    padding: 0 6px;
    border-radius: 11px;
    background: var(--accent);
    color: var(--accent-ink);
    font-size: 11px;
    font-weight: 800;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .thumb {
    height: 160px;
    border-radius: 8px;
    background: var(--surface-2);
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    color: var(--text-3);
  }
  .thumb img {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.4);
    background: #fff;
  }
  .nm {
    font-size: 12px;
    font-weight: 700;
    margin-top: 4px;
  }
  .mt {
    font-size: 11px;
    color: var(--text-3);
  }
  .bad .mt {
    color: var(--err);
  }
  .st {
    margin-top: 2px;
  }
  .acts {
    display: flex;
    justify-content: flex-end;
    gap: 2px;
  }
  .mini {
    width: 28px;
    height: 26px;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 0;
    border-radius: 6px;
    background: transparent;
    color: var(--text-3);
  }
  .mini:hover:not(:disabled) {
    background: var(--surface);
    color: var(--text);
  }
  .mini:disabled {
    opacity: 0.3;
  }
  .cardi.add {
    cursor: pointer;
    align-items: center;
    justify-content: center;
    gap: 8px;
    min-height: 230px;
    border-style: dashed;
    border-color: var(--border-strong);
    background: transparent;
    color: var(--accent);
    font-weight: 700;
    font-size: 13px;
  }
  .cardi.add:hover {
    background: var(--accent-tint);
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
  .res {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 12px;
    border-radius: 10px;
    font-size: 12px;
    font-weight: 600;
    min-width: 0;
  }
  .res.done {
    background: var(--ok-soft);
    color: var(--ok);
  }
  .res.failed {
    background: var(--err-soft);
    color: var(--err);
  }
  .res span {
    flex-grow: 1;
  }
  .link {
    padding: 0;
    border: 0;
    background: none;
    font-size: 12px;
    font-weight: 700;
    color: inherit;
    text-decoration: underline;
  }
</style>
