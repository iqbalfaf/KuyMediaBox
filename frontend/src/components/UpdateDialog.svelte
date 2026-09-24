<script lang="ts">
  import Icon from './Icon.svelte'
  import { runtime } from '../lib/api'
  import { bytes } from '../lib/format'
  import { installNow, upd } from '../lib/stores/update.svelte'
  import { isActive, tasks } from '../lib/stores/tasks.svelte'

  let dialog = $state<HTMLDialogElement>()

  $effect(() => {
    if (upd.open && dialog && !dialog.open) dialog.showModal()
    if (!upd.open && dialog?.open) dialog.close()
  })

  const busyTasks = $derived(Object.values(tasks).filter((t) => isActive(t)).length)
  const date = $derived(
    upd.info?.publishedAt ? new Intl.DateTimeFormat('id-ID', { day: 'numeric', month: 'long', year: 'numeric' }).format(new Date(upd.info.publishedAt)) : '',
  )

  function close() {
    if (!upd.installing) upd.open = false
  }
</script>

<dialog bind:this={dialog} onclose={() => (upd.open = false)} oncancel={(e) => upd.installing && e.preventDefault()}>
  {#if upd.info}
    <div class="box">
      <div class="head">
        <div class="ic"><Icon name="download" size={22} /></div>
        <div class="t">
          <b>Versi baru tersedia: v{upd.info.latest}</b>
          <span>Versi Anda sekarang v{upd.info.current}{date ? ` · dirilis ${date}` : ''}</span>
        </div>
        {#if !upd.installing}<button class="btn icon" aria-label="Tutup" onclick={close}><Icon name="x" size={16} /></button>{/if}
      </div>

      <div class="notes">
        <span class="label">Yang baru</span>
        <pre>{upd.info.notes || 'Perbaikan dan peningkatan.'}</pre>
        {#if upd.info.url}<button class="link" onclick={() => runtime.openURL(upd.info!.url)}>Lihat di GitHub</button>{/if}
      </div>

      {#if upd.installing}
        <div class="prog">
          <div class="line">
            <span>{upd.stage === 'install' ? 'Memasang & memulai ulang…' : `Mengunduh ${bytes(upd.info.assetSize)}…`}</span>
            <span>{Math.round(upd.progress * 100)}%</span>
          </div>
          <div class="bar big" class:indeterminate={upd.stage === 'install'}><div style="width: {upd.progress * 100}%"></div></div>
          {#if upd.stage === 'install' && upd.info.mode === 'installer'}<p class="hint">Jika Windows meminta izin (UAC), pilih <b>Yes</b>.</p>{/if}
        </div>
      {:else}
        {#if upd.error}<p class="err"><Icon name="alert" size={14} /> {upd.error}</p>{/if}
        {#if busyTasks > 0}<p class="warn">Ada {busyTasks} tugas yang sedang berjalan. Tugas itu akan dibatalkan saat update.</p>{/if}
        <p class="hint">
          {upd.info.mode === 'installer'
            ? 'Installer versi baru akan dijalankan otomatis, lalu aplikasi dibuka kembali.'
            : 'File aplikasi akan diganti otomatis, lalu aplikasi dibuka kembali.'}
          File diperiksa dengan checksum SHA-256 sebelum dipasang.
        </p>
        <div class="foot">
          <button class="btn" onclick={close}>Nanti saja</button>
          <button class="btn-accent" onclick={installNow}><Icon name="download" size={14} stroke={2.5} />Update sekarang</button>
        </div>
      {/if}
    </div>
  {/if}
</dialog>

<style>
  dialog {
    padding: 0;
    border: 1px solid var(--border-strong);
    border-radius: 16px;
    background: var(--surface);
    color: var(--text);
    width: min(560px, 90vw);
    box-shadow: 0 24px 64px rgba(0, 0, 0, 0.55);
  }
  dialog::backdrop {
    background: rgba(5, 6, 8, 0.6);
  }
  .box {
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 18px 20px 20px;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .ic {
    width: 44px;
    height: 44px;
    flex-shrink: 0;
    border-radius: 12px;
    background: var(--accent-tint);
    color: var(--accent);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .t {
    flex-grow: 1;
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  .t b {
    font-size: 16px;
  }
  .t span {
    font-size: 12px;
    color: var(--text-3);
  }
  .notes {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  pre {
    margin: 0;
    max-height: 220px;
    overflow: auto;
    padding: 12px 14px;
    border-radius: 10px;
    background: var(--inset);
    border: 1px solid var(--border);
    font-family: var(--font);
    font-size: 13px;
    line-height: 1.55;
    color: var(--text-4);
    white-space: pre-wrap;
    word-break: break-word;
    user-select: text;
  }
  .link {
    align-self: flex-start;
    padding: 0;
    border: 0;
    background: none;
    font-size: 12px;
    font-weight: 700;
    color: var(--accent-text-2);
  }
  .prog {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .line {
    display: flex;
    justify-content: space-between;
    font-size: 13px;
    font-weight: 700;
  }
  .big {
    height: 8px;
  }
  .err {
    margin: 0;
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    color: var(--err);
  }
  .warn {
    margin: 0;
    font-size: 12px;
    color: var(--warn);
  }
  .foot {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
</style>
