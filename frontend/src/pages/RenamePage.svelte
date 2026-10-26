<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import { onDestroy } from 'svelte'
  import PageHeader from '../components/PageHeader.svelte'
  import EmptyDrop from '../components/EmptyDrop.svelte'
  import Segmented from '../components/Segmented.svelte'
  import Chips from '../components/Chips.svelte'
  import Switch from '../components/Switch.svelte'
  import Icon from '../components/Icon.svelte'
  import { api, errText, imageUrl, pageUrl } from '../lib/api'
  import { shortPath, toast } from '../lib/stores/app.svelte'
  import { askPassword } from '../lib/stores/prompt.svelte'
  import { persistRename, renameConv as conv, renameOpts as o, renameState as st } from '../lib/stores/rename.svelte'
  import { bytes, tile } from '../lib/format'
  import type { FileItem, RenameMode, RenameResult } from '../lib/types'

  const isDoc = (it: FileItem) => /^(pdf|docx?|docm|dotx?|odt|rtf|wpd)$/.test(it.ext)
  const isImage = (it: FileItem) => /^(jpe?g|jfif|png|webp|gif|bmp|tiff?|heic|heif|avif)$/.test(it.ext)
  const stem = (name: string) => (name.includes('.') ? name.slice(0, name.lastIndexOf('.')) : name)
  const extOf = (name: string) => (name.includes('.') ? name.slice(name.lastIndexOf('.')) : '')
  const dirOf = (p: string) => p.slice(0, Math.max(p.lastIndexOf('\\'), p.lastIndexOf('/')))
  const baseOf = (p: string) => p.split(/[\\/]/).pop() || p

  // ---- preview: proposed names, read again (debounced) whenever the list or an option changes ----
  let scanning = $state(false)
  let timer: ReturnType<typeof setTimeout> | undefined
  let seq = 0
  onDestroy(() => clearTimeout(timer))

  $effect(() => {
    const items = conv.items.map((it) => ({ id: it.id, path: it.path, password: conv.passwords[it.id] ?? '' }))
    const opts = $state.snapshot(o)
    clearTimeout(timer)
    if (items.length === 0) {
      scanning = false
      return
    }
    scanning = true
    const my = ++seq
    timer = setTimeout(async () => {
      try {
        const res = await api.renamePreview(items, opts)
        if (my !== seq) return
        const next: typeof st.plans = {}
        for (const p of res ?? []) next[p.id] = p
        st.plans = next
      } catch (e) {
        if (my === seq) toast(errText(e), 'err')
      } finally {
        if (my === seq) scanning = false
      }
    }, o.mode === 'content' ? 350 : 150)
  })

  const newName = (it: FileItem) => (st.edits[it.id] ?? st.plans[it.id]?.name ?? '').trim()
  const newExt = (it: FileItem) => st.plans[it.id]?.ext ?? extOf(it.name)
  // A twin this page just numbered ("Budi (1).pdf" for "Budi") counts as done.
  const twinOf = (cur: string, want: string) => cur.startsWith(`${want} (`) && /^ \(\d+\)$/.test(cur.slice(want.length))
  const changes = (it: FileItem) => {
    const n = newName(it)
    if (!n) return false
    if (st.copy) return true
    if (n + newExt(it) === it.name) return false
    const r = st.results[it.id]
    return !(r && !r.copy && r.to === it.path && newExt(it) === extOf(it.name) && twinOf(stem(it.name), n))
  }

  // Files that would end up with the same name in the same folder: the next ones get " (1)", …
  const dupes = $derived.by(() => {
    const seen = new Map<string, number>()
    for (const it of conv.items) {
      if (!newName(it)) continue
      const k = `${st.copy ? '' : dirOf(it.path)}|${newName(it)}${newExt(it)}`.toLowerCase()
      seen.set(k, (seen.get(k) ?? 0) + 1)
    }
    return seen
  })
  const isDupe = (it: FileItem) => (dupes.get(`${st.copy ? '' : dirOf(it.path)}|${newName(it)}${newExt(it)}`.toLowerCase()) ?? 0) > 1

  const todo = $derived(conv.items.filter(changes))
  const problems = $derived(conv.items.filter((it) => !st.edits[it.id] && st.plans[it.id]?.error).length)

  const blocked = $derived.by(() => {
    if (conv.items.length === 0) return L('Tambahkan file dulu untuk memulai.', 'Add files to get started.')
    if (o.mode === 'pattern' && !o.pattern.trim()) return L('Isi pola nama.', 'Enter a name pattern.')
    if (o.mode === 'replace' && !o.find) return L('Isi teks yang dicari.', 'Enter the text to find.')
    if (st.copy && !st.copyDir) return L('Pilih folder tujuan salinan.', 'Choose the folder for the copies.')
    if (todo.length === 0) {
      if (scanning) return L('Sedang menyusun nama baru…', 'Working out the new names…')
      return problems ? L('Belum ada nama baru — lihat keterangan merah di daftar.', 'No new names yet — see the notes in red in the list.') : L('Semua file sudah bernama sesuai.', 'Every file already has this name.')
    }
    return ''
  })

  let running = $state(false)
  let width = $state(0)

  async function start() {
    if (blocked || running) return
    running = true
    persistRename()
    try {
      const items = todo
      const res = await api.renameApply(
        items.map((it) => ({ id: it.id, path: it.path, name: newName(it), ext: newExt(it) })),
        st.copy ? st.copyDir : '',
      )
      apply(res)
      st.last = res.filter((r) => r.to && !r.error)
      const ok = st.last.length
      const failed = res.filter((r) => r.error && !r.skipped).length
      toast(
        failed
          ? L(`${ok} file berhasil, ${failed} gagal`, `${ok} files done, ${failed} failed`)
          : st.copy
            ? L(`${ok} file disalin dengan nama baru`, `${ok} files copied under new names`)
            : L(`${ok} file diganti namanya`, `${ok} files renamed`),
        failed ? 'err' : 'ok',
      )
    } catch (e) {
      toast(errText(e), 'err')
    } finally {
      running = false
    }
  }

  // Renamed items take their new path; typed names are done with.
  function apply(res: RenameResult[]) {
    const byId = new Map(res.map((r) => [r.id, r]))
    for (const it of conv.items) {
      const r = byId.get(it.id)
      if (!r) continue
      st.results[it.id] = r
      if (r.to && !r.error && !r.copy) {
        it.path = r.to
        it.name = baseOf(r.to)
        it.ext = extOf(it.name).slice(1).toLowerCase()
      }
      if (r.to && !r.error) delete st.edits[it.id]
    }
  }

  async function undo() {
    running = true
    try {
      const res = await api.renameUndo($state.snapshot(st.last))
      for (const r of res) {
        const it = conv.items.find((x) => x.id === r.id)
        if (it && r.to && !r.error && !r.copy) {
          it.path = r.to
          it.name = baseOf(r.to)
          it.ext = extOf(it.name).slice(1).toLowerCase()
        }
        delete st.results[r.id]
      }
      const failed = res.filter((r) => r.error).length
      toast(failed ? L(`${failed} file tidak bisa dikembalikan`, `${failed} files couldn't be restored`) : L('Dikembalikan seperti semula', 'Restored'), failed ? 'err' : 'ok')
      st.last = []
    } catch (e) {
      toast(errText(e), 'err')
    } finally {
      running = false
    }
  }

  async function chooseDir() {
    const d = await api.pickDirectory(L('Pilih folder tujuan salinan', 'Choose the folder for the copies'), st.copyDir)
    if (d) st.copyDir = d
  }

  async function setPassword(it: FileItem) {
    const pw = await askPassword(it.name)
    if (pw) conv.passwords[it.id] = pw
  }

  function sortByName() {
    conv.items = [...conv.items].sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' }))
  }

  function remove(it: FileItem) {
    conv.remove(it)
    delete st.edits[it.id]
    delete st.results[it.id]
  }

  function clear() {
    conv.clear()
    st.edits = {}
    st.results = {}
    st.last = []
  }

  function insert(field: 'template' | 'pattern' | 'prefix' | 'suffix', code: string) {
    const t = o[field]
    o[field] = t && !t.endsWith(' ') && field !== 'prefix' ? `${t} ${code}` : `${t}${code}`
  }

  const modes = $derived<{ value: RenameMode; label: string }[]>([
    { value: 'pattern', label: L('Pola & nomor', 'Pattern & number') },
    { value: 'replace', label: L('Cari & ganti', 'Find & replace') },
    { value: 'edit', label: L('Tambah / potong', 'Add / cut') },
    { value: 'content', label: L('Dari isi PDF/Word', 'From PDF/Word text') },
  ])
</script>

<PageHeader
  title={L('Ganti nama massal', 'Bulk rename')}
  subtitle={L('Ganti nama banyak file sekaligus: pola & nomor urut, cari & ganti, tambah teks, atau dari isi PDF/Word (mis. nama siswa di rapor).', 'Rename many files at once: pattern & number, find & replace, added text, or from PDF/Word text (e.g. the student name on report cards).')}
/>

<div class="body">
  <div class="left">
    {#if conv.items.length === 0}
      <EmptyDrop
        {conv}
        title={L('Tarik & lepas file atau folder ke sini', 'Drag & drop files or folders here')}
        subtitle={L('File jenis apa saja. Nama baru tampil dulu sebelum diganti, dan bisa diurungkan.', 'Any kind of file. New names are shown before anything changes, and it can be undone.')}
        pickLabel={L('Pilih file', 'Choose files')}
        formats={[L('Semua file', 'All files')]}
        steps={[
          [L('Tambahkan file', 'Add files'), L('Bisa satu folder penuh', 'A whole folder works too')],
          [L('Pilih cara memberi nama', 'Choose how to name them'), L('Periksa & ubah di daftar', 'Check & edit them in the list')],
          [L('Klik Ganti nama', 'Click Rename'), L('Nama kembar diberi nomor', 'Duplicate names get a number')],
        ]}
      />
    {:else}
      <section class="card list" class:compact={width > 0 && width < 640} bind:clientWidth={width} aria-label={L('Daftar file', 'File list')}>
        <div class="toolbar">
          <div class="count">
            <span class="n">{conv.items.length} file</span>
            <span class="size">
              {#if scanning}<Icon name="loader" size={12} class="spin" /> {L('Menyusun nama…', 'Working…')}
              {:else}{todo.length} {L('akan diganti', 'to rename')}{#if problems} · <span class="bad">{problems} {L('bermasalah', 'with problems')}</span>{/if}{/if}
            </span>
          </div>
          <button class="btn" onclick={() => conv.pickFiles()} disabled={conv.adding} title={L('Tambah file', 'Add files')}><Icon name="plus" size={16} /><span class="bl">{L('Tambah file', 'Add files')}</span></button>
          <button class="btn" onclick={() => conv.pickFolder()} disabled={conv.adding} title={L('Tambah folder', 'Add folder')}><Icon name="folder" size={16} /><span class="bl">{L('Tambah folder', 'Add folder')}</span></button>
          <button class="btn icon" title={L('Urutkan menurut nama (untuk nomor urut)', 'Sort by name (for numbering)')} aria-label={L('Urutkan menurut nama', 'Sort by name')} onclick={sortByName}><Icon name="arrowDown" size={16} /></button>
          <button class="btn icon" aria-label={L('Kosongkan daftar', 'Clear list')} title={L('Kosongkan daftar', 'Clear list')} onclick={clear}><Icon name="trash" size={16} /></button>
        </div>
        <div class="head">
          <span>{L('NAMA SEKARANG', 'CURRENT NAME')}</span><span></span><span>{L('NAMA BARU', 'NEW NAME')}</span><span></span>
        </div>
        <div class="rows">
          {#each conv.items as it, i (it.id)}
            {@const plan = st.plans[it.id]}
            {@const res = st.results[it.id]}
            <div class="row" class:same={!changes(it) && !!newName(it)}>
              <div class="old">
                {#if it.ext === 'pdf' && !it.locked}
                  <img class="th page" src={pageUrl(it.path, 0, 80)} alt="" loading="lazy" />
                {:else if isImage(it)}
                  <img class="th" src={imageUrl(it.path, 80)} alt="" loading="lazy" />
                {:else}
                  {@const t = tile(it.name)}
                  <div class="th tl" style="background: {t.bg}; color: {t.fg}"><Icon name={it.locked ? 'lock' : isDoc(it) ? 'fileText' : 'file'} size={18} /></div>
                {/if}
                <div class="nm">
                  <span class="name ellipsis" title={it.path}>{it.name}</span>
                  <span class="meta ellipsis" title={dirOf(it.path)}>{i + 1}. {shortPath(dirOf(it.path))}{it.size ? ` · ${bytes(it.size)}` : ''}</span>
                </div>
              </div>
              <Icon name="chevronRight" size={16} class="arrow" />
              <div class="new">
                {#if it.locked && !conv.passwords[it.id] && o.mode === 'content'}
                  <button class="pw" onclick={() => setPassword(it)}><Icon name="lock" size={12} />{L('Isi password untuk membaca isinya', 'Enter the password to read it')}</button>
                {:else}
                  <div class="in">
                    <input
                      class="text-input"
                      value={newName(it)}
                      placeholder={plan ? L('Ketik nama baru', 'Type a new name') : L('Menyusun…', 'Working…')}
                      aria-label={L(`Nama baru untuk ${it.name}`, `New name for ${it.name}`)}
                      oninput={(e) => (st.edits[it.id] = e.currentTarget.value)}
                    />
                    <span class="ext">{newExt(it)}</span>
                  </div>
                  {#if res?.error && !res.skipped}
                    <span class="note bad ellipsis" title={res.error}>{res.error}</span>
                  {:else if res?.to && !res.error}
                    <span class="note ok ellipsis" title={res.to}><Icon name="check" size={11} stroke={3} /> {res.copy ? L('Disalin', 'Copied') : L('Diganti', 'Renamed')}: {baseOf(res.to)}</span>
                  {:else if st.edits[it.id] !== undefined && st.edits[it.id] !== (plan?.name ?? '')}
                    <span class="note">{L('Diubah manual', 'Edited by hand')} · <button class="link" onclick={() => delete st.edits[it.id]}>{L('Kembalikan', 'Reset')}</button></span>
                  {:else if plan?.error}
                    <span class="note bad ellipsis" title={plan.error}>{plan.error}</span>
                  {:else if newName(it) && !changes(it)}
                    <span class="note">{L('Nama sudah sesuai', 'Already has this name')}</span>
                  {:else if isDupe(it)}
                    <span class="note warn">{L('Nama kembar — akan diberi nomor', 'Same name — a number will be added')}</span>
                  {/if}
                {/if}
              </div>
              <button class="mini" aria-label={L(`Hapus ${it.name} dari daftar`, `Remove ${it.name} from the list`)} title={L('Hapus dari daftar', 'Remove from list')} onclick={() => remove(it)}><Icon name="x" size={16} /></button>
            </div>
          {/each}
        </div>
      </section>
    {/if}
  </div>

  <aside class="card panel" aria-label={L('Pengaturan', 'Settings')}>
    <div class="scroll">
      <div class="sec">
        <span class="label">{L('Cara memberi nama', 'How to name')}</span>
        <Chips columns={2} small bind:value={o.mode} options={modes} />
      </div>

      {#if o.mode === 'pattern'}
        <div class="sec">
          <label class="label" for="rn-pat">{L('Pola nama', 'Name pattern')}</label>
          <input id="rn-pat" class="text-input" bind:value={o.pattern} placeholder={L('mis. Liburan {n}', 'e.g. Holiday {n}')} />
          <div class="codes">
            {#each ['{n}', '{file}', '{tanggal}'] as c (c)}<button class="code" onclick={() => insert('pattern', c)}>{c}</button>{/each}
          </div>
          <p class="hint">{L('{n} nomor urut (sesuai urutan daftar), {file} nama lama, {tanggal} tanggal file diubah.', '{n} counter (in list order), {file} the old name, {tanggal} the date the file was changed.')}</p>
        </div>
        {@render counter()}
      {:else if o.mode === 'replace'}
        <div class="sec">
          <label class="label" for="rn-find">{L('Cari', 'Find')}</label>
          <input id="rn-find" class="text-input" bind:value={o.find} placeholder={L('mis. IMG_', 'e.g. IMG_')} />
          <label class="label" for="rn-with">{L('Ganti dengan', 'Replace with')}</label>
          <input id="rn-with" class="text-input" bind:value={o.with} placeholder={L('Kosongkan untuk menghapus', 'Leave empty to delete')} />
        </div>
        <Switch bind:checked={o.matchCase} label={L('Bedakan huruf besar/kecil', 'Match case')} />
        <Switch bind:checked={o.regex} label={L('Ekspresi reguler (regex)', 'Regular expression')} hint={L('Untuk pengguna mahir: $1, $2 di kolom ganti mengambil grup', 'For advanced users: $1, $2 in Replace insert the groups')} />
      {:else if o.mode === 'edit'}
        <div class="sec">
          <label class="label" for="rn-pre">{L('Tambah di depan', 'Add before')}</label>
          <input id="rn-pre" class="text-input" bind:value={o.prefix} placeholder={L('mis. 2026 - ', 'e.g. 2026 - ')} />
          <label class="label" for="rn-suf">{L('Tambah di belakang', 'Add after')}</label>
          <input id="rn-suf" class="text-input" bind:value={o.suffix} placeholder={L('mis.  (revisi)', 'e.g.  (final)')} />
          <div class="codes">
            {#each ['{n}', '{tanggal}'] as c (c)}<button class="code" onclick={() => insert(o.prefix ? 'suffix' : 'prefix', c)}>{c}</button>{/each}
          </div>
        </div>
        <div class="grid2">
          <div class="sec tight">
            <label class="label" for="rn-cs">{L('Hapus di depan', 'Cut at start')}</label>
            <input id="rn-cs" class="text-input" type="number" min="0" bind:value={o.cutStart} />
          </div>
          <div class="sec tight">
            <label class="label" for="rn-ce">{L('Hapus di belakang', 'Cut at end')}</label>
            <input id="rn-ce" class="text-input" type="number" min="0" bind:value={o.cutEnd} />
          </div>
        </div>
        <p class="hint">{L('Jumlah huruf yang dibuang dari nama lama.', 'Number of characters removed from the old name.')}</p>
        {#if o.prefix.includes('{n}') || o.suffix.includes('{n}')}{@render counter()}{/if}
      {:else}
        <div class="sec">
          <label class="label" for="rn-labels">{L('Label nama di dokumen', 'Name label in the document')}</label>
          <input id="rn-labels" class="text-input" bind:value={o.labels} placeholder="Nama Peserta Didik, Nama Siswa" />
          <p class="hint">{L('Teks setelah label dan titik dua dipakai sebagai nama, mis. “Nama Peserta Didik : BUDI”. Beberapa label dipisah koma, dicoba berurutan.', 'The text after the label and colon becomes the name, e.g. “Student Name : BUDI”. Separate several labels with commas; they are tried in order.')}</p>
        </div>
        <div class="sec">
          <label class="label" for="rn-tpl">{L('Format nama file', 'File name format')}</label>
          <input id="rn-tpl" class="text-input" bind:value={o.template} placeholder={'{nama}'} />
          <div class="codes">
            {#each ['{nama}', '{nisn}', '{nis}', '{kelas}', '{file}', '{n}'] as c (c)}<button class="code" onclick={() => insert('template', c)}>{c}</button>{/each}
          </div>
          <p class="hint">{L('Label lain juga bisa ditulis dalam kurung kurawal, mis. {Semester}. Hanya untuk PDF & Word; PDF hasil scan perlu OCR dulu.', 'Any other label works in braces too, e.g. {Semester}. PDF & Word only; scanned PDFs need OCR first.')}</p>
        </div>
      {/if}

      <div class="sec">
        <span class="label">{o.mode === 'content' ? L('Huruf nama', 'Name letters') : L('Huruf', 'Letters')}</span>
        <Chips
          columns={2}
          small
          bind:value={o.case}
          options={[
            { value: 'keep', label: L('Tetap', 'As is') },
            { value: 'title', label: L('Kapital Awal', 'Title Case') },
            { value: 'upper', label: L('KAPITAL', 'UPPERCASE') },
            { value: 'lower', label: L('huruf kecil', 'lowercase') },
          ]}
        />
      </div>
      <Switch bind:checked={o.tidy} label={L('Garis bawah jadi spasi', 'Underscores to spaces')} hint={L('“laporan_akhir” → “laporan akhir”', '“final_report” → “final report”')} />
      <Switch bind:checked={o.extLower} label={L('Ekstensi huruf kecil', 'Lowercase extension')} hint=".JPG → .jpg" />

      <div class="sec">
        <span class="label">{L('Cara menyimpan', 'How to save')}</span>
        <Segmented
          value={st.copy ? 'copy' : 'move'}
          onchange={(v) => (st.copy = v === 'copy')}
          label={L('Cara menyimpan', 'How to save')}
          options={[
            { value: 'move', label: L('Ganti nama asli', 'Rename originals') },
            { value: 'copy', label: L('Salin ke folder', 'Copy to folder') },
          ]}
        />
        {#if st.copy}
          <button class="btn dir" onclick={chooseDir} title={st.copyDir}><Icon name="folder" size={16} /><span class="ellipsis">{st.copyDir ? shortPath(st.copyDir) : L('Pilih folder tujuan…', 'Choose a folder…')}</span></button>
          <p class="hint">{L('File asli tetap; salinan bernama baru disimpan di folder ini.', 'The originals stay; copies with the new names go into this folder.')}</p>
        {:else}
          <p class="hint">{L('File diganti namanya di foldernya sendiri. Bisa diurungkan setelahnya.', 'Files are renamed in their own folder. You can undo it afterwards.')}</p>
        {/if}
      </div>
    </div>
    <div class="footer">
      {#if st.last.length}
        <div class="done">
          <span><Icon name="check" size={14} stroke={3} /> {st.last.length} {st.last[0].copy ? L('disalin', 'copied') : L('diganti', 'renamed')}</span>
          <button class="link" onclick={undo} disabled={running}><Icon name="undo" size={13} /> {L('Urungkan', 'Undo')}</button>
          <button class="link" onclick={() => api.revealFile(st.last[0].to)}>{L('Lihat', 'Show')}</button>
        </div>
      {/if}
      <button class="btn-primary" onclick={start} disabled={!!blocked || running} title={blocked}>
        {#if running}<Icon name="loader" size={18} class="spin" />{:else}<Icon name="tag" size={18} />{/if}
        {st.copy ? L('Salin dengan nama baru', 'Copy with new names') : L('Ganti nama', 'Rename')}{todo.length ? ` (${todo.length})` : ''}
      </button>
      {#if blocked}<span class="why">{blocked}</span>{/if}
    </div>
  </aside>
</div>

{#snippet counter()}
  <div class="grid3">
    <div class="sec tight">
      <label class="label" for="rn-start">{L('Mulai dari', 'Start at')}</label>
      <input id="rn-start" class="text-input" type="number" bind:value={o.start} />
    </div>
    <div class="sec tight">
      <label class="label" for="rn-step">{L('Langkah', 'Step')}</label>
      <input id="rn-step" class="text-input" type="number" bind:value={o.step} />
    </div>
    <div class="sec tight">
      <label class="label" for="rn-dig">{L('Digit', 'Digits')}</label>
      <input id="rn-dig" class="text-input" type="number" min="0" max="10" bind:value={o.digits} />
    </div>
  </div>
  <p class="hint">{L(`Contoh: ${String(o.start ?? 1).padStart(Math.min(10, o.digits || 0), '0')}, ${String((o.start ?? 1) + (o.step || 1)).padStart(Math.min(10, o.digits || 0), '0')}, … — urutan bisa diatur dengan tombol urutkan di daftar.`, `E.g. ${String(o.start ?? 1).padStart(Math.min(10, o.digits || 0), '0')}, ${String((o.start ?? 1) + (o.step || 1)).padStart(Math.min(10, o.digits || 0), '0')}, … — sort the list to set the order.`)}</p>
{/snippet}

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
    white-space: nowrap;
    overflow: hidden;
  }
  .n {
    font-size: 15px;
    font-weight: 700;
  }
  .size {
    font-size: 13px;
    color: var(--text-3);
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .head,
  .row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 20px minmax(0, 1.15fr) 32px;
    align-items: center;
    gap: 10px;
    padding: 0 14px;
  }
  .head {
    height: 34px;
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.06em;
    color: var(--text-3);
    border-bottom: 1px solid var(--border);
  }
  .rows {
    flex-grow: 1;
    min-height: 0;
    overflow-y: auto;
  }
  .row {
    min-height: 62px;
    padding-top: 8px;
    padding-bottom: 8px;
    border-bottom: 1px solid var(--border-soft);
  }
  .row.same {
    opacity: 0.6;
  }
  .toolbar .btn {
    flex-shrink: 0;
  }
  /* Narrow window: buttons without text, the new name under the old one. */
  .compact .bl {
    display: none;
  }
  .compact .toolbar .btn:not(.icon) {
    width: 36px;
    padding: 0;
  }
  .compact .head {
    display: none;
  }
  .compact .row {
    grid-template-columns: minmax(0, 1fr) 32px;
    row-gap: 6px;
  }
  .compact .row :global(.arrow) {
    display: none;
  }
  .compact .new {
    grid-row: 2;
    grid-column: 1;
    padding-left: 48px;
  }
  .old {
    display: flex;
    align-items: center;
    gap: 10px;
    min-width: 0;
  }
  .th {
    width: 38px;
    height: 38px;
    flex-shrink: 0;
    border-radius: 8px;
    object-fit: cover;
    background: #fff;
  }
  .th.page {
    object-fit: contain;
  }
  .th.tl {
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .nm {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .name {
    font-size: 13px;
    font-weight: 600;
  }
  .meta {
    font-size: 11px;
    color: var(--text-3);
  }
  .row :global(.arrow) {
    color: var(--text-3);
  }
  .new {
    display: flex;
    flex-direction: column;
    gap: 3px;
    min-width: 0;
  }
  .in {
    display: flex;
    align-items: center;
    gap: 4px;
    min-width: 0;
  }
  .in .text-input {
    flex-grow: 1;
    min-width: 0;
    height: 32px;
    padding: 0 9px;
    font-size: 13px;
  }
  .ext {
    font-size: 12px;
    color: var(--text-3);
    flex-shrink: 0;
  }
  .note {
    font-size: 11px;
    color: var(--text-3);
    display: flex;
    align-items: center;
    gap: 4px;
    min-width: 0;
  }
  .bad {
    color: var(--err);
  }
  .note.ok {
    color: var(--ok);
  }
  .note.warn {
    color: var(--warn);
  }
  .mini {
    width: 30px;
    height: 30px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--text-3);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .mini:hover {
    background: var(--row-active);
    color: var(--text);
  }
  .pw {
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
  .pw:hover {
    text-decoration: underline;
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
    gap: 6px;
    min-width: 0;
  }
  .grid2,
  .grid3 {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }
  .grid3 {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
  .codes {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .code {
    padding: 4px 8px;
    border-radius: 6px;
    border: 1px solid var(--border);
    background: var(--surface-2);
    font-family: ui-monospace, monospace;
    font-size: 12px;
    color: var(--text-2);
  }
  .code:hover {
    border-color: var(--accent);
    color: var(--accent);
  }
  .btn.dir {
    justify-content: flex-start;
    min-width: 0;
  }
  .link {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 0;
    border: 0;
    background: none;
    font-size: inherit;
    font-weight: 700;
    color: var(--accent);
  }
  .footer {
    padding: 16px 20px 20px;
    border-top: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: 10px;
    background: var(--surface-3);
    flex-shrink: 0;
  }
  .done {
    display: flex;
    align-items: center;
    gap: 12px;
    font-size: 12px;
    color: var(--ok);
    font-weight: 700;
  }
  .done span {
    flex-grow: 1;
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .why {
    font-size: 12px;
    color: var(--text-3);
    text-align: center;
  }
</style>
