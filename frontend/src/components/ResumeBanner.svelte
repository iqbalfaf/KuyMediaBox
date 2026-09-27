<script lang="ts" module>
  import type { PendingSummary } from '../lib/types'

  /** Downloads left unfinished when the app was closed last time. */
  export const resume = $state<{ info: PendingSummary | null }>({ info: null })
</script>

<script lang="ts">
  import { onMount } from 'svelte'
  import { L } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'
  import { api, errText } from '../lib/api'
  import { toast } from '../lib/stores/app.svelte'

  let busy = $state(false)

  async function refresh() {
    try {
      const s = await api.pendingDownloads()
      resume.info = s.items > 0 ? s : null
    } catch {
      resume.info = null
    }
  }

  onMount(refresh)

  async function go() {
    busy = true
    try {
      const n = await api.resumePendingDownloads()
      toast(L(`${n} unduhan dilanjutkan — lihat antrian di kiri bawah`, `${n} download${n === 1 ? '' : 's'} resumed — see the queue at the bottom left`), 'ok')
    } catch (e) {
      toast(errText(e), 'err')
    } finally {
      busy = false
      // Links that couldn't be read now stay listed, to try again or discard.
      await refresh()
    }
  }

  async function drop() {
    await api.discardPendingDownloads()
    resume.info = null
  }
</script>

{#if resume.info}
  <div class="banner" role="status">
    <Icon name="history" />
    <div class="txt">
      <b>{L(`${resume.info.items} unduhan dari sesi sebelumnya belum selesai`, `${resume.info.items} download${resume.info.items === 1 ? '' : 's'} from last time didn't finish`)}</b>
      <span class="ellipsis">{resume.info.titles.join(' · ')}</span>
    </div>
    <button class="btn-accent" onclick={go} disabled={busy}><Icon name={busy ? 'loader' : 'download'} size={14} stroke={2.5} />{busy ? L('Membaca link…', 'Reading links…') : L('Lanjutkan', 'Resume')}</button>
    <button class="btn" onclick={drop} disabled={busy}>{L('Buang', 'Discard')}</button>
  </div>
{/if}

<style>
  .banner {
    margin: 0 24px 14px 28px;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 14px;
    border-radius: 12px;
    background: var(--info-soft);
    border: 1px solid var(--border);
    color: var(--info);
    flex-shrink: 0;
  }
  .txt {
    flex-grow: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
    font-size: 13px;
    color: var(--text);
  }
  .txt span {
    color: var(--text-3);
  }
  .btn-accent {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
</style>
