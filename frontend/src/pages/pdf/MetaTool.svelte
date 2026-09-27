<script lang="ts">
  import { L } from '../../lib/i18n.svelte'
  import { onDestroy, onMount } from 'svelte'
  import Icon from '../../components/Icon.svelte'
  import OpenDocView from './OpenDoc.svelte'
  import SinglePanel from './SinglePanel.svelte'
  import { api, errText, pageUrl } from '../../lib/api'
  import { toast } from '../../lib/stores/app.svelte'
  import { openDoc, pdfDrop, pickDoc, single, type OpenDoc } from '../../lib/stores/pdf.svelte'
  import { trackBatch } from '../../lib/stores/tasks.svelte'
  import type { PdfBookmark, PdfMeta } from '../../lib/types'
  import type { PdfTool } from '../../lib/pdfTools'

  let { tool }: { tool: PdfTool } = $props()

  // Bookmarks are edited as a flat list with a nesting level.
  interface Row {
    key: number
    title: string
    page: number
    level: number
  }

  let doc = $state<OpenDoc | null>(null)
  let opening = $state(false)
  let meta = $state<PdfMeta>({ title: '', author: '', subject: '', keywords: '', creator: '' })
  let rows = $state<Row[]>([])
  let original = ''
  let focus = $state(-1)
  let seq = 0

  const pages = $derived(doc?.info.pages.length ?? 0)

  function flatten(list: PdfBookmark[], level = 0, out: Row[] = []): Row[] {
    for (const b of list) {
      out.push({ key: ++seq, title: b.title, page: b.page, level })
      flatten(b.kids ?? [], level + 1, out)
    }
    return out
  }

  function tree(list: Row[]): PdfBookmark[] {
    const root: PdfBookmark[] = []
    const stack: { level: number; kids: PdfBookmark[] }[] = [{ level: -1, kids: root }]
    for (const r of list) {
      const b: PdfBookmark = { title: r.title.trim() || L('Tanpa judul', 'Untitled'), page: r.page, kids: [] }
      while (stack.length > 1 && stack[stack.length - 1].level >= r.level) stack.pop()
      stack[stack.length - 1].kids.push(b)
      stack.push({ level: r.level, kids: b.kids })
    }
    return root
  }

  const shape = (list: Row[]) => JSON.stringify(list.map((r) => [r.title, r.page, r.level]))

  async function load(d: OpenDoc | null) {
    if (!d) return
    try {
      const det = await api.pdfDetails(d.path, d.password)
      doc = d
      meta = det.meta
      rows = flatten(det.bookmarks ?? [])
      original = shape(rows)
      focus = rows.length ? 0 : -1
      single[tool.id] = ''
    } catch (e) {
      toast(errText(e), 'err')
    }
  }

  async function pick(path?: string) {
    opening = true
    try {
      await load(path ? await openDoc(path) : await pickDoc())
    } finally {
      opening = false
    }
  }

  onMount(() => {
    pdfDrop.fn = (paths) => {
      const p = paths.find((x) => x.toLowerCase().endsWith('.pdf'))
      if (p) pick(p)
      else toast(L('Tarik file PDF', 'Drop a PDF file'), 'info')
    }
  })
  onDestroy(() => (pdfDrop.fn = null))

  function add(after: number) {
    const ref = rows[after]
    const r: Row = { key: ++seq, title: '', page: Math.min(pages, (ref?.page ?? 0) + 1) || 1, level: ref?.level ?? 0 }
    rows.splice(after + 1, 0, r)
    focus = after + 1
    requestAnimationFrame(() => document.getElementById(`bm-${r.key}`)?.focus())
  }

  function remove(i: number) {
    // Children move up one level so they are not lost.
    const lvl = rows[i].level
    for (let j = i + 1; j < rows.length && rows[j].level > lvl; j++) rows[j].level--
    rows.splice(i, 1)
    focus = Math.min(focus, rows.length - 1)
  }

  function indent(i: number, by: number) {
    const r = rows[i]
    const max = i === 0 ? 0 : rows[i - 1].level + 1
    const next = Math.max(0, Math.min(max, r.level + by))
    const diff = next - r.level
    if (!diff) return
    // Move the children along.
    for (let j = i + 1; j < rows.length && rows[j].level > r.level; j++) rows[j].level += diff
    r.level = next
  }

  // A row moves with its children.
  function move(i: number, dir: -1 | 1) {
    const end = (k: number) => {
      let e = k + 1
      while (e < rows.length && rows[e].level > rows[k].level) e++
      return e
    }
    const block = rows.slice(i, end(i))
    if (dir < 0) {
      let j = i - 1
      while (j > 0 && rows[j].level > rows[i].level) j--
      if (j < 0 || rows[j].level < rows[i].level) return
      rows.splice(i, block.length)
      rows.splice(j, 0, ...block)
      focus = j
    } else {
      const k = end(i)
      if (k >= rows.length || rows[k].level < rows[i].level) return
      const after = end(k)
      rows.splice(i, block.length)
      rows.splice(after - block.length, 0, ...block)
      focus = after - block.length
    }
    for (let j = 0; j < rows.length; j++) rows[j].level = Math.min(rows[j].level, j === 0 ? 0 : rows[j - 1].level + 1)
  }

  function setPage(r: Row, v: string) {
    const n = parseInt(v, 10)
    if (Number.isFinite(n)) r.page = Math.max(1, Math.min(pages, n))
  }

  const blocked = $derived.by(() => {
    if (!doc) return L('Buka PDF dulu.', 'Open a PDF first.')
    return ''
  })

  async function start() {
    if (!doc) return
    try {
      const keep = shape(rows) === original
      const ref = await api.startPdfMeta({ id: 'doc', path: doc.path, password: doc.password }, $state.snapshot(meta), tree(rows), keep)
      single[tool.id] = ref.taskId
      trackBatch('pdf', [ref.taskId])
    } catch (e) {
      toast(errText(e), 'err')
    }
  }

  const preview = $derived(focus >= 0 && rows[focus] ? rows[focus].page : 1)
</script>

<div class="body">
  {#if !doc}
    <OpenDocView
      title={L('Buka PDF yang mau diubah propertinya', 'Open the PDF to edit')}
      subtitle={L('Ubah judul, penulis, kata kunci, dan daftar isi (bookmark) yang tampil di panel samping pembaca PDF.', 'Edit the title, author, keywords and the table of contents (bookmarks) shown in the side panel of PDF readers.')}
      busy={opening}
      onpick={() => pick()}
    />
  {:else}
    <section class="card main">
      <div class="toolbar">
        <div class="count">
          <span class="n ellipsis" title={doc.path}>{doc.name}</span>
          <span class="size">{pages} {L('halaman', 'pages')}</span>
        </div>
        <button class="btn" onclick={() => pick()} disabled={opening}><Icon name={opening ? 'loader' : 'folder'} size={16} class={opening ? 'spin' : ''} />{L('Ganti PDF', 'Change PDF')}</button>
      </div>
      <div class="content">
        <div class="edit">
          <div class="props">
            <label class="label" for="m-title">{L('Judul', 'Title')}</label>
            <input id="m-title" class="text-input" bind:value={meta.title} placeholder={L('Tampil di judul jendela pembaca PDF', 'Shown in the PDF reader title bar')} />
            <label class="label" for="m-author">{L('Penulis', 'Author')}</label>
            <input id="m-author" class="text-input" bind:value={meta.author} />
            <label class="label" for="m-subject">{L('Subjek', 'Subject')}</label>
            <input id="m-subject" class="text-input" bind:value={meta.subject} />
            <label class="label" for="m-kw">{L('Kata kunci', 'Keywords')}</label>
            <input id="m-kw" class="text-input" bind:value={meta.keywords} placeholder={L('Pisahkan dengan koma', 'Separate with commas')} />
            <label class="label" for="m-creator">{L('Aplikasi pembuat', 'Creator app')}</label>
            <input id="m-creator" class="text-input" bind:value={meta.creator} />
          </div>

          <div class="bmhead">
            <b>{L('Bookmark (daftar isi)', 'Bookmarks (table of contents)')}</b>
            <span class="size">{rows.length}</span>
            <span class="grow"></span>
            {#if rows.length}
              <button class="btn small" onclick={() => ((rows = []), (focus = -1))}><Icon name="trash" size={14} />{L('Hapus semua', 'Remove all')}</button>
            {/if}
            <button class="btn small" onclick={() => add(focus >= 0 ? focus : rows.length - 1)}><Icon name="plus" size={14} />{L('Tambah', 'Add')}</button>
          </div>
          {#if rows.length === 0}
            <p class="hint">{L('Belum ada bookmark. Klik Tambah untuk membuat daftar isi: tiap bookmark melompat ke halaman tertentu.', 'No bookmarks yet. Click Add to build a table of contents: each bookmark jumps to a page.')}</p>
          {:else}
            <ol class="bms">
              {#each rows as r, i (r.key)}
                <li class="bm" class:focus={i === focus} style="--lvl: {r.level}">
                  <input
                    id="bm-{r.key}"
                    class="text-input t"
                    bind:value={r.title}
                    placeholder={L('Judul bookmark', 'Bookmark title')}
                    onfocus={() => (focus = i)}
                    onkeydown={(e) => {
                      if (e.key === 'Enter') add(i)
                      else if (e.key === 'Tab') {
                        e.preventDefault()
                        indent(i, e.shiftKey ? -1 : 1)
                      }
                    }}
                  />
                  <input class="text-input p" inputmode="numeric" value={r.page} aria-label={L('Halaman', 'Page')} onfocus={() => (focus = i)} onchange={(e) => setPage(r, e.currentTarget.value)} />
                  <div class="acts">
                    <button class="btn icon xs" title={L('Naik', 'Up')} aria-label={L('Naik', 'Up')} onclick={() => move(i, -1)}><Icon name="arrowUp" size={13} /></button>
                    <button class="btn icon xs" title={L('Turun', 'Down')} aria-label={L('Turun', 'Down')} onclick={() => move(i, 1)}><Icon name="arrowDown" size={13} /></button>
                    <button class="btn icon xs" title={L('Keluarkan (Shift+Tab)', 'Outdent (Shift+Tab)')} aria-label={L('Keluarkan', 'Outdent')} disabled={r.level === 0} onclick={() => indent(i, -1)}><Icon name="chevronLeft" size={13} /></button>
                    <button class="btn icon xs" title={L('Jadikan sub-bab (Tab)', 'Make it a sub-item (Tab)')} aria-label={L('Jadikan sub-bab', 'Indent')} onclick={() => indent(i, 1)}><Icon name="chevronRight" size={13} /></button>
                    <button class="btn icon xs" title={L('Hapus', 'Remove')} aria-label={L('Hapus', 'Remove')} onclick={() => remove(i)}><Icon name="trash" size={13} /></button>
                  </div>
                </li>
              {/each}
            </ol>
            <p class="hint">{L('Enter: bookmark baru · Tab / Shift+Tab: jadikan sub-bab / keluarkan.', 'Enter: new bookmark · Tab / Shift+Tab: indent / outdent.')}</p>
          {/if}
        </div>
        <div class="pv">
          <span class="label">{L('Halaman', 'Page')} {preview}</span>
          {#if doc.info.pages[preview - 1]}
            {@const p = doc.info.pages[preview - 1]}
            <div class="thumb" style="aspect-ratio: {p.w} / {p.h}">
              <img src={pageUrl(doc.path, preview - 1, 360)} alt="{L('Halaman', 'Page')} {preview}" />
            </div>
          {/if}
        </div>
      </div>
    </section>
  {/if}

  <SinglePanel {tool} startLabel={L('Simpan PDF', 'Save PDF')} {blocked} onstart={start}>
    <div class="note"><Icon name="info" size={16} /><span>{L('Isi halaman tidak berubah; hanya properti dan daftar isinya. Hasilnya disimpan sebagai file baru.', "The pages don't change, only the properties and the table of contents. The result is saved as a new file.")}</span></div>
  </SinglePanel>
</div>

<style>
  .body {
    flex-grow: 1;
    min-height: 0;
    display: flex;
    gap: 20px;
    padding: 0 24px 24px 28px;
  }
  .main {
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
  }
  .size {
    font-size: 13px;
    color: var(--text-3);
    white-space: nowrap;
  }
  .content {
    flex-grow: 1;
    min-height: 0;
    display: flex;
  }
  .edit {
    flex-grow: 1;
    min-width: 0;
    overflow-y: auto;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .props {
    display: grid;
    grid-template-columns: 120px minmax(0, 1fr);
    gap: 8px 12px;
    align-items: center;
  }
  .bmhead {
    display: flex;
    align-items: baseline;
    gap: 8px;
    margin-top: 8px;
    padding-top: 14px;
    border-top: 1px solid var(--border);
    white-space: nowrap;
  }
  .grow {
    flex-grow: 1;
  }
  .btn.small {
    height: 30px;
    padding: 0 10px;
    font-size: 12px;
  }
  .bms {
    margin: 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .bm {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 6px 4px calc(6px + var(--lvl) * 22px);
    border-radius: 8px;
  }
  .bm.focus {
    background: var(--accent-tint);
  }
  .bm .t {
    flex-grow: 1;
    min-width: 0;
    height: 32px;
  }
  .bm .p {
    width: 56px;
    height: 32px;
    text-align: center;
  }
  .acts {
    display: flex;
    gap: 2px;
    opacity: 0.35;
  }
  .bm:hover .acts,
  .bm.focus .acts {
    opacity: 1;
  }
  .btn.icon.xs {
    width: 26px;
    height: 26px;
    padding: 0;
  }
  .pv {
    width: 180px;
    flex-shrink: 0;
    border-left: 1px solid var(--border);
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    background: var(--canvas);
  }
  @media (max-width: 1180px) {
    .pv {
      display: none;
    }
  }
  .thumb {
    width: 100%;
    border-radius: 6px;
    overflow: hidden;
    background: #fff;
    box-shadow: 0 1px 4px rgb(0 0 0 / 0.15);
  }
  .thumb img {
    width: 100%;
    height: 100%;
    display: block;
    object-fit: contain;
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
</style>
