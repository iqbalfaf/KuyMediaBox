<script lang="ts">
  import { L } from '../../lib/i18n.svelte'
  import { onDestroy, onMount } from 'svelte'
  import Icon from '../../components/Icon.svelte'
  import { api, errText, pageUrl } from '../../lib/api'
  import { toast } from '../../lib/stores/app.svelte'
  import { openDoc, pdfDrop, pickDoc, type OpenDoc } from '../../lib/stores/pdf.svelte'
  import type { CompareResult, PdfChange } from '../../lib/types'
  import type { PdfTool } from '../../lib/pdfTools'

  let { tool }: { tool: PdfTool } = $props()

  const cmp = $state<{ a: OpenDoc | null; b: OpenDoc | null; result: CompareResult | null }>({ a: null, b: null, result: null })
  let busy = $state(false)
  let opening = $state<'' | 'a' | 'b'>('')
  let focus = $state(-1)
  let width = $state(420)

  async function pick(side: 'a' | 'b', path?: string) {
    opening = side
    try {
      const d = path ? await openDoc(path) : await pickDoc()
      if (d) {
        cmp[side] = d
        cmp.result = null
        focus = -1
      }
    } finally {
      opening = ''
    }
    if (cmp.a && cmp.b) run()
  }

  onMount(() => {
    pdfDrop.fn = async (paths) => {
      const pdfs = paths.filter((p) => p.toLowerCase().endsWith('.pdf'))
      if (!pdfs.length) return toast(L('Tarik file PDF', 'Drop PDF files'), 'info')
      if (pdfs.length >= 2) {
        await pick('a', pdfs[0])
        await pick('b', pdfs[1])
      } else await pick(cmp.a && !cmp.b ? 'b' : 'a', pdfs[0])
    }
  })
  onDestroy(() => (pdfDrop.fn = null))

  async function run() {
    if (!cmp.a || !cmp.b) return
    busy = true
    try {
      cmp.result = await api.pdfCompare(cmp.a.path, cmp.b.path)
      if (cmp.result.same) toast(L('Teks kedua PDF sama persis', 'Both PDFs have identical text'), 'ok')
    } catch (e) {
      toast(errText(e), 'err')
    } finally {
      busy = false
    }
  }

  function swap() {
    ;[cmp.a, cmp.b] = [cmp.b, cmp.a]
    cmp.result = null
    run()
  }

  // Highlight boxes per side and page.
  function boxes(side: 'a' | 'b', page: number) {
    const kind = side === 'a' ? 'removed' : 'added'
    const out: { x: number; y: number; w: number; h: number; idx: number }[] = []
    cmp.result?.changes.forEach((c, idx) => {
      if (c.kind !== kind) return
      for (const b of c.boxes) if (b.page === page) out.push({ ...b, idx })
    })
    return out
  }

  function jump(c: PdfChange, idx: number) {
    focus = idx
    const side = c.kind === 'removed' ? 'a' : 'b'
    document.getElementById(`cmp-${side}-${c.page}`)?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  }

  // Scroll both columns together.
  let colA = $state<HTMLDivElement>()
  let colB = $state<HTMLDivElement>()
  let syncing = false
  function sync(from: HTMLDivElement | undefined, to: HTMLDivElement | undefined) {
    if (!from || !to || syncing) return
    syncing = true
    const ratio = from.scrollTop / Math.max(1, from.scrollHeight - from.clientHeight)
    to.scrollTop = ratio * (to.scrollHeight - to.clientHeight)
    requestAnimationFrame(() => (syncing = false))
  }
</script>

{#snippet column(side: 'a' | 'b', d: OpenDoc | null)}
  <div class="col card">
    <div class="chead">
      <span class="tag {side}">{side === 'a' ? L('Versi lama', 'Old version') : L('Versi baru', 'New version')}</span>
      <span class="nm ellipsis" title={d?.path}>{d?.name ?? '—'}</span>
      <button class="btn small" onclick={() => pick(side)} disabled={!!opening}><Icon name={opening === side ? 'loader' : 'folder'} size={14} class={opening === side ? 'spin' : ''} />{d ? L('Ganti', 'Change') : L('Pilih PDF', 'Choose PDF')}</button>
    </div>
    {#if d}
      <div class="pages" bind:this={() => (side === 'a' ? colA : colB), (el) => (side === 'a' ? (colA = el) : (colB = el))} onscroll={() => (side === 'a' ? sync(colA, colB) : sync(colB, colA))}>
        {#each d.info.pages as p, i}
          <div class="pg" id="cmp-{side}-{i + 1}" style="width: {width}px; aspect-ratio: {p.w} / {p.h}">
            <img src={pageUrl(d.path, i, width * 1.5)} alt="{L('Halaman', 'Page')} {i + 1}" loading="lazy" />
            {#each boxes(side, i + 1) as b}
              <span class="hl {side}" class:focus={b.idx === focus} style="left: {b.x * 100}%; top: {b.y * 100}%; width: {b.w * 100}%; height: {b.h * 100}%"></span>
            {/each}
            <span class="num">{i + 1}</span>
          </div>
        {/each}
      </div>
    {:else}
      <button class="empty" onclick={() => pick(side)}>
        <Icon name="upload" size={30} />
        <span>{side === 'a' ? L('Pilih atau tarik PDF versi lama', 'Choose or drop the old PDF') : L('Pilih atau tarik PDF versi baru', 'Choose or drop the new PDF')}</span>
      </button>
    {/if}
  </div>
{/snippet}

<div class="body">
  <div class="cols">
    {@render column('a', cmp.a)}
    {@render column('b', cmp.b)}
  </div>
  <aside class="card panel" aria-label={L('Perbedaan', 'Differences')}>
    <div class="phead">
      <b>{L('Perbedaan', 'Differences')}</b>
      <div class="row">
        <button class="btn icon" title={L('Tukar kiri/kanan', 'Swap sides')} aria-label={L('Tukar kiri/kanan', 'Swap sides')} disabled={!cmp.a || !cmp.b} onclick={swap}><Icon name="refresh" size={16} /></button>
        <input class="zoom" type="range" min="260" max="700" bind:value={width} aria-label={L('Ukuran halaman', 'Page size')} />
      </div>
    </div>
    <div class="scroll">
      {#if busy}
        <p class="hint center"><Icon name="loader" size={16} class="spin" /> {L('Membandingkan…', 'Comparing…')}</p>
      {:else if !cmp.result}
        <p class="hint">{L('Pilih dua PDF: versi lama di kiri dan versi baru di kanan. Teks yang dihapus ditandai merah, yang ditambah hijau.', 'Choose two PDFs: the old version on the left, the new one on the right. Removed text is marked red, added text green.')}</p>
      {:else if cmp.result.same}
        <div class="same"><Icon name="check" size={18} stroke={3} />{L('Tidak ada perbedaan teks.', 'No text differences.')}</div>
        {#if cmp.result.pagesA !== cmp.result.pagesB}<p class="hint">{L(`Jumlah halaman berbeda: ${cmp.result.pagesA} vs ${cmp.result.pagesB}.`, `Page counts differ: ${cmp.result.pagesA} vs ${cmp.result.pagesB}.`)}</p>{/if}
      {:else}
        <div class="sum">
          {#if cmp.result.removed}<span class="pill err">−{cmp.result.removed} {L('kata', 'words')}</span>{/if}
          {#if cmp.result.added}<span class="pill ok">+{cmp.result.added} {L('kata', 'words')}</span>{/if}
          <span class="pill muted">{cmp.result.changes.length} {L('perubahan', 'changes')}</span>
        </div>
        <ul class="changes">
          {#each cmp.result.changes as c, idx}
            <li>
              <button class="chg {c.kind}" class:focus={idx === focus} onclick={() => jump(c, idx)}>
                <span class="k">{c.kind === 'removed' ? '−' : '+'}</span>
                <span class="tx">{c.text.length > 160 ? c.text.slice(0, 160) + '…' : c.text}</span>
                <span class="p">{L('hal', 'p.')} {c.page}</span>
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
    <div class="foot">
      <button class="btn-primary" onclick={run} disabled={!cmp.a || !cmp.b || busy}><Icon name="columns" size={18} />{L('Bandingkan', 'Compare')}</button>
      <p class="hint center">{L('Yang dibandingkan adalah teksnya. PDF hasil scan perlu di-OCR dulu.', 'The text is compared. Scanned PDFs need OCR first.')}</p>
    </div>
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
  .cols {
    flex-grow: 1;
    min-width: 0;
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 14px;
  }
  .col {
    min-width: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .chead {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 12px;
    border-bottom: 1px solid var(--border);
  }
  .tag {
    padding: 3px 8px;
    border-radius: 6px;
    font-size: 11px;
    font-weight: 800;
    white-space: nowrap;
  }
  .tag.a {
    background: var(--err-soft);
    color: var(--err);
  }
  .tag.b {
    background: var(--ok-soft);
    color: var(--ok);
  }
  .nm {
    flex-grow: 1;
    font-size: 13px;
    font-weight: 700;
  }
  .btn.small {
    height: 30px;
    font-size: 12px;
  }
  .pages {
    flex-grow: 1;
    min-height: 0;
    overflow: auto;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
    padding: 16px;
    background: var(--canvas-2);
  }
  .pg {
    position: relative;
    flex-shrink: 0;
    max-width: 100%;
    background: #fff;
    box-shadow: 0 4px 18px rgba(0, 0, 0, 0.5);
  }
  .pg img {
    width: 100%;
    height: 100%;
    display: block;
  }
  .hl {
    position: absolute;
    border-radius: 2px;
    mix-blend-mode: multiply;
  }
  .hl.a {
    background: rgba(255, 70, 70, 0.4);
  }
  .hl.b {
    background: rgba(40, 200, 110, 0.4);
  }
  .hl.focus {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
  .num {
    position: absolute;
    right: 6px;
    bottom: 6px;
    padding: 1px 6px;
    border-radius: 5px;
    background: rgba(0, 0, 0, 0.55);
    color: #fff;
    font-size: 11px;
    font-weight: 700;
  }
  .empty {
    flex-grow: 1;
    margin: 14px;
    border: 2px dashed var(--border-strong);
    border-radius: 12px;
    background: var(--canvas);
    color: var(--text-3);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    font-size: 13px;
    font-weight: 600;
  }
  .empty:hover {
    color: var(--accent);
    border-color: var(--accent);
  }
  .panel {
    width: 320px;
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .phead {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 12px 16px;
    border-bottom: 1px solid var(--border);
  }
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .zoom {
    width: 90px;
    accent-color: var(--accent);
  }
  .scroll {
    flex-grow: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 14px 16px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .center {
    text-align: center;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
  }
  .same {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px;
    border-radius: 10px;
    background: var(--ok-soft);
    color: var(--ok);
    font-weight: 700;
    font-size: 13px;
  }
  .sum {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .changes {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .chg {
    width: 100%;
    display: grid;
    grid-template-columns: 16px 1fr auto;
    gap: 8px;
    align-items: start;
    padding: 8px 10px;
    border-radius: 8px;
    border: 1px solid transparent;
    background: var(--surface-2);
    text-align: left;
    font-size: 12px;
  }
  .chg.removed .k,
  .chg.removed .tx {
    color: var(--danger-text);
  }
  .chg.removed .tx {
    text-decoration: line-through;
  }
  .chg.added .k,
  .chg.added .tx {
    color: #8ff0bf;
  }
  .chg.focus {
    border-color: var(--accent);
  }
  .k {
    font-weight: 900;
  }
  .tx {
    word-break: break-word;
  }
  .p {
    color: var(--text-3);
    white-space: nowrap;
  }
  .foot {
    padding: 14px 16px 18px;
    border-top: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: 8px;
    background: var(--surface-3);
  }
</style>
