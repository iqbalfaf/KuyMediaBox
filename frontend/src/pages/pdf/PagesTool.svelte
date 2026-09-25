<script lang="ts">
  import { L } from '../../lib/i18n.svelte'
  import { onDestroy, onMount } from 'svelte'
  import OutputPicker from '../../components/OutputPicker.svelte'
  import RunFooter from '../../components/RunFooter.svelte'
  import Segmented from '../../components/Segmented.svelte'
  import Switch from '../../components/Switch.svelte'
  import Icon from '../../components/Icon.svelte'
  import OpenDocView from './OpenDoc.svelte'
  import { api, pageUrl } from '../../lib/api'
  import { showDetail, toast } from '../../lib/stores/app.svelte'
  import {
    cardsOf, newKey, openDoc, pageTools, pdfDrop, pdfOpts, pickDoc, rangesText, single, startEdit, startSingle, type Card, type OpenDoc,
  } from '../../lib/stores/pdf.svelte'
  import { isActive, tasks } from '../../lib/stores/tasks.svelte'
  import { bytes, parseRange } from '../../lib/format'
  import type { PdfTool } from '../../lib/pdfTools'

  let { tool }: { tool: PdfTool } = $props()
  const o = pdfOpts
  if (!pageTools[tool.id]) pageTools[tool.id] = { sources: [], cards: [] }
  const ws = $derived(pageTools[tool.id])
  const doc = $derived(ws.sources[0] as OpenDoc | undefined)
  const n = $derived(doc?.info.pages.length ?? 0)
  const organize = $derived(tool.id === 'organize')

  let opening = $state(false)
  let size = $state(150)

  async function load(d: OpenDoc | null, append = false) {
    if (!d) return
    const w = pageTools[tool.id]
    if (append && w.sources.length) {
      w.sources.push(d)
      w.cards.push(...cardsOf(d, w.sources.length - 1))
      return
    }
    w.sources = [d]
    w.cards = cardsOf(d, 0)
    if (tool.id === 'remove' || tool.id === 'extract') o.pages = ''
    if (tool.id === 'split') o.split.ranges = ''
  }
  async function pick(append = false) {
    opening = true
    try {
      await load(await pickDoc(), append)
    } finally {
      opening = false
    }
  }
  onMount(() => {
    pdfDrop.fn = async (paths) => {
      const pdfs = paths.filter((p) => p.toLowerCase().endsWith('.pdf'))
      if (!pdfs.length) return toast(L('Tarik file PDF', 'Drop a PDF file'), 'info')
      opening = true
      try {
        for (const [i, p] of pdfs.entries()) {
          await load(await openDoc(p), organize && (i > 0 || ws.sources.length > 0))
          if (!organize) break
        }
      } finally {
        opening = false
      }
    }
  })
  onDestroy(() => (pdfDrop.fn = null))

  // ---- selection (remove / extract) ----
  const selected = $derived.by(() => {
    if (tool.id !== 'remove' && tool.id !== 'extract') return new Set<number>()
    return parseRange(o.pages, n) ?? new Set<number>()
  })
  function toggle(p: number) {
    const s = new Set(selected)
    if (s.has(p)) s.delete(p)
    else s.add(p)
    o.pages = rangesText([...s])
  }

  // ---- split groups ----
  const groups = $derived.by(() => {
    const g = new Map<number, number>()
    if (tool.id !== 'split' || !n) return g
    if (o.split.mode === 'every') {
      const every = Math.max(1, o.split.every || 1)
      for (let p = 1; p <= n; p++) g.set(p, Math.floor((p - 1) / every))
    } else if (o.split.mode === 'all') {
      for (let p = 1; p <= n; p++) g.set(p, p - 1)
    } else {
      o.split.ranges.split(/[,;\n]+/).forEach((part, i) => {
        const r = parseRange(part, n)
        r?.forEach((p) => !g.has(p) && g.set(p, i))
      })
    }
    return g
  })
  const groupCount = $derived(new Set(groups.values()).size)
  function cutAfter(p: number) {
    // Rebuild contiguous ranges with a cut toggled after page p.
    const cuts = new Set<number>()
    for (let q = 1; q < n; q++) if (groups.get(q) !== groups.get(q + 1)) cuts.add(q)
    if (cuts.has(p)) cuts.delete(p)
    else cuts.add(p)
    const parts: string[] = []
    let start = 1
    for (let q = 1; q <= n; q++) {
      if (cuts.has(q) || q === n) {
        parts.push(start === q ? `${q}` : `${start}-${q}`)
        start = q + 1
      }
    }
    o.split.mode = 'ranges'
    o.split.ranges = parts.join(', ')
  }
  const tones = ['#ff7a45', '#5b9dff', '#5be3a0', '#c792ea', '#ffc77a', '#ff8a8a', '#8ec3ff', '#e0e070']

  // ---- organize ----
  let dragFrom = $state(-1)
  let dragOver = $state(-1)
  function move(from: number, to: number) {
    const list = [...ws.cards]
    const [c] = list.splice(from, 1)
    list.splice(to, 0, c)
    pageTools[tool.id].cards = list
  }
  function rotate(c: Card, d: number) {
    c.rotate = (((c.rotate + d) % 360) + 360) % 360
  }
  function duplicate(i: number) {
    const c = ws.cards[i]
    pageTools[tool.id].cards.splice(i + 1, 0, { ...c, key: newKey() })
  }
  function removeCard(i: number) {
    pageTools[tool.id].cards.splice(i, 1)
  }
  function addBlank(i: number) {
    const ref = ws.cards[i] ?? ws.cards[ws.cards.length - 1]
    pageTools[tool.id].cards.splice(i + 1, 0, { key: newKey(), src: -1, page: 0, rotate: 0, w: ref?.w ?? 595, h: ref?.h ?? 842 })
  }
  function reset() {
    const w = pageTools[tool.id]
    w.cards = w.sources.flatMap((d, i) => cardsOf(d, i))
  }

  // ---- run ----
  const task = $derived(single[tool.id] ? tasks[single[tool.id]] : undefined)
  let starting = $state(false)
  const blocked = $derived.by(() => {
    if (!doc) return L('Buka PDF dulu.', 'Open a PDF first.')
    if (tool.id === 'remove') {
      if (selected.size === 0) return L('Klik halaman yang mau dihapus.', 'Click the pages to remove.')
      if (selected.size >= n) return L('Tidak bisa menghapus semua halaman.', "Can't remove every page.")
    }
    if (tool.id === 'extract' && selected.size === 0) return L('Klik halaman yang mau diambil.', 'Click the pages to extract.')
    if (tool.id === 'split' && o.split.mode === 'ranges' && groupCount === 0) return L('Klik di antara halaman untuk memotong, atau isi rentang.', 'Click between pages to cut, or enter ranges.')
    if (organize && ws.cards.length === 0) return L('Dokumen harus punya halaman.', 'The document needs pages.')
    return ''
  })
  async function start() {
    if (!doc) return
    starting = true
    try {
      if (organize) {
        await startEdit({
          tool: 'organize',
          sources: ws.sources.map((s) => ({ path: s.path, password: s.password })),
          pages: ws.cards.map((c) => ({ src: c.src, page: c.page, rotate: c.rotate, w: c.w, h: c.h })),
        })
      } else {
        await startSingle(tool.id, doc)
      }
    } finally {
      starting = false
    }
  }

  const startLabel = $derived(
    {
      split: `${L('Pisahkan jadi', 'Split into')} ${groupCount} ${L('file', groupCount === 1 ? 'file' : 'files')}`,
      remove: `${L('Hapus', 'Remove')} ${selected.size} ${L('halaman', selected.size === 1 ? 'page' : 'pages')}`,
      extract: `${L('Ambil', 'Extract')} ${selected.size} ${L('halaman', selected.size === 1 ? 'page' : 'pages')}`,
      organize: L('Simpan susunan', 'Save the new order'),
    }[tool.id] ?? L('Mulai', 'Start'),
  )
  const hint = $derived(
    {
      split: L('Klik garis di antara halaman untuk memotong. Warna yang sama = satu file.', 'Click the line between pages to cut. Same colour = one file.'),
      remove: L('Klik halaman untuk menandai yang dihapus.', 'Click pages to mark them for removal.'),
      extract: L('Klik halaman yang mau diambil.', 'Click the pages you want.'),
      organize: L('Geser kartu untuk mengubah urutan. Arahkan kursor ke kartu untuk memutar, menggandakan, atau menghapus.', 'Drag cards to reorder. Hover a card to rotate, duplicate or delete it.'),
    }[tool.id] ?? '',
  )
</script>

<div class="body">
  {#if !doc}
    <OpenDocView
      title={L('Buka PDF yang mau diatur', 'Open the PDF to work on')}
      subtitle={L('Tarik file PDF ke sini atau klik tombol. Halamannya akan tampil sebagai gambar kecil.', 'Drag a PDF here or click the button. Its pages appear as thumbnails.')}
      busy={opening}
      onpick={() => pick()}
    />
  {:else}
    <section class="card list">
      <div class="toolbar">
        <div class="count">
          <span class="n ellipsis" title={doc.path}>{ws.sources.length > 1 ? `${ws.sources.length} PDF` : doc.name}</span>
          <span class="size">{ws.cards.length} {L('halaman', 'pages')}</span>
          {#if opening}<span class="loading"><Icon name="loader" size={14} class="spin" /></span>{/if}
        </div>
        {#if organize}
          <button class="btn" onclick={() => pick(true)} disabled={opening}><Icon name="plus" size={16} />{L('Tambah PDF', 'Add PDF')}</button>
          <button class="btn" onclick={() => addBlank(ws.cards.length - 1)}><Icon name="filePlus" size={16} />{L('Halaman kosong', 'Blank page')}</button>
          <button class="btn icon" title={L('Putar semua ke kanan', 'Rotate all right')} aria-label={L('Putar semua ke kanan', 'Rotate all right')} onclick={() => ws.cards.forEach((c) => rotate(c, 90))}><Icon name="rotateCw" size={16} /></button>
          <button class="btn icon" title={L('Kembalikan seperti semula', 'Reset')} aria-label={L('Kembalikan seperti semula', 'Reset')} onclick={reset}><Icon name="undo" size={16} /></button>
        {:else}
          <button class="btn" onclick={() => pick()} disabled={opening}><Icon name="folder" size={16} />{L('Ganti PDF', 'Change PDF')}</button>
        {/if}
        <input class="zoom" type="range" min="100" max="280" bind:value={size} aria-label={L('Ukuran gambar halaman', 'Thumbnail size')} />
      </div>
      <p class="tip">{hint}</p>
      <div class="cards" style="--w: {size}px">
        {#each ws.cards as c, i (c.key)}
          {@const src = ws.sources[c.src]}
          {@const isSel = selected.has(c.page)}
          {@const g = groups.get(c.page) ?? -1}
          <div class="slot">
            <div
              class="pcard"
              class:sel={isSel}
              class:del={tool.id === 'remove' && isSel}
              class:over={dragOver === i && dragFrom !== i}
              style={tool.id === 'split' && g >= 0 ? `--g: ${tones[g % tones.length]}` : ''}
              class:grouped={tool.id === 'split' && g >= 0}
              draggable={organize}
              role={organize ? 'listitem' : 'button'}
              tabindex={organize ? undefined : 0}
              onclick={() => !organize && (tool.id === 'remove' || tool.id === 'extract') && toggle(c.page)}
              onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && !organize && toggle(c.page)}
              ondragstart={(e) => {
                dragFrom = i
                e.dataTransfer?.setData('text/plain', String(i))
              }}
              ondragover={(e) => {
                if (!organize) return
                e.preventDefault()
                dragOver = i
              }}
              ondrop={(e) => {
                e.preventDefault()
                if (dragFrom >= 0) move(dragFrom, i)
                dragFrom = dragOver = -1
              }}
              ondragend={() => (dragFrom = dragOver = -1)}
            >
              <div class="pimg">
                {#if c.src < 0}
                  <div class="blank" style="aspect-ratio: {c.w} / {c.h}; transform: rotate({c.rotate}deg)"></div>
                {:else}
                  <img src={pageUrl(src.path, c.page - 1, size * 1.5)} alt="{L('Halaman', 'Page')} {c.page}" loading="lazy" draggable="false" style="transform: rotate({c.rotate}deg);{c.rotate % 180 ? ' max-width: calc(var(--w) * 1.3); max-height: calc(var(--w) - 16px)' : ''}" />
                {/if}
                {#if tool.id === 'remove' && isSel}<span class="mark x"><Icon name="x" size={18} stroke={3} /></span>{/if}
                {#if tool.id === 'extract' && isSel}<span class="mark ok"><Icon name="check" size={18} stroke={3} /></span>{/if}
                {#if organize}
                  <div class="hover">
                    <button aria-label={L('Putar ke kiri', 'Rotate left')} onclick={() => rotate(c, -90)}><Icon name="rotateCcw" size={14} /></button>
                    <button aria-label={L('Putar ke kanan', 'Rotate right')} onclick={() => rotate(c, 90)}><Icon name="rotateCw" size={14} /></button>
                    <button aria-label={L('Gandakan', 'Duplicate')} onclick={() => duplicate(i)}><Icon name="copy" size={14} /></button>
                    <button aria-label={L('Sisipkan halaman kosong setelahnya', 'Insert a blank page after')} onclick={() => addBlank(i)}><Icon name="filePlus" size={14} /></button>
                    <button aria-label={L('Hapus halaman', 'Delete page')} onclick={() => removeCard(i)}><Icon name="trash" size={14} /></button>
                  </div>
                {/if}
              </div>
              <span class="pn">{c.src < 0 ? L('Kosong', 'Blank') : ws.sources.length > 1 ? `${c.src + 1}·${c.page}` : c.page}</span>
            </div>
            {#if tool.id === 'split' && i < ws.cards.length - 1}
              {@const cut = groups.get(c.page) !== groups.get(c.page + 1)}
              <button class="cut" class:on={cut} onclick={() => cutAfter(c.page)} aria-label={cut ? L('Gabungkan kembali', 'Join again') : L('Potong di sini', 'Cut here')} title={cut ? L('Gabungkan kembali', 'Join again') : L('Potong di sini', 'Cut here')}>
                <Icon name="scissors" size={14} />
              </button>
            {/if}
          </div>
        {/each}
      </div>
    </section>
  {/if}

  <aside class="card panel" aria-label={L('Pengaturan', 'Settings')}>
    <div class="scroll">
      {#if tool.id === 'split'}
        <div class="sec">
          <span class="label">{L('Cara memisah', 'How to split')}</span>
          <Segmented
            label={L('Cara memisah', 'How to split')}
            bind:value={o.split.mode}
            options={[
              { value: 'ranges', label: L('Rentang', 'Ranges') },
              { value: 'every', label: L('Tiap N hal', 'Every N') },
              { value: 'all', label: L('Per halaman', 'Each page') },
            ]}
          />
        </div>
        {#if o.split.mode === 'ranges'}
          <div class="sec tight">
            <label class="label" for="ranges">{L('Rentang (satu file per rentang)', 'Ranges (one file each)')}</label>
            <input id="ranges" class="text-input" bind:value={o.split.ranges} placeholder={L('mis. 1-3, 4-10, 11-', 'e.g. 1-3, 4-10, 11-')} />
          </div>
        {:else if o.split.mode === 'every'}
          <div class="sec tight">
            <label class="label" for="every">{L('Halaman per file', 'Pages per file')}</label>
            <input id="every" class="text-input" type="number" min="1" max={Math.max(1, n)} bind:value={o.split.every} />
          </div>
        {/if}
        <p class="hint">{L(`Hasil: ${groupCount} file PDF dalam satu folder.`, `Result: ${groupCount} PDF files in one folder.`)}</p>
      {:else if tool.id === 'remove' || tool.id === 'extract'}
        <div class="sec tight">
          <label class="label" for="sel">{tool.id === 'remove' ? L('Halaman yang dihapus', 'Pages to remove') : L('Halaman yang diambil', 'Pages to extract')}</label>
          <input id="sel" class="text-input" bind:value={o.pages} placeholder={L('Klik halaman, atau ketik mis. 2, 5-7', 'Click pages, or type e.g. 2, 5-7')} />
          <div class="row">
            <button class="btn small" onclick={() => (o.pages = n ? `1-${n}` : '')}>{L('Pilih semua', 'Select all')}</button>
            <button class="btn small" onclick={() => (o.pages = rangesText(Array.from({ length: n }, (_, i) => i + 1).filter((p) => p % 2 === 0)))}>{L('Genap', 'Even')}</button>
            <button class="btn small" onclick={() => (o.pages = rangesText(Array.from({ length: n }, (_, i) => i + 1).filter((p) => p % 2 === 1)))}>{L('Ganjil', 'Odd')}</button>
            <button class="btn small" onclick={() => (o.pages = '')}>{L('Kosongkan', 'Clear')}</button>
          </div>
        </div>
        <p class="hint">{L(`${selected.size} dari ${n} halaman dipilih.`, `${selected.size} of ${n} pages selected.`)}</p>
        {#if tool.id === 'extract'}
          <Switch bind:checked={o.separate} label={L('Setiap halaman jadi file sendiri', 'Each page as its own file')} />
        {/if}
      {:else}
        <div class="note"><Icon name="info" size={16} /><span>{L('Bisa menggabungkan halaman dari beberapa PDF: klik Tambah PDF, lalu susun halamannya.', 'You can combine pages from several PDFs: click Add PDF, then arrange the pages.')}</span></div>
      {/if}
      <div class="sec">
        <span class="label">{L('Simpan ke', 'Save to')}</span>
        <OutputPicker kind="pdf" />
      </div>
      {#if task && !isActive(task)}
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
      running={isActive(task) || starting}
      summary={!!task}
      busy={starting}
      {startLabel}
      disabled={!!blocked}
      disabledHint={blocked}
      lastOutput={task?.output ?? ''}
      onstart={start}
      oncancel={() => task && api.cancelTask(task.id)}
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
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .btn.small {
    height: 30px;
    padding: 0 10px;
    font-size: 12px;
  }
  .list {
    flex-grow: 1;
    min-width: 0;
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
    min-width: 44px;
  }
  .size {
    font-size: 13px;
    color: var(--text-3);
    white-space: nowrap;
  }
  .loading {
    color: var(--accent);
    align-self: center;
  }
  .zoom {
    width: 72px;
    accent-color: var(--accent);
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
    display: flex;
    flex-wrap: wrap;
    align-content: flex-start;
    gap: 14px 0;
    padding: 14px 16px 20px;
  }
  .slot {
    display: flex;
    align-items: center;
  }
  .pcard {
    position: relative;
    width: var(--w);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    padding: 8px;
    margin: 0 7px;
    border-radius: 12px;
    border: 2px solid transparent;
    cursor: pointer;
    user-select: none;
  }
  .pcard[draggable='true'] {
    cursor: grab;
  }
  .pcard:hover {
    background: var(--surface-2);
  }
  .pcard.sel {
    border-color: var(--ok);
    background: var(--ok-soft);
  }
  .pcard.del {
    border-color: var(--err);
    background: var(--err-soft);
  }
  .pcard.grouped {
    border-color: var(--g);
    border-style: solid;
  }
  .pcard.over {
    box-shadow: -6px 0 0 var(--accent);
  }
  .pimg {
    position: relative;
    width: 100%;
    height: calc(var(--w) * 1.3);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .pimg img,
  .blank {
    max-width: 100%;
    max-height: 100%;
    background: #fff;
    box-shadow: 0 2px 10px var(--shadow);
    transition: transform 0.2s;
  }
  .blank {
    width: 70%;
  }
  .del .pimg img {
    opacity: 0.45;
  }
  .mark {
    position: absolute;
    top: 6px;
    right: 6px;
    width: 30px;
    height: 30px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    color: #fff;
  }
  .mark.x {
    background: #d9383a;
  }
  .mark.ok {
    background: #1f9d62;
  }
  .hover {
    position: absolute;
    bottom: 6px;
    left: 50%;
    transform: translateX(-50%);
    display: none;
    gap: 2px;
    padding: 3px;
    border-radius: 9px;
    background: rgba(18, 21, 27, 0.92);
    border: 1px solid var(--border-strong);
  }
  .pcard:hover .hover {
    display: flex;
  }
  .hover button {
    width: 28px;
    height: 28px;
    border: 0;
    border-radius: 6px;
    background: transparent;
    color: var(--text-2);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .hover button:hover {
    background: var(--surface-2);
    color: var(--text);
  }
  .pn {
    font-size: 12px;
    font-weight: 700;
    color: var(--text-2);
  }
  .cut {
    width: 26px;
    height: 60%;
    min-height: 80px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: #4a515e;
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
  }
  .cut::before {
    content: '';
    position: absolute;
    top: 0;
    bottom: 0;
    left: 50%;
    border-left: 2px dashed #353b47;
  }
  .cut :global(svg) {
    position: relative;
    background: var(--surface);
    padding: 2px;
    border-radius: 4px;
  }
  .cut:hover {
    color: var(--accent);
  }
  .cut.on {
    color: var(--accent);
  }
  .cut.on::before {
    border-left: 2px solid var(--accent);
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
