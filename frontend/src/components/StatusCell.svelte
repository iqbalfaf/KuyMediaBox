<script lang="ts">
  import Icon from './Icon.svelte'
  import type { TaskInfo } from '../lib/types'
  import type { ItemState } from '../lib/stores/converter.svelte'

  let { state, task }: { state: ItemState; task?: TaskInfo } = $props()
  const p = $derived(task && task.progress >= 0 ? Math.round(task.progress * 100) : -1)
</script>

{#if state === 'running'}
  <div class="run">
    <span class="rt">{p >= 0 ? `Memproses ${p}%` : 'Memproses…'}</span>
    <div class="bar thin" class:indeterminate={p < 0}><div style="width: {Math.max(0, p)}%"></div></div>
  </div>
{:else if state === 'done'}
  <span class="pill ok"><Icon name="check" size={12} stroke={3} />Selesai</span>
{:else if state === 'failed'}
  <span class="pill err">Gagal</span>
{:else if state === 'invalid'}
  <span class="pill err">Tidak valid</span>
{:else if state === 'queued'}
  <span class="pill muted">Menunggu</span>
{:else if state === 'canceled'}
  <span class="pill muted">Dibatalkan</span>
{:else if state === 'skipped'}
  <span class="pill muted">Dilewati</span>
{:else}
  <span class="pill info">Siap</span>
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
