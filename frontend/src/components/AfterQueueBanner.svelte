<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'
  import { after, cancelAfter } from '../lib/stores/history.svelte'
</script>

{#if after.pending}
  <div class="banner" role="alertdialog" aria-live="assertive">
    <span class="ic"><Icon name="power" size={20} /></span>
    <div class="t">
      <b>{after.action === 'shutdown' ? L('Komputer akan dimatikan', 'The computer will shut down') : L('Komputer akan sleep', 'The computer will go to sleep')}</b>
      <span>{L(`Semua tugas selesai. Dalam ${after.seconds} detik…`, `All tasks are done. In ${after.seconds} seconds…`)}</span>
    </div>
    <button class="btn-accent" onclick={cancelAfter}>{L('Batalkan', 'Cancel')}</button>
  </div>
{/if}

<style>
  .banner {
    position: fixed;
    left: 50%;
    bottom: 24px;
    transform: translateX(-50%);
    z-index: 60;
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 14px 16px;
    border-radius: 14px;
    border: 1px solid var(--accent);
    background: var(--surface);
    box-shadow: 0 16px 48px var(--shadow);
    min-width: 420px;
  }
  .ic {
    width: 40px;
    height: 40px;
    border-radius: 10px;
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
    gap: 2px;
  }
  .t span {
    font-size: 12px;
    color: var(--text-2);
  }
</style>
