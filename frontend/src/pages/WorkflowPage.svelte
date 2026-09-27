<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import { onDestroy, onMount } from 'svelte'
  import PageHeader from '../components/PageHeader.svelte'
  import Select from '../components/Select.svelte'
  import Icon from '../components/Icon.svelte'
  import { api, errText } from '../lib/api'
  import { saveSettings, settings, toast } from '../lib/stores/app.svelte'
  import { flowDrop, flowResults } from '../lib/stores/flows.svelte'
  import { pdfOpts } from '../lib/stores/pdf.svelte'
  import { userPresets, type Preset } from '../lib/stores/presets.svelte'
  import { describeJob, moduleBuiltins, moduleJob, presetState, type ModuleKind } from '../lib/modules'
  import { pdfTools } from '../lib/pdfTools'
  import type { FlowStep, Workflow } from '../lib/types'

  type StepKind = FlowStep['kind']

  const flows = $derived(settings.value?.workflows ?? [])
  let selected = $state<string>('')
  let draft = $state<Workflow | null>(null)
  let busy = $state(false)

  const kindName = $derived<Record<StepKind, string>>({ image: L('Gambar', 'Image'), video: 'Video', audio: 'Audio', subtitle: 'Subtitle', pdf: 'PDF' })
  const kindIcon: Record<StepKind, string> = { image: 'image', video: 'video', audio: 'music', subtitle: 'subtitles', pdf: 'fileText' }

  // PDF tools that work file by file without asking anything.
  const flowPdfTools = $derived(pdfTools.filter((t) => t.pattern === 'batch' && t.input === 'pdf' && !['digisign', 'pdfacheck', 'unlock'].includes(t.id)))

  function presetsOf(kind: ModuleKind): Preset[] {
    return [
      { id: 'current', name: L('Pengaturan halaman sekarang', 'Current page settings'), value: {} },
      ...moduleBuiltins(kind).map((b) => ({ ...b, builtin: true })),
      ...userPresets(kind),
    ]
  }

  /** A step from a module preset (or a PDF tool with the current PDF settings). */
  function makeStep(kind: StepKind, id: string): FlowStep {
    if (kind === 'pdf') {
      const tool = flowPdfTools.find((t) => t.id === id) ?? flowPdfTools[0]
      return { kind, tool: tool.id, preset: '', label: tool.name(), job: $state.snapshot(pdfOpts) }
    }
    const p = presetsOf(kind).find((x) => x.id === id) ?? presetsOf(kind)[0]
    return { kind, tool: '', preset: p.id, label: p.name, job: moduleJob(kind, presetState(kind, p.id === 'current' ? null : p)) }
  }

  function stepText(s: FlowStep): string {
    if (s.kind === 'pdf') return `PDF · ${pdfTools.find((t) => t.id === s.tool)?.name() ?? s.tool}`
    return describeJob(s.kind, s.job)
  }

  // Ready-made chains.
  const templates = $derived([
    {
      name: L('Video → lagu MP3 → volume rata', 'Video → MP3 song → even volume'),
      steps: () => [makeStep('video', 'b-mp3'), { ...makeStep('audio', 'current'), preset: 'custom', label: L('MP3 · volume rata', 'MP3 · even volume'), job: moduleJob('audio', presetState('audio', { id: 'x', name: '', value: { a: { format: 'mp3', bitrate: 192, normalize: true, loudness: -14 } } })) }],
    },
    {
      name: L('Foto produk → tanpa latar → WEBP ringan', 'Product photo → no background → light WEBP'),
      steps: () => [makeStep('image', 'b-product'), makeStep('image', 'b-web')],
    },
    {
      name: L('Video → subtitle → video bersubtitle', 'Video → subtitles → subtitled video'),
      steps: () => [makeStep('subtitle', 'b-burn')],
    },
    {
      name: L('PDF scan → OCR → kompres', 'Scanned PDF → OCR → compress'),
      steps: () => [makeStep('pdf', 'ocr'), makeStep('pdf', 'compress')],
    },
  ])

  function edit(w: Workflow) {
    selected = w.id
    draft = structuredClone($state.snapshot(w)) as Workflow
  }

  function create(name = '', steps: FlowStep[] = []) {
    draft = { id: `wf${Date.now()}`, name: name || L('Alur kerja baru', 'New workflow'), steps: steps.length ? steps : [makeStep('video', 'b-mp3')] }
    selected = draft.id
  }

  async function saveDraft() {
    if (!draft) return
    const name = draft.name.trim()
    if (!name) return toast(L('Beri nama alur kerja', 'Give the workflow a name'), 'info')
    if (!draft.steps.length) return toast(L('Tambahkan minimal satu langkah', 'Add at least one step'), 'info')
    const w = { ...$state.snapshot(draft), name } as Workflow
    const list = flows.some((f) => f.id === w.id) ? flows.map((f) => (f.id === w.id ? w : f)) : [...flows, w]
    busy = true
    try {
      await saveSettings({ workflows: list })
      toast(L(`Alur kerja "${name}" disimpan`, `Workflow "${name}" saved`), 'ok')
    } finally {
      busy = false
    }
  }

  async function remove(w: Workflow) {
    await saveSettings({ workflows: flows.filter((f) => f.id !== w.id) })
    if (selected === w.id) {
      selected = ''
      draft = null
    }
  }

  function setKind(i: number, kind: StepKind) {
    if (!draft) return
    draft.steps[i] = makeStep(kind, kind === 'pdf' ? 'compress' : 'current')
  }
  function setPreset(i: number, id: string) {
    if (!draft) return
    draft.steps[i] = makeStep(draft.steps[i].kind, id)
  }
  function move(i: number, d: number) {
    if (!draft) return
    const j = i + d
    if (j < 0 || j >= draft.steps.length) return
    const s = draft.steps
    ;[s[i], s[j]] = [s[j], s[i]]
  }

  const dirty = $derived.by(() => {
    if (!draft) return false
    const saved = flows.find((f) => f.id === draft!.id)
    return !saved || JSON.stringify(saved) !== JSON.stringify($state.snapshot(draft))
  })

  async function run(w: Workflow, paths?: string[]) {
    if (!w.steps.length) return
    try {
      if (!paths) {
        const first = w.steps[0].kind
        const list = await api.pickFiles(first === 'subtitle' ? 'subtitle' : first)
        paths = list.filter((x) => !x.error).map((x) => x.path)
        if (!paths.length) return
      }
      const n = await api.runWorkflow(w.id, paths)
      toast(L(`${n} file masuk alur kerja "${w.name}"`, `${n} file${n === 1 ? '' : 's'} sent through "${w.name}"`), 'ok')
    } catch (e) {
      toast(errText(e), 'err')
    }
  }

  onMount(() => {
    flowDrop.fn = (paths) => {
      const w = flows.find((f) => f.id === selected) ?? (flows.length === 1 ? flows[0] : undefined)
      if (!w) return toast(L('Pilih alur kerja dulu, lalu tarik file ke sini.', 'Choose a workflow first, then drop the files here.'), 'info')
      if (dirty) return toast(L('Simpan perubahan alur kerja dulu.', 'Save the workflow changes first.'), 'info')
      run(w, paths)
    }
  })
  onDestroy(() => (flowDrop.fn = null))

  const base = (p: string) => p.split(/[\\/]/).pop() || p
</script>

<PageHeader title={L('Alur kerja', 'Workflows')} subtitle={L('Rangkai beberapa langkah jadi satu klik: hasil tiap langkah otomatis lanjut ke langkah berikutnya.', 'Chain steps into one click: the result of each step goes straight into the next.')} />

<div class="body">
  <section class="card main" aria-label={L('Daftar alur kerja', 'Workflow list')}>
    <div class="toolbar">
      <div class="count">
        <span class="n">{flows.length} {L('alur kerja', flows.length === 1 ? 'workflow' : 'workflows')}</span>
      </div>
      <button class="btn-accent" onclick={() => create()}><Icon name="plus" size={16} stroke={2.5} />{L('Buat alur kerja', 'New workflow')}</button>
    </div>
    <div class="scroll">
      {#if flows.length === 0}
        <div class="intro">
          <Icon name="layers" size={30} />
          <b>{L('Belum ada alur kerja', 'No workflows yet')}</b>
          <p class="hint">{L('Mulai dari contoh di bawah, lalu ubah langkahnya sesuai kebutuhan.', 'Start from an example below, then adjust the steps.')}</p>
        </div>
      {/if}
      {#each flows as w (w.id)}
        <div class="flow" class:on={w.id === selected} role="button" tabindex="0" onclick={() => edit(w)} onkeydown={(e) => e.key === 'Enter' && edit(w)}>
          <div class="fh">
            <b class="ellipsis">{w.name}</b>
            <div class="acts">
              <button class="btn small" onclick={(e) => (e.stopPropagation(), run(w))}><Icon name="play" size={14} />{L('Jalankan…', 'Run…')}</button>
              <button class="btn icon small" title={L('Hapus', 'Delete')} aria-label={L('Hapus', 'Delete')} onclick={(e) => (e.stopPropagation(), remove(w))}><Icon name="trash" size={14} /></button>
            </div>
          </div>
          <div class="chain">
            {#each w.steps as s, i}
              {#if i > 0}<Icon name="chevronRight" size={14} />{/if}
              <span class="chip"><Icon name={kindIcon[s.kind]} size={13} />{stepText(s)}</span>
            {/each}
          </div>
        </div>
      {/each}

      <div class="sec">
        <span class="label">{L('Contoh siap pakai', 'Ready-made examples')}</span>
        <div class="tpls">
          {#each templates as t}
            <button class="tpl" onclick={() => create(t.name, t.steps())}><Icon name="plus" size={14} />{t.name}</button>
          {/each}
        </div>
      </div>

      {#if flowResults.list.length}
        <div class="sec">
          <span class="label">{L('Hasil terakhir', 'Recent results')}</span>
          {#each flowResults.list.slice(0, 12) as r (r.at + r.input)}
            <div class="res" class:err={!!r.error}>
              <Icon name={r.error ? 'alert' : 'check'} size={14} />
              <span class="ellipsis" title={r.input}>{base(r.input || r.workflow)}</span>
              <span class="rw ellipsis">{r.error || base(r.output)}</span>
              {#if r.output}<button class="link" onclick={() => api.revealFile(r.output)}>{L('Lihat', 'Show')}</button>{/if}
            </div>
          {/each}
        </div>
      {/if}
      <p class="hint">{L('Tips: pilih alur kerja lalu tarik file ke jendela ini. Alur kerja juga bisa dipakai di halaman Download (setelah unduh) dan di folder pantauan (Pengaturan › Proses & otomatisasi).', 'Tip: select a workflow and drop files on this window. Workflows also work on the Download page (after downloading) and in watched folders (Settings › Processing & automation).')}</p>
    </div>
  </section>

  <aside class="card panel" aria-label={L('Ubah alur kerja', 'Edit workflow')}>
    {#if draft}
      <div class="scroll">
        <div class="sec tight">
          <label class="label" for="wf-name">{L('Nama', 'Name')}</label>
          <input id="wf-name" class="text-input" bind:value={draft.name} />
        </div>
        <div class="sec">
          <span class="label">{L('Langkah', 'Steps')}</span>
          {#each draft.steps as s, i (i)}
            <div class="step">
              <div class="sh">
                <span class="num">{i + 1}</span>
                <div class="grow">
                  <Select
                    label={L('Modul', 'Module')}
                    value={s.kind}
                    onchange={(v) => setKind(i, v as StepKind)}
                    options={(['video', 'audio', 'image', 'subtitle', 'pdf'] as StepKind[]).map((k) => ({ value: k, label: kindName[k] }))}
                  />
                </div>
                <button class="btn icon xs" title={L('Naik', 'Up')} aria-label={L('Naik', 'Up')} disabled={i === 0} onclick={() => move(i, -1)}><Icon name="arrowUp" size={13} /></button>
                <button class="btn icon xs" title={L('Turun', 'Down')} aria-label={L('Turun', 'Down')} disabled={i === draft.steps.length - 1} onclick={() => move(i, 1)}><Icon name="arrowDown" size={13} /></button>
                <button class="btn icon xs" title={L('Hapus langkah', 'Remove step')} aria-label={L('Hapus langkah', 'Remove step')} onclick={() => draft && draft.steps.splice(i, 1)}><Icon name="trash" size={13} /></button>
              </div>
              {#if s.kind === 'pdf'}
                <Select label={L('Alat PDF', 'PDF tool')} value={s.tool} onchange={(v) => setPreset(i, v)} options={flowPdfTools.map((t) => ({ value: t.id, label: t.name() }))} />
                <p class="hint">{L('Memakai pengaturan alat PDF saat langkah ini dipilih.', 'Uses the PDF tool settings from when this step was chosen.')}</p>
              {:else}
                <Select label={L('Preset', 'Preset')} value={s.preset} onchange={(v) => setPreset(i, v)} options={[...(presetsOf(s.kind).some((p) => p.id === s.preset) ? [] : [{ value: s.preset, label: s.label }]), ...presetsOf(s.kind).map((p) => ({ value: p.id, label: p.name }))]} />
                <p class="hint">{stepText(s)}</p>
              {/if}
            </div>
          {/each}
          <button class="btn" onclick={() => draft && draft.steps.push(makeStep(draft.steps.at(-1)?.kind === 'video' ? 'audio' : 'video', 'current'))}><Icon name="plus" size={16} />{L('Tambah langkah', 'Add step')}</button>
        </div>
        <div class="note"><Icon name="info" size={16} /><span>{L('Hasil antara disimpan sementara lalu dihapus. Hasil akhir masuk ke folder hasil modul langkah terakhir (atau di samping file asli bila diatur "folder yang sama").', 'In-between results are temporary and removed afterwards. The final result goes to the output folder of the last step (or next to the original when set to "same folder").')}</span></div>
      </div>
      <div class="foot">
        <button class="btn" disabled={!dirty} onclick={() => { const w = flows.find((f) => f.id === draft?.id); if (w) edit(w); else { draft = null; selected = '' } }}>{L('Batal', 'Cancel')}</button>
        <button class="btn-accent" disabled={busy || !dirty} onclick={saveDraft}><Icon name="check" size={16} stroke={2.5} />{L('Simpan', 'Save')}</button>
      </div>
    {:else}
      <div class="scroll empty">
        <Icon name="layers" size={28} />
        <p class="hint">{L('Pilih alur kerja untuk mengubah langkahnya, atau buat yang baru.', 'Select a workflow to edit its steps, or make a new one.')}</p>
      </div>
    {/if}
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
  }
  .n {
    font-size: 15px;
    font-weight: 700;
  }
  .scroll {
    flex-grow: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .panel .scroll {
    padding: 18px 20px;
    gap: 16px;
  }
  .intro,
  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    text-align: center;
    color: var(--text-2);
    padding: 12px 0;
  }
  .empty {
    justify-content: center;
  }
  .flow {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 12px 14px;
    border: 1px solid var(--border);
    border-radius: 12px;
    cursor: pointer;
  }
  .flow:hover {
    border-color: var(--border-strong);
  }
  .flow.on {
    border-color: var(--accent);
    background: var(--accent-tint);
  }
  .fh {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .fh b {
    flex-grow: 1;
    font-size: 14px;
  }
  .acts {
    display: flex;
    gap: 6px;
  }
  .btn.small {
    height: 30px;
    padding: 0 10px;
    font-size: 12px;
  }
  .btn.icon.small {
    width: 30px;
    padding: 0;
  }
  .btn.icon.xs {
    width: 28px;
    height: 28px;
    padding: 0;
    flex-shrink: 0;
  }
  .chain {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
    color: var(--text-3);
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 4px 9px;
    border-radius: 999px;
    background: var(--canvas);
    color: var(--text-1);
    font-size: 12px;
    font-weight: 600;
  }
  .sec {
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin-top: 6px;
  }
  .sec.tight {
    gap: 8px;
    margin-top: 0;
  }
  .tpls {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }
  .tpl {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 12px;
    border: 1px dashed var(--border-strong);
    border-radius: 10px;
    background: none;
    color: inherit;
    font-size: 13px;
    font-weight: 600;
    text-align: left;
  }
  .tpl:hover {
    border-color: var(--accent);
    color: var(--accent);
  }
  .res {
    display: grid;
    grid-template-columns: 16px minmax(0, 1fr) minmax(0, 1fr) auto;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    color: var(--ok);
  }
  .res.err {
    color: var(--err);
  }
  .res span {
    color: var(--text-1);
  }
  .res .rw {
    color: var(--text-3);
  }
  .link {
    padding: 0;
    border: 0;
    background: none;
    font-size: 12px;
    font-weight: 700;
    color: var(--accent);
    text-decoration: underline;
  }
  .panel {
    width: 344px;
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .step {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 10px;
    border-radius: 10px;
    background: var(--canvas);
  }
  .sh {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .grow {
    flex-grow: 1;
    min-width: 0;
  }
  .num {
    width: 22px;
    height: 22px;
    flex-shrink: 0;
    border-radius: 50%;
    background: var(--accent);
    color: #fff;
    font-size: 12px;
    font-weight: 800;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-right: 4px;
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
  .foot {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding: 14px 20px;
    border-top: 1px solid var(--border);
  }
</style>
