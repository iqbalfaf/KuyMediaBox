<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'
  import type { TaskInfo } from '../lib/types'
  import type { ItemState } from '../lib/stores/converter.svelte'

  let { state, task }: { state: ItemState; task?: TaskInfo } = $props()
  const p = $derived(task && task.progress >= 0 ? Math.round(task.progress * 100) : -1)
</script>

{#if state === 'running'}
  <div class="run">
    <span class="rt">{p >= 0 ? `${L('Memproses', 'Processing')} ${p}%` : L('Memproses…', 'Processing…')}</span>
    <div class="bar thin" class:indeterminate={p < 0}><div style="width: {Math.max(0, p)}%"></div></div>
  </div>
{:else if state === 'done'}
  <span class="pill ok"><Icon name="check" size={12} stroke={3} />{L('Selesai', 'Done')}</span>
{:else if state === 'failed'}
  <span class="pill err">{L('Gagal', 'Failed')}</span>
{:else if state === 'invalid'}
  <span class="pill err">{L('Tidak valid', 'Invalid')}</span>
{:else if state === 'queued'}
  <span class="pill muted">{L('Menunggu', 'Waiting')}</span>
{:else if state === 'canceled'}
  <span class="pill muted">{L('Dibatalkan', 'Canceled')}</span>
{:else if state === 'skipped'}
  <span class="pill muted">{L('Dilewati', 'Skipped')}</span>
{:else}
  <span class="pill info">{L('Siap', 'Ready')}</span>
{/if}

<style>
  .run {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .rt {
    font-size: 12px;
    font-weight: 700;
    color: var(--accent-text-2);
    white-space: nowrap;
  }
  .thin {
    height: 4px;
  }
</style>
