<script lang="ts">
  import Icon from './Icon.svelte'
  import { api, errText } from '../lib/api'
  import { nav, toast, tool, toolState } from '../lib/stores/app.svelte'

  let { ids, why }: { ids: string[]; why: string } = $props()

  const missing = $derived(toolState.loaded ? ids.map((id) => tool(id)).filter((t) => t && !t.found) : [])
  const busy = $derived(missing.some((t) => t!.busy))
  const progress = $derived(missing.length ? missing.reduce((a, t) => a + (t!.busy ? t!.progress : 0), 0) / missing.length : 0)

  async function installAll() {
    for (const t of missing) {
      try {
        await api.installTool(t!.id)
        toast(`${t!.name} berhasil dipasang`, 'ok')
      } catch (e) {
        toast(`${t!.name}: ${errText(e)}`, 'err')
        return
      }
    }
  }
</script>

{#if missing.length > 0}
  <div class="banner" role="status">
    <Icon name="alert" />
    <div class="txt">
      <b>{missing.map((t) => t!.name).join(' & ')} belum terpasang.</b>
      <span>{why}</span>
    </div>
    {#if busy}
      <div class="prog"><div class="bar"><div style="width: {progress * 100}%"></div></div><span>{Math.round(progress * 100)}%</span></div>
    {:else}
      <button class="btn-accent" onclick={installAll}><Icon name="download" size={14} stroke={2.5} />Unduh sekarang</button>
      <button class="btn" onclick={() => (nav.page = 'settings')}>Pengaturan</button>
    {/if}
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
    background: #2b2112;
    border: 1px solid #5c4520;
    color: var(--warn);
    flex-shrink: 0;
  }
  .txt {
    flex-grow: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
    font-size: 13px;
    min-width: 0;
  }
  .txt b {
    color: #ffe0b0;
  }
  .txt span {
    color: #d9c3a0;
    font-size: 12px;
  }
  .prog {
    width: 180px;
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    font-weight: 700;
  }
  .prog .bar {
    flex-grow: 1;
  }
</style>
