<script lang="ts">
  import Icon from './Icon.svelte'
  import { nav, toolAttention, toolState } from '../lib/stores/app.svelte'
  import { clock, summarize } from '../lib/stores/tasks.svelte'
  import { upd } from '../lib/stores/update.svelte'
  import type { Kind, Page } from '../lib/types'

  const kindName: Record<Kind, string> = { image: 'Gambar', video: 'Video', audio: 'Audio', download: 'Download' }
  const unit: Record<Kind, string> = { image: 'file', video: 'file', audio: 'file', download: 'item' }

  const queue = $derived.by(() => {
    void clock.now
    const kinds: Kind[] = ['download', 'video', 'audio', 'image']
    const active = kinds.map((k) => ({ k, s: summarize(k) })).filter((x) => x.s.active > 0)
    if (active.length === 0) return null
    const main = active[0]
    const total = active.reduce((a, x) => a + x.s.total, 0)
    const progress = active.reduce((a, x) => a + x.s.progress * x.s.total, 0) / Math.max(1, total)
    let sub = `${kindName[main.k]} · ${main.s.finished + 1 > main.s.total ? main.s.total : main.s.finished + 1} dari ${main.s.total} ${unit[main.k]}`
    if (active.length > 1) sub += ` · +${active.length - 1} lainnya`
    return { progress, sub }
  })

  const attention = $derived(toolAttention())

  const items: { page: Page; label: string; icon: string; group: string }[] = [
    { page: 'image', label: 'Gambar', icon: 'image', group: 'KONVERSI' },
    { page: 'video', label: 'Video', icon: 'video', group: 'KONVERSI' },
    { page: 'audio', label: 'Audio', icon: 'music', group: 'KONVERSI' },
    { page: 'download', label: 'YouTube & Spotify', icon: 'download', group: 'UNDUH' },
  ]
</script>

<nav aria-label="Navigasi utama">
  <div class="brand drag">
    <div class="logo"><Icon name="box" size={20} stroke={2.2} /></div>
    <div class="name">
      <span class="title">KuyMediaBox</span>
      <span class="ver">{upd.version ? `v${upd.version}` : ''}</span>
    </div>
  </div>

  {#each items as it, i}
    {#if i === 0 || items[i - 1].group !== it.group}
      <div class="group" class:first={i === 0}>{it.group}</div>
    {/if}
    <button class="item" class:active={nav.page === it.page} onclick={() => (nav.page = it.page)}>
      <span class="ic"><Icon name={it.icon} /></span>{it.label}
    </button>
  {/each}

  <div class="spacer drag"></div>

  <div class="queue">
    <div class="q-head">
      <span class="q-title">{queue ? 'Antrian berjalan' : 'Antrian kosong'}</span>
      <span class="q-pct">{queue ? `${Math.round(queue.progress * 100)}%` : '—'}</span>
    </div>
    <div class="bar"><div style="width: {queue ? queue.progress * 100 : 0}%"></div></div>
    <span class="q-sub">{queue ? queue.sub : 'Belum ada tugas'}</span>
  </div>

  {#if upd.info?.available}
    <button class="tools-link app-upd" onclick={() => (upd.open = true)}>
      <span class="dot"></span>Update v{upd.info.latest} tersedia
    </button>
  {/if}
  {#if toolState.loaded && attention.missing + attention.updates > 0}
    <button class="tools-link" class:bad={attention.missing > 0} onclick={() => (nav.page = 'settings')}>
      <span class="dot"></span>
      {#if attention.missing > 0}
        {attention.missing} tools belum terpasang
      {:else}
        {attention.updates} update tools tersedia
      {/if}
    </button>
  {/if}
  <button class="item" class:active={nav.page === 'settings'} onclick={() => (nav.page = 'settings')}>
    <span class="ic"><Icon name="sliders" /></span>Pengaturan
  </button>
</nav>

<style>
  nav {
    width: 232px;
    flex-shrink: 0;
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 20px 14px;
    background: var(--sidebar);
    border-right: 1px solid var(--border-soft);
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 4px 8px 20px;
  }
  .logo {
    width: 36px;
    height: 36px;
    border-radius: 10px;
    background: var(--accent);
    color: var(--accent-ink);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .name {
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  .title {
    font-size: 16px;
    font-weight: 800;
    letter-spacing: -0.01em;
  }
  .ver {
    font-size: 11px;
    color: var(--text-3);
  }
  .group {
    padding: 18px 12px 6px;
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.08em;
    color: var(--text-3);
  }
  .group.first {
    padding-top: 6px;
  }
  .item {
    display: flex;
    align-items: center;
    gap: 12px;
    height: 44px;
    padding: 0 12px;
    border: 0;
    border-radius: 10px;
    background: transparent;
    color: var(--text-2);
    font-size: 14px;
    font-weight: 600;
    text-align: left;
    transition: background 0.15s, color 0.15s;
  }
  .item:hover {
    background: #181c23;
    color: var(--text);
  }
  .item .ic {
    display: flex;
    color: var(--text-3);
  }
  .item.active {
    background: var(--accent-tint);
    color: #ffd9c8;
  }
  .item.active .ic {
    color: var(--accent);
  }
  .spacer {
    flex-grow: 1;
  }
  .queue {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 14px;
    border-radius: 12px;
    background: var(--surface);
    border: 1px solid var(--border-soft);
    margin-bottom: 8px;
  }
  .q-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .q-title {
    font-size: 12px;
    font-weight: 700;
  }
  .q-pct {
    font-size: 12px;
    font-weight: 700;
    color: var(--accent-text-2);
    font-variant-numeric: tabular-nums;
  }
  .q-sub {
    font-size: 12px;
    color: var(--text-2);
  }
  .tools-link {
    display: flex;
    align-items: center;
    gap: 10px;
    height: 36px;
    padding: 0 12px;
    border: 0;
    border-radius: 10px;
    background: transparent;
    color: var(--warn);
    font-size: 12px;
    font-weight: 600;
    text-align: left;
  }
  .tools-link:hover {
    background: #181c23;
  }
  .tools-link .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--warn-dot);
  }
  .tools-link.app-upd {
    color: var(--accent-text);
  }
  .tools-link.app-upd .dot {
    background: var(--accent);
  }
  .tools-link.bad {
    color: var(--err);
  }
  .tools-link.bad .dot {
    background: #ff6b6b;
  }
</style>
