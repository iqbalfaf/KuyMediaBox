<script lang="ts">
  import { L } from '../../lib/i18n.svelte'
  import { onDestroy, onMount } from 'svelte'
  import Icon from '../../components/Icon.svelte'
  import Switch from '../../components/Switch.svelte'
  import OpenDocView from './OpenDoc.svelte'
  import SinglePanel from './SinglePanel.svelte'
  import { api, errText, pageUrl } from '../../lib/api'
  import { toast } from '../../lib/stores/app.svelte'
  import { openDoc, pdfDrop, pickDoc, single, type OpenDoc } from '../../lib/stores/pdf.svelte'
  import { trackBatch } from '../../lib/stores/tasks.svelte'
  import type { PdfFormField } from '../../lib/types'
  import type { PdfTool } from '../../lib/pdfTools'

  let { tool }: { tool: PdfTool } = $props()

  let doc = $state<OpenDoc | null>(null)
  let opening = $state(false)
  let fields = $state<PdfFormField[]>([])
  let flatten = $state(false)
  let page = $state(1)

  async function load(d: OpenDoc | null) {
    if (!d) return
    try {
      const list = await api.pdfFormFields(d.path, d.password)
      doc = d
      fields = list.map((f) => ({ ...f, values: f.values ?? [], options: f.options ?? [] }))
      page = fields[0]?.page || 1
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

  // Fields grouped by page, in document order.
  const groups = $derived.by(() => {
    const m = new Map<number, PdfFormField[]>()
    for (const f of fields) {
      const p = f.page || 0
      if (!m.has(p)) m.set(p, [])
      m.get(p)!.push(f)
    }
    return [...m.entries()].sort((a, b) => a[0] - b[0])
  })
  const editable = $derived(fields.filter((f) => !f.locked).length)

  function toggleValue(f: PdfFormField, v: string, on: boolean) {
    const set = new Set(f.values ?? [])
    if (on) set.add(v)
    else set.delete(v)
    f.values = (f.options ?? []).filter((o) => set.has(o))
  }

  function clearAll() {
    for (const f of fields) {
      if (f.locked) continue
      f.value = ''
      f.values = []
      f.checked = false
    }
  }

  const label = (f: PdfFormField) => f.name.split('.').pop() || f.name

  const blocked = $derived.by(() => {
    if (!doc) return L('Buka PDF dulu.', 'Open a PDF first.')
    if (!fields.length) return L('PDF ini tidak punya kolom formulir.', 'This PDF has no form fields.')
    return ''
  })

  async function start() {
    if (!doc) return
    try {
      const ref = await api.startPdfForm({ id: 'doc', path: doc.path, password: doc.password }, $state.snapshot(fields) as PdfFormField[], flatten)
      single[tool.id] = ref.taskId
      trackBatch('pdf', [ref.taskId])
    } catch (e) {
      toast(errText(e), 'err')
    }
  }
</script>

<div class="body">
  {#if !doc}
    <OpenDocView
      title={L('Buka formulir PDF', 'Open a PDF form')}
      subtitle={L('Isi kolom, centang kotak, dan pilih opsi tanpa Adobe Acrobat. Formulir hasil scan tidak punya kolom: pakai Edit PDF untuk menulis di atasnya.', "Fill fields, tick boxes and choose options without Adobe Acrobat. Scanned forms have no fields: use Edit PDF to write on them.")}
      busy={opening}
      onpick={() => pick()}
    />
  {:else}
    <section class="card main">
      <div class="toolbar">
        <div class="count">
          <span class="n ellipsis" title={doc.path}>{doc.name}</span>
          <span class="size">{fields.length} {L('kolom', fields.length === 1 ? 'field' : 'fields')}</span>
        </div>
        {#if editable}
          <button class="btn" onclick={clearAll}><Icon name="undo" size={16} />{L('Kosongkan', 'Clear')}</button>
        {/if}
        <button class="btn" onclick={() => pick()} disabled={opening}><Icon name={opening ? 'loader' : 'folder'} size={16} class={opening ? 'spin' : ''} />{L('Ganti PDF', 'Change PDF')}</button>
      </div>
      <div class="content">
        <div class="edit">
          {#if fields.length === 0}
            <div class="none">
              <Icon name="info" size={28} />
              <b>{L('Tidak ada kolom formulir', 'No form fields')}</b>
              <p class="hint">{L('PDF ini tidak punya kolom isian (mungkin hasil scan atau dicetak ke PDF). Pakai alat "Edit PDF" untuk menambahkan teks di atas halaman.', 'This PDF has no fillable fields (it may be scanned or printed to PDF). Use the "Edit PDF" tool to add text on the page.')}</p>
            </div>
          {/if}
          {#each groups as [p, list] (p)}
            <div class="group">
              {#if p}<button class="ghead" onclick={() => (page = p)}>{L('Halaman', 'Page')} {p}</button>{/if}
              {#each list as f (f.id)}
                <div class="field" class:locked={f.locked} role="group" aria-label={label(f)} onfocusin={() => f.page && (page = f.page)}>
                  {#if f.kind === 'check'}
                    <label class="check">
                      <input type="checkbox" bind:checked={f.checked} disabled={f.locked} />
                      <span class="ellipsis" title={f.name}>{label(f)}</span>
                    </label>
                  {:else}
                    <span class="flabel ellipsis" title={f.name}>{label(f)}{#if f.locked} <Icon name="lock" size={12} />{/if}</span>
                    {#if f.kind === 'text' && f.multiline}
                      <textarea class="text-input area" bind:value={f.value} disabled={f.locked} rows="3"></textarea>
                    {:else if f.kind === 'text'}
                      <input class="text-input" bind:value={f.value} disabled={f.locked} />
                    {:else if f.kind === 'date'}
                      <input class="text-input" bind:value={f.value} disabled={f.locked} placeholder={f.format || 'yyyy-mm-dd'} />
                    {:else if f.kind === 'radio'}
                      <div class="opts">
                        {#each f.options ?? [] as o}
                          <label class="opt"><input type="radio" name={f.id} value={o} bind:group={f.value} disabled={f.locked} />{o}</label>
                        {/each}
                      </div>
                    {:else if f.kind === 'list' && f.multi}
                      <div class="opts">
                        {#each f.options ?? [] as o}
                          <label class="opt"><input type="checkbox" checked={(f.values ?? []).includes(o)} disabled={f.locked} onchange={(e) => toggleValue(f, o, e.currentTarget.checked)} />{o}</label>
                        {/each}
                      </div>
                    {:else}
                      <select class="text-input" bind:value={f.value} disabled={f.locked}>
                        <option value="">—</option>
                        {#each f.options ?? [] as o}<option value={o}>{o}</option>{/each}
                      </select>
                    {/if}
                  {/if}
                </div>
              {/each}
            </div>
          {/each}
        </div>
        {#if fields.length}
          <div class="pv">
            <span class="label">{L('Halaman', 'Page')} {page}</span>
            {#if doc.info.pages[page - 1]}
              {@const pg = doc.info.pages[page - 1]}
              <div class="thumb" style="aspect-ratio: {pg.w} / {pg.h}">
                <img src={pageUrl(doc.path, page - 1, 360)} alt="{L('Halaman', 'Page')} {page}" />
              </div>
            {/if}
            <p class="hint">{L('Pratinjau halaman asli; isian tampil di file hasil.', 'Preview of the original page; your entries appear in the result.')}</p>
          </div>
        {/if}
      </div>
    </section>
  {/if}

  <SinglePanel {tool} startLabel={flatten ? L('Isi & ratakan', 'Fill & flatten') : L('Simpan formulir', 'Save the form')} {blocked} onstart={start}>
    <Switch
      bind:checked={flatten}
      label={L('Ratakan setelah diisi', 'Flatten after filling')}
      hint={L('Isian jadi bagian halaman: tampil sama di semua aplikasi dan tidak bisa diubah lagi. Cocok sebelum dikirim.', 'Entries become part of the page: they look the same everywhere and can no longer be changed. Good before sending.')}
    />
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
    gap: 18px;
  }
  .none {
    margin: auto;
    max-width: 380px;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    text-align: center;
    color: var(--text-2);
  }
  .group {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .ghead {
    align-self: flex-start;
    padding: 0;
    border: 0;
    background: none;
    font-size: 12px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--text-3);
  }
  .field {
    display: grid;
    grid-template-columns: 180px minmax(0, 1fr);
    gap: 12px;
    align-items: center;
  }
  .field.locked {
    opacity: 0.6;
  }
  .flabel {
    font-size: 13px;
    font-weight: 600;
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .check {
    grid-column: 1 / -1;
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    font-weight: 600;
    min-width: 0;
  }
  .check input,
  .opt input {
    accent-color: var(--accent);
  }
  .area {
    height: auto;
    padding: 8px 10px;
    resize: vertical;
    font: inherit;
  }
  .opts {
    display: flex;
    flex-wrap: wrap;
    gap: 6px 14px;
  }
  .opt {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
  }
  .pv {
    width: 190px;
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
</style>
