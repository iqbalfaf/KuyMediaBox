<script lang="ts">
  import PageHeader from '../components/PageHeader.svelte'
  import Segmented from '../components/Segmented.svelte'
  import Switch from '../components/Switch.svelte'
  import OutputPicker from '../components/OutputPicker.svelte'
  import Icon from '../components/Icon.svelte'
  import { api, errText, runtime } from '../lib/api'
  import {
    defaultDirs, fixedFolder, outputOf, saveSettings, setOutput, settings, shortPath, toast, toolState,
  } from '../lib/stores/app.svelte'
  import type { OutputKind, ToolStatus } from '../lib/types'
  import { checkNow, upd } from '../lib/stores/update.svelte'

  const icons: Record<string, string> = { ffmpeg: 'video', ytdlp: 'download', jsruntime: 'code', spotdl: 'music' }
  const sourceLabel: Record<string, string> = {
    downloaded: 'dipasang oleh KuyMediaBox',
    bundled: 'dari folder aplikasi',
    system: 'ditemukan di sistem (PATH)',
    custom: 'lokasi pilihan Anda',
  }
  const folderRows: { kind: OutputKind; label: string; icon: string }[] = [
    { kind: 'image', label: 'Gambar', icon: 'image' },
    { kind: 'video', label: 'Video', icon: 'video' },
    { kind: 'audio', label: 'Audio', icon: 'music' },
    { kind: 'download', label: 'Download', icon: 'download' },
  ]

  let checking = $state(false)
  let suffix = $state(settings.value?.suffix ?? '_converted')
  $effect(() => {
    // Keep the field in sync when settings load after mount.
    if (settings.value && document.activeElement?.id !== 'akhiran') suffix = settings.value.suffix
  })

  async function recheck() {
    checking = true
    try {
      toolState.list = await api.recheckTools()
      toast('Tools diperiksa ulang', 'ok')
    } catch (e) {
      toast(errText(e), 'err')
    } finally {
      checking = false
    }
  }

  async function install(t: ToolStatus) {
    try {
      await api.installTool(t.id)
      toast(`${t.name} ${t.updateAvailable ? 'berhasil di-update' : 'berhasil dipasang'}`, 'ok')
    } catch (e) {
      toast(`${t.name}: ${errText(e)}`, 'err')
    }
  }

  async function pickPath(t: ToolStatus) {
    try {
      await api.pickToolPath(t.id)
    } catch (e) {
      toast(errText(e), 'err')
    }
  }

  async function resetPath(t: ToolStatus) {
    try {
      await api.resetToolPath(t.id)
    } catch (e) {
      toast(errText(e), 'err')
    }
  }

  function saveSuffix() {
    if (settings.value && suffix !== settings.value.suffix) saveSettings({ suffix })
  }

  const example = $derived(`foto.jpg → foto${settings.value?.suffix ?? ''}.jpg`)

  async function resetAllFolders() {
    if (!settings.value) return
    await saveSettings({
      outputs: { image: { mode: 'default', dir: '' }, video: { mode: 'default', dir: '' }, audio: { mode: 'default', dir: '' }, download: { mode: 'default', dir: '' } },
    })
    toast('Semua folder hasil kembali ke default', 'ok')
  }

  const allDefault = $derived(folderRows.every((r) => outputOf(r.kind).mode === 'default'))
</script>

<PageHeader title="Pengaturan" subtitle="Kelola tools pendukung dan cara file disimpan." />

<div class="body">
  <div class="col-main">
    <section class="card" aria-label="Folder hasil">
      <div class="head">
        <div class="ht">
          <h2>Folder hasil</h2>
          <p>Tempat file hasil disimpan untuk setiap menu. Tombol <b>Simpan ke</b> di tiap halaman ikut berubah.</p>
        </div>
        <button class="btn" onclick={resetAllFolders} disabled={allDefault}><Icon name="refresh" size={16} />Semua ke default</button>
      </div>

      {#each folderRows as r (r.kind)}
        {@const o = outputOf(r.kind)}
        {@const folder = fixedFolder(r.kind)}
        <div class="frow">
          <div class="fic"><Icon name={r.icon} size={20} /></div>
          <div class="finfo">
            <span class="fname">{r.label}</span>
            <span class="fdef ellipsis" title={defaultDirs[r.kind] ?? ''}>Default: {shortPath(defaultDirs[r.kind] ?? '')}</span>
          </div>
          <OutputPicker kind={r.kind} placement="down" />
          <div class="factions">
            <button class="mini" title={folder ? `Buka ${folder}` : 'Mode dinamis: folder ikut lokasi file asli'} aria-label="Buka folder {r.label}" disabled={!folder} onclick={() => api.openFolder(folder)}>
              <Icon name="folderOpen" size={16} />
            </button>
            <button class="mini" title="Kembalikan ke folder default" aria-label="Reset folder {r.label}" disabled={o.mode === 'default'} onclick={() => setOutput(r.kind, { mode: 'default', dir: '' })}>
              <Icon name="refresh" size={16} />
            </button>
          </div>
        </div>
      {/each}

      {#if settings.value}
        <div class="fextra">
          <Switch
            checked={settings.value.downloadSubfolders}
            onchange={(v) => saveSettings({ downloadSubfolders: v })}
            label="Subfolder otomatis untuk playlist, channel & album"
            hint="Contoh: Downloads › KuyMediaBox › Nama Playlist"
          />
          <p class="hint">
            <b>Dinamis</b> = hasil disimpan di samping file asli, jadi lokasinya ikut file sumber. <b>Folder default</b> & <b>folder pilihan</b> = semua hasil terkumpul di satu tempat.
          </p>
        </div>
      {/if}
    </section>

    <section class="card" aria-label="Tools pendukung">
      <div class="head">
        <div class="ht">
          <h2>Tools pendukung</h2>
          <p>Dicek otomatis setiap aplikasi dibuka. Yang belum ada bisa diunduh di sini.</p>
        </div>
        <button class="btn" onclick={recheck} disabled={checking}><Icon name="refresh" size={16} class={checking ? 'spin' : ''} />Periksa ulang</button>
      </div>

      {#if !toolState.loaded && toolState.list.every((t) => !t.found && !t.error)}
        <div class="checking"><Icon name="loader" size={16} class="spin" /> Memeriksa tools…</div>
      {/if}

      {#each toolState.list as t (t.id)}
        <div class="tool" class:hl={t.updateAvailable || !t.found}>
          <div class="tic"><Icon name={icons[t.id] ?? 'box'} size={20} /></div>
          <div class="tinfo">
            <div class="tname">
              <span>{t.id === 'jsruntime' && t.found ? `JS runtime (${t.runtime === 'deno' ? 'Deno' : 'Node.js'})` : t.name}</span>
              {#if t.found && t.version}<span class="ver">{t.version}</span>{/if}
              {#if !t.found && !t.busy}<span class="ver bad">Belum ada</span>{/if}
            </div>
            <span class="tdesc">
              {t.description}
              {#if t.found}
                · {sourceLabel[t.source] ?? ''}
              {:else}
                · dibutuhkan untuk {t.required}
              {/if}
              {#if t.id === 'jsruntime' && t.found && t.runtime === 'node'}— Deno tidak perlu diunduh{/if}
            </span>
            {#if t.updateAvailable && !t.busy}<span class="upd">Versi {t.latest} tersedia</span>{/if}
            {#if t.error && !t.busy}<span class="terr">{t.error}</span>{/if}
            <span class="tlinks">
              {#if t.found}<span class="path ellipsis" title={t.path}>{t.path}</span>{/if}
              <button class="tl" onclick={() => pickPath(t)}>Pilih file…</button>
              {#if t.source === 'custom'}<button class="tl" onclick={() => resetPath(t)}>Reset</button>{/if}
            </span>
          </div>
          <div class="tact">
            {#if t.busy}
              <div class="prog">
                <div class="bar"><div style="width: {t.progress * 100}%"></div></div>
                <span>{Math.round(t.progress * 100)}%</span>
              </div>
            {:else if !t.found}
              <button class="btn" onclick={() => install(t)}><Icon name="download" size={14} stroke={2.5} />Unduh</button>
            {:else if t.updateAvailable}
              <button class="btn-accent" onclick={() => install(t)}><Icon name="refresh" size={14} stroke={2.5} />Update</button>
            {:else}
              <span class="pill ok"><Icon name="check" size={12} stroke={3} />Siap</span>
            {/if}
          </div>
        </div>
      {/each}
    </section>
  </div>

  <div class="col-side">
  <section class="card" aria-label="Tentang dan update">
    <div class="head"><h2>Tentang & update</h2></div>
    <div class="gbody">
      <div class="about">
        <div class="logo"><Icon name="box" size={22} stroke={2.2} /></div>
        <div class="about-t">
          <b>KuyMediaBox</b>
          <span>Versi {upd.version || '—'}</span>
        </div>
        {#if upd.info?.available}
          <button class="btn-accent" onclick={() => (upd.open = true)}><Icon name="download" size={14} stroke={2.5} />Update v{upd.info.latest}</button>
        {:else}
          <button class="btn" onclick={checkNow} disabled={upd.checking}>
            <Icon name={upd.checking ? 'loader' : 'refresh'} size={16} class={upd.checking ? 'spin' : ''} />{upd.checking ? 'Mengecek…' : 'Cek update'}
          </button>
        {/if}
      </div>
      {#if settings.value}
        <Switch checked={settings.value.autoUpdate} onchange={(v) => saveSettings({ autoUpdate: v })} label="Cek update otomatis" hint="Saat aplikasi dibuka, dari GitHub Releases" />
      {/if}
      <button class="link" onclick={() => runtime.openURL('https://github.com/iqbalfaf/KuyMediaBox/releases')}>Lihat semua rilis di GitHub</button>
    </div>
  </section>

  <section class="card general" aria-label="Pengaturan umum">
    <div class="head"><h2>Umum</h2></div>
    {#if settings.value}
      <div class="gbody">
        <div class="sec">
          <label class="label" for="akhiran">Tambahan di nama file</label>
          <input id="akhiran" class="text-input" bind:value={suffix} onblur={saveSuffix} onkeydown={(e) => e.key === 'Enter' && (e.currentTarget as HTMLInputElement).blur()} placeholder="(kosong)" maxlength="40" />
          <span class="hint">Contoh: {example}</span>
        </div>
        <div class="sec">
          <span class="label">Jika nama file sudah ada</span>
          <Segmented
            label="Jika nama file sudah ada"
            value={settings.value.conflict}
            onchange={(v) => saveSettings({ conflict: v })}
            options={[
              { value: 'rename', label: 'Nama baru' },
              { value: 'skip', label: 'Lewati' },
              { value: 'overwrite', label: 'Timpa' },
            ]}
          />
          <p class="hint">File asli tidak pernah ditimpa, apa pun pilihannya.</p>
        </div>
        <Switch checked={settings.value.notify} onchange={(v) => saveSettings({ notify: v })} label="Notifikasi saat selesai" hint="Muncul di pojok kanan bawah Windows" />
        <Switch checked={settings.value.skipDownloaded} onchange={(v) => saveSettings({ skipDownloaded: v })} label="Lewati video yang pernah diunduh" hint="Bawaan untuk link baru di halaman Download" />
      </div>
    {/if}
  </section>
  </div>
</div>

<style>
  .col-side {
    width: 360px;
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    gap: 20px;
  }
  .about {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .logo {
    width: 44px;
    height: 44px;
    flex-shrink: 0;
    border-radius: 12px;
    background: var(--accent);
    color: var(--accent-ink);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .about-t {
    flex-grow: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .about-t b {
    font-size: 15px;
  }
  .about-t span {
    font-size: 12px;
    color: var(--text-3);
  }
  .body {
    flex-grow: 1;
    min-height: 0;
    display: flex;
    gap: 20px;
    padding: 0 24px 24px 28px;
    overflow-y: auto;
    align-items: flex-start;
  }
  .col-main {
    flex-grow: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 20px;
  }
  .col-main .card {
    overflow: visible;
  }
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
    min-width: 0;
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
  .head p b {
    color: var(--text-2);
  }
  .frow {
    display: grid;
    grid-template-columns: 40px 170px minmax(0, 1fr) auto;
    align-items: center;
    gap: 14px;
    padding: 12px 20px;
    border-bottom: 1px solid var(--border-soft);
  }
  .fic {
    width: 40px;
    height: 40px;
    border-radius: 12px;
    background: var(--surface-2);
    color: var(--accent-text-2);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .finfo {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .fname {
    font-size: 14px;
    font-weight: 700;
  }
  .fdef {
    font-size: 11px;
    color: var(--text-3);
  }
  .factions {
    display: flex;
    gap: 2px;
  }
  .mini {
    width: 34px;
    height: 34px;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--text-2);
  }
  .mini:hover:not(:disabled) {
    background: var(--surface-2);
    color: var(--text);
  }
  .mini:disabled {
    opacity: 0.3;
  }
  .fextra {
    padding: 14px 20px 18px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .fextra b {
    color: var(--text-2);
  }
  .checking {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px 20px;
    font-size: 13px;
    color: var(--text-2);
  }
  .tool {
    display: grid;
    grid-template-columns: 44px minmax(0, 1fr) auto;
    align-items: center;
    gap: 14px;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border-soft);
  }
  .tool:last-child {
    border-bottom: 0;
    border-radius: 0 0 16px 16px;
  }
  .tool.hl {
    background: #1c1f27;
  }
  .tic {
    width: 44px;
    height: 44px;
    border-radius: 12px;
    background: var(--surface-2);
    color: var(--accent-text-2);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .tinfo {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }
  .tname {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 15px;
    font-weight: 700;
  }
  .ver {
    padding: 2px 7px;
    border-radius: 6px;
    background: var(--inset);
    border: 1px solid var(--border);
    color: var(--text-2);
    font-family: var(--mono);
    font-size: 11px;
    font-weight: 500;
  }
  .ver.bad {
    font-family: var(--font);
    font-weight: 700;
    background: var(--err-soft);
    border-color: transparent;
    color: #ff9c9c;
  }
  .tdesc {
    font-size: 12px;
    color: var(--text-3);
  }
  .upd {
    font-size: 12px;
    font-weight: 700;
    color: var(--warn);
  }
  .terr {
    font-size: 12px;
    color: var(--err);
  }
  .tlinks {
    display: flex;
    align-items: center;
    gap: 10px;
    min-width: 0;
  }
  .path {
    font-family: var(--mono);
    font-size: 11px;
    color: #6f7887;
    max-width: 360px;
  }
  .tl {
    padding: 0;
    border: 0;
    background: none;
    font-size: 11px;
    font-weight: 700;
    color: var(--accent-text-2);
    white-space: nowrap;
  }
  .tl:hover {
    color: var(--accent-text);
  }
  .prog {
    width: 150px;
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    font-weight: 700;
    color: var(--accent-text-2);
  }
  .prog .bar {
    flex-grow: 1;
  }
  .gbody {
    padding: 18px 20px 20px;
    display: flex;
    flex-direction: column;
    gap: 20px;
  }
  .sec {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
</style>
