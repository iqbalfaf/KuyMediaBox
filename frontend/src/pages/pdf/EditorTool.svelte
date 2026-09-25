<script lang="ts">
  import { L, locale } from '../../lib/i18n.svelte'
  import { onDestroy, onMount } from 'svelte'
  import OutputPicker from '../../components/OutputPicker.svelte'
  import RunFooter from '../../components/RunFooter.svelte'
  import Segmented from '../../components/Segmented.svelte'
  import Switch from '../../components/Switch.svelte'
  import Icon from '../../components/Icon.svelte'
  import OpenDocView from './OpenDoc.svelte'
  import SignatureDialog from './SignatureDialog.svelte'
  import { api, errText, pageUrl } from '../../lib/api'
  import { showDetail, toast } from '../../lib/stores/app.svelte'
  import { editors, openDoc, pdfDrop, pdfOpts, pickDoc, signatures, single, startEdit, type OpenDoc } from '../../lib/stores/pdf.svelte'
  import { isActive, tasks } from '../../lib/stores/tasks.svelte'
  import { bytes } from '../../lib/format'
  import type { PdfTool } from '../../lib/pdfTools'
  import type { EditItem } from '../../lib/types'

  let { tool }: { tool: PdfTool } = $props()
  if (!editors[tool.id]) editors[tool.id] = { doc: null, items: [], page: 0 }
  const ed = $derived(editors[tool.id])
  const doc = $derived(ed.doc as OpenDoc | null)
  const n = $derived(doc?.info.pages.length ?? 0)
  const size = $derived(doc?.info.pages[ed.page] ?? { w: 595, h: 842 })
  const o = pdfOpts

  // ---- opening ----
  let opening = $state(false)
  async function load(d: OpenDoc | null) {
    if (!d) return
    editors[tool.id] = { doc: d, items: [], page: 0 }
    history = []
    selected = -1
    if (tool.id === 'crop') {
      resetCrop()
      o.crop.mode = 'box'
    }
  }
  async function pick() {
    opening = true
    try {
      await load(await pickDoc())
    } finally {
      opening = false
    }
  }
  onMount(() => {
    pdfDrop.fn = async (paths) => {
      const p = paths.find((x) => x.toLowerCase().endsWith('.pdf'))
      if (!p) return toast(L('Tarik file PDF', 'Drop a PDF file'), 'info')
      opening = true
      try {
        await load(await openDoc(p))
      } finally {
        opening = false
      }
    }
  })
  onDestroy(() => (pdfDrop.fn = null))

  // ---- view ----
  let area = $state<HTMLDivElement>()
  let areaW = $state(800)
  let areaH = $state(600)
  let zoom = $state(1)
  const fit = $derived(Math.max(0.1, Math.min((areaW - 48) / size.w, (areaH - 48) / size.h)))
  const scale = $derived(fit * zoom)
  const renderW = $derived(Math.min(2400, Math.round(size.w * scale * (window.devicePixelRatio || 1))))
  function go(p: number) {
    ed.page = Math.max(0, Math.min(n - 1, p))
    selected = -1
  }

  // ---- tools & style ----
  type Mode = 'select' | 'text' | 'rect' | 'ellipse' | 'line' | 'ink' | 'whiteout' | 'image' | 'sign' | 'date' | 'box'
  let mode = $state<Mode>(tool.id === 'redact' ? 'box' : tool.id === 'sign' ? 'sign' : 'select')
  const style = $state({ color: '#1a1a1a', fill: '', stroke: 2, size: 14, bold: false, opacity: 1 })
  let selected = $state(-1)
  let pendingImage = $state<{ url: string; w: number; h: number } | null>(null)
  let sigIndex = $state(0)
  let sigDialog = $state(false)

  const toolsFor: Record<string, { m: Mode; icon: string; label: () => string }[]> = {
    edit: [
      { m: 'select', icon: 'pointer', label: () => L('Pilih & geser', 'Select & move') },
      { m: 'text', icon: 'type', label: () => L('Teks', 'Text') },
      { m: 'rect', icon: 'square', label: () => L('Kotak', 'Box') },
      { m: 'ellipse', icon: 'circle', label: () => L('Lingkaran', 'Circle') },
      { m: 'line', icon: 'line', label: () => L('Garis', 'Line') },
      { m: 'ink', icon: 'pencil', label: () => L('Coretan bebas', 'Freehand') },
      { m: 'whiteout', icon: 'fileMinus', label: () => L('Tutup putih', 'White-out') },
      { m: 'image', icon: 'image', label: () => L('Gambar', 'Picture') },
    ],
    sign: [
      { m: 'select', icon: 'pointer', label: () => L('Pilih & geser', 'Select & move') },
      { m: 'sign', icon: 'signature', label: () => L('Tanda tangan', 'Signature') },
      { m: 'text', icon: 'type', label: () => L('Nama / teks', 'Name / text') },
      { m: 'date', icon: 'calendar', label: () => L('Tanggal', 'Date') },
    ],
    redact: [
      { m: 'select', icon: 'pointer', label: () => L('Pilih & geser', 'Select & move') },
      { m: 'box', icon: 'square', label: () => L('Tandai area', 'Mark area') },
    ],
    crop: [],
  }

  // ---- history ----
  let history: string[] = []
  function snapshot() {
    history.push(JSON.stringify($state.snapshot(ed.items)))
    if (history.length > 60) history.shift()
  }
  function undo() {
    const s = history.pop()
    if (s !== undefined) {
      ed.items = JSON.parse(s)
      selected = -1
    }
  }

  function blankItem(kind: EditItem['kind'], x: number, y: number): EditItem {
    return {
      kind, page: ed.page + 1, x, y, w: 0, h: 0, x2: x, y2: y, points: [], text: '', size: style.size, bold: style.bold, align: 'left',
      color: style.color, fill: style.fill, stroke: style.stroke, opacity: style.opacity, angle: 0, imageUrl: '',
    }
  }

  // ---- geometry ----
  let svg = $state<SVGSVGElement>()
  function pt(e: PointerEvent): { x: number; y: number } {
    const m = svg!.getScreenCTM()!.inverse()
    const p = new DOMPoint(e.clientX, e.clientY).matrixTransform(m)
    return { x: Math.max(0, Math.min(size.w, p.x)), y: Math.max(0, Math.min(size.h, p.y)) }
  }
  let measureCtx: CanvasRenderingContext2D | null = null
  function textW(t: string, sz: number, bold: boolean): number {
    measureCtx ??= document.createElement('canvas').getContext('2d')
    measureCtx!.font = `${bold ? 'bold ' : ''}${sz}px Helvetica, Arial, sans-serif`
    return Math.max(...t.split('\n').map((l) => measureCtx!.measureText(l).width), sz * 0.5)
  }
  /** Bounding box of an item (points). */
  function bbox(it: EditItem) {
    switch (it.kind) {
      case 'text': {
        const lines = it.text.split('\n').length
        return { x: it.x, y: it.y - it.size * 0.8, w: textW(it.text, it.size, it.bold), h: it.size * (1.2 * (lines - 1) + 1) }
      }
      case 'line':
        return { x: Math.min(it.x, it.x2), y: Math.min(it.y, it.y2), w: Math.abs(it.x2 - it.x), h: Math.abs(it.y2 - it.y) }
      case 'ink': {
        const xs = it.points.map((p) => p[0])
        const ys = it.points.map((p) => p[1])
        return { x: Math.min(...xs), y: Math.min(...ys), w: Math.max(...xs) - Math.min(...xs), h: Math.max(...ys) - Math.min(...ys) }
      }
    }
    return { x: it.x, y: it.y, w: it.w, h: it.h }
  }

  // ---- pointer handling ----
  type Drag = { kind: 'create' | 'move' | 'resize' | 'crop-move' | 'crop-resize'; start: { x: number; y: number }; orig?: EditItem; corner?: string; cropOrig?: typeof o.crop.box }
  let drag: Drag | null = null
  let draft = $state<EditItem | null>(null)

  const pageItems = $derived(ed.items.map((it, i) => ({ it, i })).filter((x) => x.it.page === ed.page + 1))

  function today(): string {
    return new Intl.DateTimeFormat(locale(), { day: 'numeric', month: 'long', year: 'numeric' }).format(new Date())
  }

  function bgDown(e: PointerEvent) {
    if (e.button !== 0 || !svg) return
    const p = pt(e)
    if (tool.id === 'crop') return
    if (mode === 'select') {
      selected = -1
      return
    }
    svg.setPointerCapture(e.pointerId)
    if (mode === 'text' || mode === 'date') {
      snapshot()
      const it = blankItem('text', p.x, p.y + style.size * 0.35)
      it.text = mode === 'date' ? today() : L('Teks', 'Text')
      ed.items.push(it)
      selected = ed.items.length - 1
      mode = 'select'
      setTimeout(() => textArea?.select(), 30)
      return
    }
    if (mode === 'sign' || mode === 'image') {
      const src = mode === 'sign' ? signatures[sigIndex] : pendingImage
      if (!src) {
        if (mode === 'sign') sigDialog = true
        else imageInput?.click()
        return
      }
      snapshot()
      const w = mode === 'sign' ? 150 : Math.min(220, size.w * 0.4)
      const h = (w * src.h) / src.w
      const it = blankItem('image', p.x - w / 2, p.y - h / 2)
      Object.assign(it, { w, h, imageUrl: src.url })
      ed.items.push(it)
      selected = ed.items.length - 1
      mode = 'select'
      return
    }
    const kind = mode === 'whiteout' || mode === 'box' ? 'rect' : (mode as EditItem['kind'])
    const it = blankItem(kind, p.x, p.y)
    if (mode === 'whiteout') Object.assign(it, { fill: '#ffffff', color: '', stroke: 0, opacity: 1 })
    if (mode === 'box') Object.assign(it, { fill: '#000000', color: '', stroke: 0, opacity: 1 })
    if (mode === 'ink') it.points = [[p.x, p.y]]
    draft = it
    drag = { kind: 'create', start: p }
  }

  function itemDown(e: PointerEvent, i: number) {
    if (e.button !== 0 || tool.id === 'crop' || (mode !== 'select' && mode !== 'box')) return
    if (mode === 'box' && ed.items[i].kind !== 'rect') return
    e.stopPropagation()
    selected = i
    snapshot()
    svg!.setPointerCapture(e.pointerId)
    drag = { kind: 'move', start: pt(e), orig: structuredClone($state.snapshot(ed.items[i])) as EditItem }
  }

  function handleDown(e: PointerEvent, corner: string) {
    e.stopPropagation()
    if (tool.id === 'crop') {
      svg!.setPointerCapture(e.pointerId)
      drag = { kind: 'crop-resize', start: pt(e), corner, cropOrig: { ...o.crop.box } }
      return
    }
    snapshot()
    svg!.setPointerCapture(e.pointerId)
    drag = { kind: 'resize', start: pt(e), orig: structuredClone($state.snapshot(ed.items[selected])) as EditItem, corner }
  }

  function cropDown(e: PointerEvent) {
    e.stopPropagation()
    svg!.setPointerCapture(e.pointerId)
    drag = { kind: 'crop-move', start: pt(e), cropOrig: { ...o.crop.box } }
  }

  function onMove(e: PointerEvent) {
    if (!drag) return
    const p = pt(e)
    const dx = p.x - drag.start.x
    const dy = p.y - drag.start.y
    if (drag.kind === 'create' && draft) {
      if (draft.kind === 'ink') {
        draft.points = [...draft.points, [p.x, p.y]]
      } else if (draft.kind === 'line') {
        draft.x2 = p.x
        draft.y2 = p.y
      } else {
        draft.x = Math.min(p.x, drag.start.x)
        draft.y = Math.min(p.y, drag.start.y)
        draft.w = Math.abs(dx)
        draft.h = Math.abs(dy)
      }
      return
    }
    if (drag.kind === 'move' && drag.orig && selected >= 0) {
      const it = ed.items[selected]
      const a = drag.orig
      it.x = a.x + dx
      it.y = a.y + dy
      it.x2 = a.x2 + dx
      it.y2 = a.y2 + dy
      if (a.points.length) it.points = a.points.map(([x, y]) => [x + dx, y + dy] as [number, number])
      return
    }
    if (drag.kind === 'resize' && drag.orig && selected >= 0) {
      const it = ed.items[selected]
      const a = drag.orig
      if (it.kind === 'text') {
        const b = bbox(a)
        it.size = Math.max(4, Math.min(200, (a.size * (b.w + dx)) / Math.max(1, b.w)))
        return
      }
      if (it.kind === 'image' && !e.shiftKey) {
        const w = Math.max(8, a.w + dx)
        it.w = w
        it.h = (w * a.h) / a.w
        return
      }
      it.w = Math.max(4, a.w + dx)
      it.h = Math.max(4, a.h + dy)
      return
    }
    if (drag.cropOrig) {
      const c = drag.cropOrig
      const ux = dx / size.w
      const uy = dy / size.h
      const b = o.crop.box
      if (drag.kind === 'crop-move') {
        b.x = Math.max(0, Math.min(1 - c.w, c.x + ux))
        b.y = Math.max(0, Math.min(1 - c.h, c.y + uy))
        return
      }
      let x0 = c.x, y0 = c.y, x1 = c.x + c.w, y1 = c.y + c.h
      if (drag.corner!.includes('l')) x0 = Math.min(x1 - 0.03, Math.max(0, c.x + ux))
      if (drag.corner!.includes('r')) x1 = Math.max(x0 + 0.03, Math.min(1, c.x + c.w + ux))
      if (drag.corner!.includes('t')) y0 = Math.min(y1 - 0.03, Math.max(0, c.y + uy))
      if (drag.corner!.includes('b')) y1 = Math.max(y0 + 0.03, Math.min(1, c.y + c.h + uy))
      Object.assign(b, { x: x0, y: y0, w: x1 - x0, h: y1 - y0 })
    }
  }

  function onUp() {
    if (drag?.kind === 'create' && draft) {
      const d = draft
      let keep = true
      if (d.kind === 'ink') keep = d.points.length > 1
      else if (d.kind === 'line') keep = Math.hypot(d.x2 - d.x, d.y2 - d.y) > 3
      else if (d.w < 4 || d.h < 4) {
        // A click makes a default-size shape.
        const w = mode === 'box' ? 120 : 100
        const h = mode === 'box' ? 18 : 60
        Object.assign(d, { w, h })
      }
      if (keep) {
        snapshot()
        ed.items.push(d)
        if (mode !== 'ink' && mode !== 'box') {
          selected = ed.items.length - 1
          mode = 'select'
        }
      }
    }
    draft = null
    drag = null
  }

  function removeSelected() {
    if (selected < 0) return
    snapshot()
    ed.items.splice(selected, 1)
    selected = -1
  }

  function onKey(e: KeyboardEvent) {
    const t = e.target as HTMLElement
    if (t?.closest?.('input, textarea, select') || !doc) return
    if ((e.key === 'Delete' || e.key === 'Backspace') && selected >= 0) {
      e.preventDefault()
      removeSelected()
    } else if (e.ctrlKey && e.key.toLowerCase() === 'z') {
      e.preventDefault()
      undo()
    } else if (e.key === 'Escape') {
      selected = -1
      mode = tool.id === 'redact' ? 'box' : 'select'
    } else if (e.key === 'PageDown' || (e.key === 'ArrowRight' && selected < 0)) {
      go(ed.page + 1)
    } else if (e.key === 'PageUp' || (e.key === 'ArrowLeft' && selected < 0)) {
      go(ed.page - 1)
    }
  }

  // ---- pictures ----
  let imageInput = $state<HTMLInputElement>()
  let textArea = $state<HTMLTextAreaElement>()
  function onImage(e: Event) {
    const input = e.currentTarget as HTMLInputElement
    const f = input.files?.[0]
    input.value = ''
    if (!f) return
    const r = new FileReader()
    r.onload = () => {
      const url = String(r.result)
      const img = new Image()
      img.onload = () => {
        pendingImage = { url, w: img.naturalWidth, h: img.naturalHeight }
        mode = 'image'
        toast(L('Klik di halaman untuk menaruh gambar', 'Click on the page to place the picture'), 'info')
      }
      img.src = url
    }
    r.readAsDataURL(f)
  }
  function addSignature(url: string, w: number, h: number) {
    signatures.push({ url, w, h })
    sigIndex = signatures.length - 1
    mode = 'sign'
    toast(L('Klik di halaman untuk menaruh tanda tangan', 'Click on the page to place the signature'), 'info')
  }

  // ---- redact search ----
  let query = $state('')
  let matchCase = $state(false)
  let searching = $state(false)
  async function findAll() {
    if (!doc || !query.trim()) return
    searching = true
    try {
      const rects = await api.pdfFind(doc.path, query, matchCase)
      if (!rects.length) {
        toast(L('Teks tidak ditemukan', 'Text not found'), 'info')
        return
      }
      snapshot()
      for (const r of rects) {
        const ps = doc.info.pages[r.page - 1]
        const it = blankItem('rect', r.x * ps.w, r.y * ps.h)
        Object.assign(it, { page: r.page, w: r.w * ps.w, h: r.h * ps.h, fill: '#000000', color: '', stroke: 0, opacity: 1 })
        ed.items.push(it)
      }
      toast(L(`${rects.length} tempat ditandai`, `${rects.length} places marked`), 'ok')
    } catch (e) {
      toast(errText(e), 'err')
    } finally {
      searching = false
    }
  }

  // ---- crop ----
  function resetCrop() {
    o.crop.box = { page: 1, x: 0.05, y: 0.05, w: 0.9, h: 0.9 }
  }
  let cropAll = $state(true)

  // ---- run ----
  const task = $derived(single[tool.id] ? tasks[single[tool.id]] : undefined)
  let starting = $state(false)
  const redactCount = $derived(ed.items.filter((it) => it.kind === 'rect').length)
  const blocked = $derived.by(() => {
    if (!doc) return L('Buka PDF dulu.', 'Open a PDF first.')
    if ((tool.id === 'edit' || tool.id === 'sign') && ed.items.length === 0) return tool.id === 'sign' ? L('Taruh tanda tangan di halaman.', 'Place a signature on the page.') : L('Tambahkan sesuatu ke halaman.', 'Add something to the page.')
    if (tool.id === 'redact' && redactCount === 0) return L('Tandai area yang mau disensor.', 'Mark the areas to redact.')
    return ''
  })
  async function start() {
    if (!doc) return
    starting = true
    const sources = [{ path: doc.path, password: doc.password }]
    try {
      if (tool.id === 'redact') {
        const boxes = ed.items
          .filter((it) => it.kind === 'rect')
          .map((it) => {
            const ps = doc.info.pages[it.page - 1]
            return { page: it.page, x: it.x / ps.w, y: it.y / ps.h, w: it.w / ps.w, h: it.h / ps.h }
          })
        await startEdit({ tool: 'redact', sources, redact: { boxes, dpi: 200, color: '#000000' } })
      } else if (tool.id === 'crop') {
        const crop = { ...$state.snapshot(o.crop), pages: o.crop.mode === 'box' && !cropAll ? String(ed.page + 1) : o.crop.pages }
        await startEdit({ tool: 'crop', sources, crop })
      } else {
        await startEdit({ tool: tool.id, sources, items: $state.snapshot(ed.items) as EditItem[] })
      }
    } finally {
      starting = false
    }
  }

  const sel = $derived(selected >= 0 ? ed.items[selected] : null)
  const startLabel = $derived(
    { edit: L('Simpan PDF', 'Save PDF'), sign: L('Tanda tangani', 'Sign'), redact: L(`Sensor ${redactCount} area`, `Redact ${redactCount} areas`), crop: L('Potong', 'Crop') }[tool.id] ?? L('Mulai', 'Start'),
  )
  const counts = $derived.by(() => {
    const c: Record<number, number> = {}
    for (const it of ed.items) c[it.page] = (c[it.page] ?? 0) + 1
    return c
  })
</script>

<svelte:window onkeydown={onKey} />
<input bind:this={imageInput} type="file" accept="image/png,image/jpeg" hidden onchange={onImage} />

<div class="body">
  {#if !doc}
    <OpenDocView
      title={L('Buka PDF', 'Open a PDF')}
      subtitle={{
        edit: L('Tambahkan teks, bentuk, coretan, atau gambar langsung di halaman.', 'Add text, shapes, drawings or pictures right on the page.'),
        sign: L('Bubuhkan tanda tangan dan tanggal di dokumen.', 'Put your signature and the date on the document.'),
        redact: L('Tandai atau cari teks yang mau dihitamkan permanen.', 'Mark or search the text to black out for good.'),
        crop: L('Pilih area halaman yang mau dipakai.', 'Choose the part of the page to keep.'),
      }[tool.id] ?? ''}
      busy={opening}
      onpick={pick}
    />
  {:else}
    <section class="card work">
      <div class="toolbar">
        {#each toolsFor[tool.id] as tb (tb.m)}
          <button class="tbtn" class:on={mode === tb.m} title={tb.label()} aria-label={tb.label()} aria-pressed={mode === tb.m} onclick={() => {
            mode = tb.m
            if (tb.m === 'image') imageInput?.click()
            if (tb.m === 'sign' && signatures.length === 0) sigDialog = true
          }}><Icon name={tb.icon} size={17} /></button>
        {/each}
        {#if toolsFor[tool.id].length}<span class="sep"></span>{/if}
        {#if tool.id !== 'crop'}
          <button class="tbtn" title={L('Urungkan (Ctrl+Z)', 'Undo (Ctrl+Z)')} aria-label={L('Urungkan', 'Undo')} onclick={undo}><Icon name="undo" size={17} /></button>
          <button class="tbtn" title={L('Hapus yang dipilih (Delete)', 'Delete selected (Delete)')} aria-label={L('Hapus yang dipilih', 'Delete selected')} disabled={selected < 0} onclick={removeSelected}><Icon name="trash" size={17} /></button>
        {/if}
        <span class="grow"></span>
        <button class="tbtn" aria-label={L('Halaman sebelumnya', 'Previous page')} disabled={ed.page === 0} onclick={() => go(ed.page - 1)}><Icon name="chevronLeft" size={17} /></button>
        <span class="pg">{ed.page + 1} / {n}</span>
        <button class="tbtn" aria-label={L('Halaman berikutnya', 'Next page')} disabled={ed.page >= n - 1} onclick={() => go(ed.page + 1)}><Icon name="chevronRight" size={17} /></button>
        <span class="sep"></span>
        <button class="tbtn" aria-label={L('Perkecil', 'Zoom out')} onclick={() => (zoom = Math.max(0.3, zoom / 1.25))}><Icon name="zoomOut" size={17} /></button>
        <button class="zoomv" onclick={() => (zoom = 1)} title={L('Pas di layar', 'Fit')}>{Math.round(zoom * 100)}%</button>
        <button class="tbtn" aria-label={L('Perbesar', 'Zoom in')} onclick={() => (zoom = Math.min(5, zoom * 1.25))}><Icon name="zoomIn" size={17} /></button>
        <span class="sep"></span>
        <button class="tbtn" title={L('Ganti PDF', 'Change PDF')} aria-label={L('Ganti PDF', 'Change PDF')} onclick={pick}><Icon name="folder" size={17} /></button>
      </div>
      <div class="split">
        <div class="strip">
          {#each doc.info.pages as p, i}
            <button class="th" class:on={i === ed.page} onclick={() => go(i)} aria-label="{L('Halaman', 'Page')} {i + 1}">
              <img src={pageUrl(doc.path, i, 120)} alt="" loading="lazy" style="aspect-ratio: {p.w} / {p.h}" />
              <span>{i + 1}</span>
              {#if counts[i + 1]}<b class="badge">{counts[i + 1]}</b>{/if}
            </button>
          {/each}
        </div>
        <div class="area" bind:this={area} bind:clientWidth={areaW} bind:clientHeight={areaH}>
          <div class="page" style="width: {size.w * scale}px; height: {size.h * scale}px">
            {#key `${doc.path}-${ed.page}`}
              <img class="pimg" src={pageUrl(doc.path, ed.page, renderW)} alt="{L('Halaman', 'Page')} {ed.page + 1}" draggable="false" />
            {/key}
            <svg
              bind:this={svg}
              viewBox="0 0 {size.w} {size.h}"
              class="ov mode-{mode}"
              role="application"
              aria-label={L('Area gambar halaman', 'Page drawing area')}
              onpointerdown={bgDown}
              onpointermove={onMove}
              onpointerup={onUp}
              onpointercancel={onUp}
            >
              {#each pageItems as { it, i } (i)}
                {@render shape(it, i)}
              {/each}
              {#if draft}{@render shape(draft, -2)}{/if}
              {#if sel && sel.page === ed.page + 1}
                {@const b = bbox(sel)}
                <rect class="selbox" x={b.x - 3} y={b.y - 3} width={b.w + 6} height={b.h + 6} vector-effect="non-scaling-stroke" />
                {#if sel.kind !== 'line' && sel.kind !== 'ink'}
                  <circle class="handle" cx={b.x + b.w + 3} cy={b.y + b.h + 3} r={6 / scale} role="presentation" onpointerdown={(e) => handleDown(e, 'br')} />
                {/if}
              {/if}
              {#if tool.id === 'crop' && o.crop.mode === 'box'}
                {@const c = o.crop.box}
                {@const x = c.x * size.w}
                {@const y = c.y * size.h}
                {@const w = c.w * size.w}
                {@const h = c.h * size.h}
                <path class="dim" fill-rule="evenodd" d="M0 0H{size.w}V{size.h}H0Z M{x} {y}H{x + w}V{y + h}H{x}Z" />
                <rect class="cropbox" {x} {y} width={w} height={h} role="presentation" onpointerdown={cropDown} vector-effect="non-scaling-stroke" />
                {#each [['tl', x, y], ['tr', x + w, y], ['bl', x, y + h], ['br', x + w, y + h]] as [k, hx, hy] (k)}
                  <rect class="chandle" x={Number(hx) - 7 / scale} y={Number(hy) - 7 / scale} width={14 / scale} height={14 / scale} role="presentation" onpointerdown={(e) => handleDown(e, String(k))} />
                {/each}
              {/if}
            </svg>
          </div>
        </div>
      </div>
    </section>
  {/if}

  {#snippet shape(it: EditItem, i: number)}
    <g class="item" class:redact={tool.id === 'redact'} opacity={it.opacity} onpointerdown={(e) => i >= 0 && itemDown(e, i)} role="presentation">
      {#if it.kind === 'text'}
        {#each it.text.split('\n') as line, k}
          <text x={it.x} y={it.y + k * it.size * 1.2} font-size={it.size} font-family="Helvetica, Arial, sans-serif" font-weight={it.bold ? 700 : 400} fill={it.color || '#000'} xml:space="preserve">{line}</text>
        {/each}
        {@const b = bbox(it)}
        <rect x={b.x} y={b.y} width={b.w} height={b.h} fill="transparent" />
      {:else if it.kind === 'rect'}
        <rect x={it.x} y={it.y} width={it.w} height={it.h} fill={it.fill || 'none'} stroke={it.color || 'none'} stroke-width={it.stroke} pointer-events="all" />
      {:else if it.kind === 'ellipse'}
        <ellipse cx={it.x + it.w / 2} cy={it.y + it.h / 2} rx={it.w / 2} ry={it.h / 2} fill={it.fill || 'none'} stroke={it.color || 'none'} stroke-width={it.stroke} pointer-events="all" />
      {:else if it.kind === 'line'}
        <line x1={it.x} y1={it.y} x2={it.x2} y2={it.y2} stroke={it.color} stroke-width={it.stroke} stroke-linecap="round" />
        <line x1={it.x} y1={it.y} x2={it.x2} y2={it.y2} stroke="transparent" stroke-width={Math.max(10, it.stroke)} />
      {:else if it.kind === 'ink'}
        <polyline points={it.points.map((p) => p.join(',')).join(' ')} fill="none" stroke={it.color} stroke-width={it.stroke} stroke-linecap="round" stroke-linejoin="round" />
        <polyline points={it.points.map((p) => p.join(',')).join(' ')} fill="none" stroke="transparent" stroke-width={Math.max(10, it.stroke)} />
      {:else if it.kind === 'image'}
        <image href={it.imageUrl} x={it.x} y={it.y} width={it.w} height={it.h} preserveAspectRatio="none" />
      {/if}
    </g>
  {/snippet}

  <aside class="card panel" aria-label={L('Pengaturan', 'Settings')}>
    <div class="scroll">
      {#if tool.id === 'edit' || tool.id === 'sign'}
        {#if tool.id === 'sign'}
          <div class="sec">
            <span class="label">{L('Tanda tangan', 'Signatures')}</span>
            <div class="sigs">
              {#each signatures as s, i}
                <button class="sig" class:on={sigIndex === i && mode === 'sign'} onclick={() => {
                  sigIndex = i
                  mode = 'sign'
                }} aria-label="{L('Tanda tangan', 'Signature')} {i + 1}"><img src={s.url} alt="" /></button>
              {/each}
              <button class="sig add" onclick={() => (sigDialog = true)}><Icon name="plus" size={18} />{L('Buat baru', 'New')}</button>
            </div>
            <p class="hint">{L('Pilih tanda tangan, lalu klik di halaman. Geser untuk memindah, tarik titik oranye untuk mengubah ukuran.', 'Pick a signature, then click on the page. Drag to move it, pull the orange dot to resize.')}</p>
          </div>
        {/if}
        {#if sel}
          <div class="sec">
            <span class="label">{L('Objek terpilih', 'Selected object')}</span>
            {#if sel.kind === 'text'}
              <textarea bind:this={textArea} class="text-input ta" bind:value={sel.text} rows="3" aria-label={L('Isi teks', 'Text')}></textarea>
              <div class="row-between">
                <span class="t13">{L('Ukuran', 'Size')}</span>
                <input class="text-input small" type="number" min="4" max="200" bind:value={sel.size} aria-label={L('Ukuran huruf', 'Font size')} />
              </div>
              <label class="check"><input type="checkbox" bind:checked={sel.bold} /> {L('Tebal', 'Bold')}</label>
            {/if}
            {#if sel.kind !== 'image'}
              <div class="row-between">
                <span class="t13">{sel.kind === 'text' ? L('Warna', 'Colour') : L('Warna garis', 'Line colour')}</span>
                <input class="color" type="color" value={sel.color || '#000000'} oninput={(e) => (sel.color = (e.currentTarget as HTMLInputElement).value)} aria-label={L('Warna', 'Colour')} />
              </div>
            {/if}
            {#if sel.kind === 'rect' || sel.kind === 'ellipse'}
              <div class="row-between">
                <span class="t13">{L('Isi', 'Fill')}</span>
                <div class="row">
                  <label class="check"><input type="checkbox" checked={!!sel.fill} onchange={(e) => (sel.fill = (e.currentTarget as HTMLInputElement).checked ? '#ffe066' : '')} /> {L('Berwarna', 'Filled')}</label>
                  {#if sel.fill}<input class="color" type="color" value={sel.fill} oninput={(e) => (sel.fill = (e.currentTarget as HTMLInputElement).value)} aria-label={L('Warna isi', 'Fill colour')} />{/if}
                </div>
              </div>
            {/if}
            {#if sel.kind !== 'text' && sel.kind !== 'image'}
              <div class="row-between"><span class="t13">{L('Tebal garis', 'Line width')}</span><span class="val">{sel.stroke} pt</span></div>
              <input type="range" min="0" max="12" step="0.5" bind:value={sel.stroke} aria-label={L('Tebal garis', 'Line width')} />
            {/if}
            <div class="row-between"><span class="t13">{L('Transparansi', 'Opacity')}</span><span class="val">{Math.round(sel.opacity * 100)}%</span></div>
            <input type="range" min="0.1" max="1" step="0.05" bind:value={sel.opacity} aria-label={L('Transparansi', 'Opacity')} />
            <button class="btn danger" onclick={removeSelected}><Icon name="trash" size={14} />{L('Hapus objek', 'Delete object')}</button>
          </div>
        {:else if tool.id === 'edit'}
          <div class="sec">
            <span class="label">{L('Gaya untuk objek baru', 'Style for new objects')}</span>
            <div class="row-between"><span class="t13">{L('Warna', 'Colour')}</span><input class="color" type="color" bind:value={style.color} aria-label={L('Warna', 'Colour')} /></div>
            <div class="row-between"><span class="t13">{L('Ukuran teks', 'Text size')}</span><input class="text-input small" type="number" min="4" max="200" bind:value={style.size} aria-label={L('Ukuran teks', 'Text size')} /></div>
            <div class="row-between"><span class="t13">{L('Tebal garis', 'Line width')}</span><span class="val">{style.stroke} pt</span></div>
            <input type="range" min="0.5" max="12" step="0.5" bind:value={style.stroke} aria-label={L('Tebal garis', 'Line width')} />
            <p class="hint">{L('Pilih alat di atas, lalu klik atau tarik di halaman. "Tutup putih" menutupi tulisan lama agar bisa ditimpa teks baru.', 'Pick a tool above, then click or drag on the page. "White-out" covers old text so you can write over it.')}</p>
          </div>
        {/if}
      {:else if tool.id === 'redact'}
        <div class="sec">
          <span class="label">{L('Cari & sensor teks', 'Find & redact text')}</span>
          <div class="row">
            <input class="text-input" bind:value={query} placeholder={L('mis. nomor rekening, nama', 'e.g. account number, name')} onkeydown={(e) => e.key === 'Enter' && findAll()} />
            <button class="btn" onclick={findAll} disabled={searching || !query.trim()}><Icon name={searching ? 'loader' : 'search'} size={14} class={searching ? 'spin' : ''} /></button>
          </div>
          <label class="check"><input type="checkbox" bind:checked={matchCase} /> {L('Huruf besar/kecil harus sama', 'Match case')}</label>
        </div>
        <div class="sec">
          <span class="label">{L('Area ditandai', 'Marked areas')}: {redactCount}</span>
          <p class="hint">{L('Tarik kotak di atas teks atau gambar. Klik kotak untuk memilih, tekan Delete untuk menghapus.', 'Drag a box over text or pictures. Click a box to select it, press Delete to remove it.')}</p>
          {#if redactCount}<button class="btn" onclick={() => { snapshot(); ed.items = []; selected = -1 }}><Icon name="trash" size={14} />{L('Hapus semua tanda', 'Clear all marks')}</button>{/if}
        </div>
        <div class="note"><Icon name="alert" size={16} /><span>{L('Halaman yang disensor diubah jadi gambar, jadi teks di bawah kotak benar-benar hilang dan tidak bisa disalin atau dipulihkan. Halaman lain tidak berubah.', "Redacted pages are turned into images, so the text under the boxes is really gone and can't be copied or recovered. Other pages stay as they are.")}</span></div>
      {:else if tool.id === 'crop'}
        <div class="sec">
          <Segmented
            label={L('Cara memotong', 'How to crop')}
            bind:value={o.crop.mode}
            options={[
              { value: 'box', label: L('Pilih area', 'Pick area') },
              { value: 'margins', label: L('Margin (mm)', 'Margins (mm)') },
            ]}
          />
        </div>
        {#if o.crop.mode === 'box'}
          <p class="hint">{L('Tarik sudut kotak oranye untuk memilih area yang disimpan. Area gelap dibuang.', 'Drag the corners of the orange box to choose what to keep. The dark part is removed.')}</p>
          <Switch bind:checked={cropAll} label={L('Terapkan ke semua halaman', 'Apply to every page')} hint={cropAll ? '' : L(`Hanya halaman ${ed.page + 1}`, `Only page ${ed.page + 1}`)} />
          <button class="btn" onclick={resetCrop}><Icon name="undo" size={14} />{L('Atur ulang area', 'Reset area')}</button>
        {:else}
          <div class="grid2">
            {#each [['top', L('Atas', 'Top')], ['bottom', L('Bawah', 'Bottom')], ['left', L('Kiri', 'Left')], ['right', L('Kanan', 'Right')]] as [k, label]}
              <label class="sec tight"><span class="label">{label}</span><input class="text-input" type="number" min="0" max="200" bind:value={(o.crop as any)[k]} /></label>
            {/each}
          </div>
          <div class="sec tight">
            <label class="label" for="cpages">{L('Halaman', 'Pages')}</label>
            <input id="cpages" class="text-input" bind:value={o.crop.pages} placeholder={L('Semua halaman — atau mis. 1-3', 'Every page — or e.g. 1-3')} />
          </div>
        {/if}
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

<SignatureDialog bind:open={sigDialog} onsave={addSignature} />

<style>
  .body {
    flex-grow: 1;
    min-height: 0;
    display: flex;
    gap: 20px;
    padding: 0 24px 24px 28px;
  }
  .work {
    flex-grow: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .toolbar {
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 8px 10px;
    border-bottom: 1px solid var(--border);
    overflow-x: auto;
  }
  .tbtn {
    width: 36px;
    height: 36px;
    border: 0;
    border-radius: 9px;
    background: transparent;
    color: var(--text-2);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .tbtn:hover:not(:disabled) {
    background: var(--surface-2);
    color: var(--text);
  }
  .tbtn.on {
    background: var(--accent-soft);
    color: var(--accent-text);
  }
  .tbtn:disabled {
    opacity: 0.35;
  }
  .sep {
    width: 1px;
    height: 22px;
    background: var(--border);
    margin: 0 4px;
  }
  .grow {
    flex-grow: 1;
  }
  .pg {
    min-width: 54px;
    text-align: center;
    font-size: 13px;
    font-weight: 700;
    font-variant-numeric: tabular-nums;
  }
  .zoomv {
    min-width: 52px;
    height: 30px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--text-2);
    font-size: 12px;
    font-weight: 700;
  }
  .btn.small {
    height: 32px;
    font-size: 12px;
  }
  .split {
    flex-grow: 1;
    min-height: 0;
    display: flex;
  }
  .strip {
    width: 104px;
    flex-shrink: 0;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 12px 10px;
    border-right: 1px solid var(--border-soft);
  }
  .th {
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    padding: 5px;
    border-radius: 8px;
    border: 2px solid transparent;
    background: transparent;
    color: var(--text-3);
    font-size: 11px;
    font-weight: 700;
  }
  .th img {
    width: 100%;
    background: #fff;
  }
  .th.on {
    border-color: var(--accent);
    color: var(--accent-text);
  }
  .badge {
    position: absolute;
    top: 2px;
    right: 2px;
    min-width: 18px;
    height: 18px;
    border-radius: 9px;
    background: var(--accent);
    color: var(--accent-ink);
    font-size: 10px;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0 4px;
  }
  .area {
    flex-grow: 1;
    min-width: 0;
    overflow: auto;
    display: flex;
    padding: 24px;
    background: var(--canvas-2);
  }
  .page {
    position: relative;
    margin: auto;
    flex-shrink: 0;
    background: #fff;
    box-shadow: 0 6px 30px rgba(0, 0, 0, 0.5);
  }
  .pimg {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    user-select: none;
  }
  .ov {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    touch-action: none;
    cursor: crosshair;
  }
  .ov.mode-select {
    cursor: default;
  }
  .ov.mode-text,
  .ov.mode-date {
    cursor: text;
  }
  .ov :global(.item) {
    cursor: move;
  }
  .ov.mode-select :global(.item:hover) {
    filter: drop-shadow(0 0 2px rgba(255, 122, 69, 0.8));
  }
  .ov :global(.item.redact rect) {
    fill-opacity: 0.85;
    stroke: #ff5a5a;
    stroke-dasharray: 3 2;
  }
  .ov :global(text) {
    user-select: none;
  }
  .selbox {
    fill: none;
    stroke: var(--accent);
    stroke-width: 1.5;
    stroke-dasharray: 5 3;
    pointer-events: none;
  }
  .handle {
    fill: var(--accent);
    stroke: #fff;
    stroke-width: 1;
    cursor: nwse-resize;
  }
  .dim {
    fill: rgba(10, 12, 16, 0.6);
    pointer-events: none;
  }
  .cropbox {
    fill: transparent;
    stroke: var(--accent);
    stroke-width: 2;
    cursor: move;
  }
  .chandle {
    fill: var(--accent);
    stroke: #fff;
    stroke-width: 0.5;
    cursor: nwse-resize;
  }
  .panel {
    width: 320px;
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
    gap: 6px;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .row-between {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 10px;
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
  .ta {
    height: auto;
    padding: 10px 12px;
    resize: vertical;
    font-family: Helvetica, Arial, sans-serif;
  }
  .text-input.small {
    width: 80px;
    height: 34px;
  }
  input[type='range'] {
    width: 100%;
    margin: 0;
    accent-color: var(--accent);
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
  .check {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    font-weight: 600;
    color: var(--text-2);
  }
  .check input {
    accent-color: var(--accent);
  }
  .sigs {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px;
  }
  .sig {
    height: 64px;
    border-radius: 10px;
    border: 2px solid var(--border);
    background: #fff;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 6px;
  }
  .sig img {
    max-width: 100%;
    max-height: 100%;
  }
  .sig.on {
    border-color: var(--accent);
  }
  .sig.add {
    background: transparent;
    border-style: dashed;
    color: var(--accent);
    gap: 6px;
    font-size: 12px;
    font-weight: 700;
  }
  .note {
    display: flex;
    gap: 10px;
    align-items: flex-start;
    padding: 12px;
    border-radius: 10px;
    background: var(--note-bg);
    color: var(--warn);
    font-size: 12px;
    line-height: 1.5;
  }
  .note :global(svg) {
    flex-shrink: 0;
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
