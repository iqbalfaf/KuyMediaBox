<script lang="ts" module>
  import type { Collection, DownloadOptions } from '../lib/types'

  /** "Follow this link" request from a Download row. */
  export const follow = $state<{ open: boolean; col: Collection | null; opts: DownloadOptions | null; tabs: string[] }>({ open: false, col: null, opts: null, tabs: [] })

  export function askFollow(col: Collection, opts: DownloadOptions, tabs: string[]) {
    Object.assign(follow, { col, opts, tabs, open: true })
  }
</script>

<script lang="ts">
  import { onMount } from 'svelte'
  import { L } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'
  import Chips from './Chips.svelte'
  import Segmented from './Segmented.svelte'
  import Switch from './Switch.svelte'
  import { api, errText } from '../lib/api'
  import { toast } from '../lib/stores/app.svelte'
  import { loadSubs, subs } from '../lib/stores/subs.svelte'
  import { sourceName } from '../lib/stores/download.svelte'

  let followDialog: HTMLDialogElement
  let listDialog: HTMLDialogElement
  let hours = $state(6)
  let backfill = $state<'new' | 'all'>('new')
  let busy = $state(false)

  onMount(loadSubs)

  $effect(() => {
    if (follow.open && followDialog && !followDialog.open) {
      backfill = 'new'
      followDialog.showModal()
    }
    if (!follow.open && followDialog?.open) followDialog.close()
  })
  $effect(() => {
    if (subs.open && listDialog && !listDialog.open) listDialog.showModal()
    if (!subs.open && listDialog?.open) listDialog.close()
  })

  const intervals = [
    { value: 1, label: L('1 jam', '1 h') },
    { value: 6, label: L('6 jam', '6 h') },
    { value: 12, label: L('12 jam', '12 h') },
    { value: 24, label: L('1 hari', '1 day') },
    { value: 168, label: L('1 minggu', '1 week') },
  ]

  async function confirm() {
    if (!follow.col || !follow.opts) return
    busy = true
    try {
      await api.addSubscription(follow.col.key, hours, $state.snapshot(follow.tabs), $state.snapshot(follow.opts), backfill === 'all')
      toast(L(`Mengikuti "${follow.col.title}" — dicek tiap ${intervals.find((i) => i.value === hours)?.label}`, `Following "${follow.col.title}" — checked every ${intervals.find((i) => i.value === hours)?.label}`), 'ok')
      follow.open = false
    } catch (e) {
      toast(errText(e), 'err')
    } finally {
      busy = false
    }
  }

  function ago(ts: number): string {
    if (!ts) return L('belum pernah', 'never')
    const m = Math.round((Date.now() / 1000 - ts) / 60)
    if (m < 1) return L('baru saja', 'just now')
    if (m < 60) return L(`${m} menit lalu`, `${m} min ago`)
    const h = Math.round(m / 60)
    if (h < 48) return L(`${h} jam lalu`, `${h} h ago`)
    return L(`${Math.round(h / 24)} hari lalu`, `${Math.round(h / 24)} days ago`)
  }
</script>

<dialog bind:this={followDialog} onclose={() => (follow.open = false)}>
  <div class="box">
    <div class="head">
      <Icon name="bell" size={18} />
      <b class="ellipsis">{L('Ikuti', 'Follow')} · {follow.col?.title}</b>
      <button class="btn icon" aria-label={L('Tutup', 'Close')} onclick={() => (follow.open = false)}><Icon name="x" size={16} /></button>
    </div>
    <p class="hint">{L('KuyMediaBox memeriksa link ini secara berkala selama aplikasi berjalan (juga saat di tray), lalu mengunduh item baru dengan pengaturan kategori ini.', 'KuyMediaBox checks this link regularly while it runs (also from the tray) and downloads new items with this category\'s settings.')}</p>
    <span class="label">{L('Periksa setiap', 'Check every')}</span>
    <Chips bind:value={hours} columns={5} small options={intervals} />
    <span class="label">{L('Mulai dari', 'Start with')}</span>
    <Segmented
      label={L('Mulai dari', 'Start with')}
      bind:value={backfill}
      options={[
        { value: 'new', label: L('Hanya yang baru', 'New items only') },
        { value: 'all', label: L('Semua yang belum ada', 'Everything missing') },
      ]}
    />
    <p class="hint">
      {backfill === 'new'
        ? L('Isi yang sekarang ada dianggap sudah dilihat; hanya unggahan berikutnya yang diunduh.', 'What is listed now counts as seen; only future uploads are downloaded.')
        : L(`Semua ${follow.col?.entries.length ?? 0} item yang belum pernah diunduh ikut diunduh sekarang.`, `All ${follow.col?.entries.length ?? 0} items not downloaded yet are fetched now too.`)}
    </p>
    <div class="foot">
      <span class="grow"></span>
      <button class="btn" onclick={() => (follow.open = false)}>{L('Batal', 'Cancel')}</button>
      <button class="btn-accent" onclick={confirm} disabled={busy}><Icon name="bell" size={14} />{L('Ikuti', 'Follow')}</button>
    </div>
  </div>
</dialog>

<dialog bind:this={listDialog} onclose={() => (subs.open = false)} class="wide">
  <div class="box">
    <div class="head">
      <Icon name="bell" size={18} />
      <b>{L('Langganan', 'Subscriptions')} ({subs.list.length})</b>
      <button class="btn icon" aria-label={L('Tutup', 'Close')} onclick={() => (subs.open = false)}><Icon name="x" size={16} /></button>
    </div>
    {#if subs.list.length === 0}
      <p class="empty">{L('Belum ada langganan. Tempel link channel, playlist, atau profil di halaman Download, lalu klik ikon lonceng.', 'No subscriptions yet. Paste a channel, playlist or profile link on the Download page, then click the bell icon.')}</p>
    {:else}
      <div class="list">
        {#each subs.list as s (s.id)}
          <div class="sub" class:off={!s.enabled}>
            <div class="st">
              <b class="ellipsis" title={s.url}>{s.title}</b>
              <span class="ellipsis">
                {sourceName[s.source] ?? s.source} · {s.mode === 'audio' ? L('audio', 'audio') : 'video'} ·
                {s.checking ? L('memeriksa…', 'checking…') : `${L('dicek', 'checked')} ${ago(s.lastCheck)}`}
                {#if s.lastNew && !s.checking} · <b class="new">{s.lastNew} {L('baru', 'new')}</b>{/if}
                · {L('total', 'total')} {s.totalNew}
              </span>
              {#if s.lastError}<span class="err ellipsis" title={s.lastError}>{s.lastError}</span>{/if}
            </div>
            <select
              aria-label={L('Interval', 'Interval')}
              value={s.everyHours}
              onchange={(e) => api.setSubscription(s.id, s.enabled, +(e.currentTarget as HTMLSelectElement).value)}
            >
              {#each intervals as i}<option value={i.value}>{i.label}</option>{/each}
            </select>
            <div class="sw"><Switch checked={s.enabled} label="" onchange={(v) => api.setSubscription(s.id, v, s.everyHours)} /></div>
            <button class="btn icon" title={L('Periksa sekarang', 'Check now')} aria-label={L('Periksa sekarang', 'Check now')} disabled={s.checking} onclick={() => api.checkSubscription(s.id)}><Icon name={s.checking ? 'loader' : 'refresh'} size={15} /></button>
            <button class="btn icon" title={L('Berhenti mengikuti', 'Unfollow')} aria-label={L('Berhenti mengikuti', 'Unfollow')} onclick={() => api.removeSubscription(s.id)}><Icon name="trash" size={15} /></button>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</dialog>

<style>
  dialog {
    padding: 0;
    border: 1px solid var(--border-strong);
    border-radius: 16px;
    background: var(--surface);
    color: var(--text);
    width: min(480px, 92vw);
    box-shadow: 0 24px 64px rgba(0, 0, 0, 0.55);
  }
  dialog.wide {
    width: min(760px, 92vw);
  }
  dialog::backdrop {
    background: rgba(5, 6, 8, 0.7);
  }
  .box {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 16px 18px 18px;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 10px;
    color: var(--accent);
  }
  .head b {
    flex: 1;
    font-size: 15px;
    color: var(--text);
  }
  .hint {
    margin: 0;
  }
  .foot {
    display: flex;
    gap: 8px;
    margin-top: 6px;
  }
  .grow {
    flex: 1;
  }
  .empty {
    color: var(--text-3);
    font-size: 13px;
    padding: 20px 0;
    text-align: center;
  }
  .list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-height: 60vh;
    overflow-y: auto;
  }
  .sub {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 12px;
    border-radius: 12px;
    background: var(--surface-2);
    border: 1px solid var(--border);
  }
  .sub.off {
    opacity: 0.6;
  }
  .st {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
    font-size: 12px;
    color: var(--text-3);
  }
  .st > b {
    font-size: 13px;
    color: var(--text);
  }
  .new {
    color: var(--ok);
  }
  .err {
    color: var(--err);
  }
  select {
    height: 32px;
    border-radius: 8px;
    border: 1px solid var(--border);
    background: var(--surface);
    color: var(--text);
    padding: 0 6px;
  }
  .sw {
    width: 48px;
  }
</style>
