<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import { onMount } from 'svelte'
  import Icon from './Icon.svelte'
  import Switch from './Switch.svelte'
  import Select from './Select.svelte'
  import { api, errText } from '../lib/api'
  import { saveSettings, settings, shortPath, toast } from '../lib/stores/app.svelte'
  import { load } from '../lib/stores/persist'
  import type { WatchRule } from '../lib/types'

  const kinds = $derived<{ id: string; label: string; icon: string; def: number }[]>([
    { id: 'image', label: L('Gambar', 'Images'), icon: 'image', def: 3 },
    { id: 'video', label: 'Video', icon: 'video', def: 1 },
    { id: 'audio', label: 'Audio', icon: 'music', def: 2 },
    { id: 'download', label: 'Download', icon: 'download', def: 2 },
    { id: 'pdf', label: 'PDF', icon: 'fileText', def: 2 },
  ])

  function setParallel(kind: string, n: number) {
    if (!settings.value) return
    n = Math.max(1, Math.min(8, n))
    saveSettings({ parallel: { ...settings.value.parallel, [kind]: n } })
  }

  // ---- "Send to" menu ----
  let sendTo = $state(false)
  let sendBusy = $state(false)
  onMount(async () => {
    try {
      sendTo = await api.getSendTo()
    } catch {
      /* not on Windows */
    }
  })
  async function toggleSendTo(on: boolean) {
    sendBusy = true
    try {
      sendTo = await api.setSendTo(on)
      toast(on ? L('KuyMediaBox ada di menu Kirim ke (Send to)', 'KuyMediaBox added to the Send to menu') : L('Dihapus dari menu Kirim ke', 'Removed from the Send to menu'), 'ok')
    } catch (e) {
      sendTo = !on
      toast(errText(e), 'err')
    } finally {
      sendBusy = false
    }
  }

  // ---- watched folders ----
  /** The settings a page uses right now, as the backend expects them. */
  function moduleOptions(kind: string): any {
    if (kind === 'image') return load<any>('kmb.image', { o: {} }).o
    if (kind === 'video') {
      const v = load<any>('kmb.video', { mode: 'video', v: {}, a: {} })
      return { mode: v.mode === 'audio' ? 'audio' : 'video', video: v.v, audio: v.a, frameEvery: 0, frameFormat: 'jpg' }
    }
    return { mode: 'convert', options: load<any>('kmb.audio', { a: {} }).a }
  }

  function describe(r: WatchRule): string {
    const o = r.options ?? {}
    if (r.kind === 'image') return `${L('Gambar', 'Images')} → ${String(o.format ?? 'jpg').toUpperCase()}`
    if (r.kind === 'video') return o.mode === 'audio' ? `Video → ${String(o.audio?.format ?? 'mp3').toUpperCase()}` : `Video → ${String(o.video?.format ?? 'mp4').toUpperCase()} ${String(o.video?.codec ?? '').toUpperCase()}`
    return `Audio → ${String(o.options?.format ?? 'mp3').toUpperCase()}`
  }

  async function addWatch() {
    if (!settings.value) return
    const dir = await api.pickDirectory(L('Pilih folder yang dipantau', 'Choose the folder to watch'))
    if (!dir) return
    const rule: WatchRule = { id: `w${Date.now()}`, dir, kind: 'image', options: moduleOptions('image'), enabled: true }
    await saveSettings({ watch: [...(settings.value.watch ?? []), rule] })
    toast(L('Folder dipantau. Pilih jenis file yang diproses.', 'Folder watched. Choose which files get converted.'), 'ok')
  }

  function updateWatch(id: string, patch: Partial<WatchRule>) {
    if (!settings.value) return
    saveSettings({ watch: settings.value.watch.map((r) => (r.id === id ? { ...r, ...patch } : r)) })
  }

  function removeWatch(id: string) {
    if (!settings.value) return
    saveSettings({ watch: settings.value.watch.filter((r) => r.id !== id) })
  }
</script>

{#if settings.value}
  <section class="card" aria-label={L('Proses & otomatisasi', 'Processing & automation')}>
    <div class="head">
      <div class="ht">
        <h2>{L('Proses & otomatisasi', 'Processing & automation')}</h2>
        <p>{L('Berapa tugas berjalan bersamaan, integrasi Windows, dan folder yang diproses otomatis.', 'How many tasks run at once, Windows integration and folders converted automatically.')}</p>
      </div>
    </div>

    <div class="block">
      <span class="label">{L('Jumlah proses paralel', 'Tasks at the same time')}</span>
      <div class="par">
        {#each kinds as k (k.id)}
          {@const n = settings.value.parallel?.[k.id] ?? k.def}
          <div class="pk">
            <span class="pl"><Icon name={k.icon} size={15} />{k.label}</span>
            <div class="stepper">
              <button aria-label={L(`Kurangi ${k.label}`, `Fewer ${k.label}`)} disabled={n <= 1} onclick={() => setParallel(k.id, n - 1)}><Icon name="minus" size={14} /></button>
              <span>{n}</span>
              <button aria-label={L(`Tambah ${k.label}`, `More ${k.label}`)} disabled={n >= 8} onclick={() => setParallel(k.id, n + 1)}><Icon name="plus" size={14} /></button>
            </div>
          </div>
        {/each}
      </div>
      <p class="hint">{L('Angka lebih besar = lebih cepat untuk banyak file, tapi lebih berat untuk komputer. Video sebaiknya 1–2.', 'Higher = faster for many files, but heavier on the PC. Keep video at 1–2.')}</p>
    </div>

    <div class="block">
      <Switch
        checked={sendTo}
        onchange={(v) => !sendBusy && toggleSendTo(v)}
        label={L('Menu klik kanan "Kirim ke › KuyMediaBox"', 'Right-click menu "Send to › KuyMediaBox"')}
        hint={L('Pilih file di Explorer, klik kanan › Kirim ke › KuyMediaBox. File masuk ke halaman yang cocok.', 'Select files in Explorer, right-click › Send to › KuyMediaBox. They open on the matching page.')}
      />
    </div>

    <div class="block">
      <div class="row-between">
        <div class="col">
          <span class="label">{L('Folder pantauan', 'Watched folders')}</span>
          <span class="hint">{L('File baru yang masuk ke folder ini otomatis dikonversi dengan pengaturan halamannya saat folder ditambahkan.', 'New files that land in these folders are converted with the page settings saved with the rule.')}</span>
        </div>
        <button class="btn" onclick={addWatch}><Icon name="plus" size={16} />{L('Tambah folder', 'Add folder')}</button>
      </div>
      {#each settings.value.watch ?? [] as r (r.id)}
        <div class="watch" class:off={!r.enabled}>
          <span class="wic"><Icon name="eye" size={16} /></span>
          <div class="winfo">
            <span class="ellipsis" title={r.dir}>{shortPath(r.dir) || r.dir}</span>
            <span class="hint ellipsis">{describe(r)}</span>
          </div>
          <div class="wkind">
            <Select
              label={L('Jenis file', 'File type')}
              value={r.kind}
              onchange={(v) => updateWatch(r.id, { kind: v as WatchRule['kind'], options: moduleOptions(v) })}
              options={[
                { value: 'image', label: L('Gambar', 'Images') },
                { value: 'video', label: 'Video' },
                { value: 'audio', label: 'Audio' },
              ]}
            />
          </div>
          <button class="mini" title={L('Pakai pengaturan halaman sekarang', 'Use the page settings from now')} aria-label={L('Perbarui pengaturan', 'Update settings')} onclick={() => { updateWatch(r.id, { options: moduleOptions(r.kind) }); toast(L('Pengaturan folder pantauan diperbarui', 'Watched folder settings updated'), 'ok') }}><Icon name="refresh" size={15} /></button>
          <button class="mini" title={r.enabled ? L('Jeda', 'Pause') : L('Aktifkan', 'Resume')} aria-label={r.enabled ? L('Jeda', 'Pause') : L('Aktifkan', 'Resume')} onclick={() => updateWatch(r.id, { enabled: !r.enabled })}><Icon name={r.enabled ? 'pause' : 'play'} size={15} /></button>
          <button class="mini" title={L('Hapus', 'Remove')} aria-label={L('Hapus folder pantauan', 'Remove watched folder')} onclick={() => removeWatch(r.id)}><Icon name="trash" size={15} /></button>
        </div>
      {/each}
      {#if (settings.value.watch ?? []).length === 0}
        <p class="hint">{L('Belum ada folder yang dipantau. Hasil disimpan ke folder hasil tiap menu.', 'No watched folders yet. Results go to each module\'s output folder.')}</p>
      {/if}
    </div>
  </section>
{/if}

<style>
  .head {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 18px 20px;
    border-bottom: 1px solid var(--border);
  }
  .ht {
    flex-grow: 1;
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  h2 {
    margin: 0;
    font-size: 16px;
    font-weight: 800;
  }
  .head p {
    margin: 0;
    font-size: 12px;
    color: var(--text-3);
  }
  .block {
    padding: 16px 20px;
    border-bottom: 1px solid var(--border-soft);
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .block:last-child {
    border-bottom: 0;
  }
  .par {
    display: grid;
    grid-template-columns: repeat(5, minmax(0, 1fr));
    gap: 8px;
  }
  .pk {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 10px;
    border-radius: 12px;
    background: var(--surface-2);
    border: 1px solid var(--border);
  }
  .pl {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    font-weight: 700;
    color: var(--text-2);
  }
  .stepper {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .stepper span {
    font-size: 18px;
    font-weight: 800;
    font-variant-numeric: tabular-nums;
  }
  .stepper button {
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--text-2);
  }
  .stepper button:disabled {
    opacity: 0.35;
  }
  .row-between {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }
  .col {
    display: flex;
    flex-direction: column;
    gap: 3px;
    min-width: 0;
  }
  .watch {
    display: grid;
    grid-template-columns: 32px minmax(0, 1fr) 150px 32px 32px 32px;
    align-items: center;
    gap: 8px;
    padding: 8px 8px 8px 10px;
    border-radius: 12px;
    background: var(--surface-2);
    border: 1px solid var(--border);
  }
  .watch.off {
    opacity: 0.6;
  }
  .wic {
    width: 32px;
    height: 32px;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--accent-tint);
    color: var(--accent);
  }
  .winfo {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
    font-size: 13px;
    font-weight: 600;
  }
  .mini {
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--text-2);
  }
  .mini:hover {
    background: var(--hover);
    color: var(--text);
  }
</style>
