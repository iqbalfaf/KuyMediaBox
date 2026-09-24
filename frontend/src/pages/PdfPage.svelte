<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import { onMount } from 'svelte'
  import PageHeader from '../components/PageHeader.svelte'
  import Icon from '../components/Icon.svelte'
  import BatchTool from './pdf/BatchTool.svelte'
  import CombineTool from './pdf/CombineTool.svelte'
  import PagesTool from './pdf/PagesTool.svelte'
  import EditorTool from './pdf/EditorTool.svelte'
  import CompareTool from './pdf/CompareTool.svelte'
  import { categories, pdfTools, toolById } from '../lib/pdfTools'
  import { initPdf, pdfNav } from '../lib/stores/pdf.svelte'

  onMount(() => {
    initPdf()
  })

  const tool = $derived(toolById(pdfNav.tool))

  const shown = $derived.by(() => {
    const q = pdfNav.query.trim().toLowerCase()
    if (!q) return pdfTools
    return pdfTools.filter((t) => `${t.name()} ${t.desc()} ${t.formats.join(' ')}`.toLowerCase().includes(q))
  })

  function open(id: string) {
    pdfNav.tool = id
  }
</script>

{#if tool}
  <PageHeader title={tool.name()} subtitle={tool.desc()} back={() => (pdfNav.tool = null)} backLabel={L('Semua alat PDF', 'All PDF tools')} />
  {#key tool.id}
    {#if tool.pattern === 'batch' || tool.pattern === 'web'}
      <BatchTool {tool} />
    {:else if tool.pattern === 'combine'}
      <CombineTool {tool} />
    {:else if tool.pattern === 'pages'}
      <PagesTool {tool} />
    {:else if tool.pattern === 'editor'}
      <EditorTool {tool} />
    {:else}
      <CompareTool {tool} />
    {/if}
  {/key}
{:else}
  <PageHeader title={L('Alat PDF', 'PDF Tools')} subtitle={L('29 alat PDF, semuanya diproses di komputer ini — file tidak pernah diunggah.', '29 PDF tools, all processed on this computer — files are never uploaded.')} />
  <div class="hub">
    <div class="search">
      <Icon name="search" size={16} />
      <input class="q" placeholder={L('Cari alat, mis. gabung, kompres, word…', 'Search tools, e.g. merge, compress, word…')} bind:value={pdfNav.query} />
      {#if pdfNav.query}<button class="clear" aria-label={L('Hapus pencarian', 'Clear search')} onclick={() => (pdfNav.query = '')}><Icon name="x" size={14} /></button>{/if}
    </div>
    {#each categories as c (c.id)}
      {@const list = shown.filter((t) => t.cat === c.id)}
      {#if list.length}
        <section>
          <h2>{c.name()}</h2>
          <div class="grid">
            {#each list as t (t.id)}
              {@const [bg, fg] = t.color.split('|')}
              <button class="tool" onclick={() => open(t.id)}>
                <span class="tile" style="background: {bg}; color: {fg}"><Icon name={t.icon} size={20} /></span>
                <span class="tt">
                  <span class="tn">{t.name()}</span>
                  <span class="td">{t.desc()}</span>
                </span>
              </button>
            {/each}
          </div>
        </section>
      {/if}
    {/each}
    {#if shown.length === 0}
      <p class="none">{L('Tidak ada alat yang cocok.', 'No matching tools.')}</p>
    {/if}
  </div>
{/if}

<style>
  .hub {
    flex-grow: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 0 24px 28px 28px;
    display: flex;
    flex-direction: column;
    gap: 22px;
  }
  .search {
    position: relative;
    display: flex;
    align-items: center;
    gap: 10px;
    height: 44px;
    padding: 0 14px;
    max-width: 520px;
    border-radius: 12px;
    border: 1px solid var(--border);
    background: var(--surface);
    color: var(--text-3);
    flex-shrink: 0;
  }
  .search:focus-within {
    border-color: var(--accent);
  }
  .q {
    flex-grow: 1;
    border: 0;
    background: transparent;
    font-size: 14px;
    outline: none;
  }
  .clear {
    border: 0;
    background: transparent;
    color: var(--text-3);
    display: flex;
    padding: 4px;
  }
  h2 {
    margin: 0 0 10px;
    font-size: 12px;
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--text-3);
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
    gap: 10px;
  }
  .tool {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 14px;
    border-radius: 14px;
    border: 1px solid var(--border);
    background: var(--surface);
    text-align: left;
    transition: border-color 0.15s, background 0.15s, transform 0.1s;
  }
  .tool:hover {
    border-color: var(--border-strong);
    background: #1c2029;
  }
  .tool:active {
    transform: translateY(1px);
  }
  .tile {
    width: 42px;
    height: 42px;
    flex-shrink: 0;
    border-radius: 12px;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .tt {
    display: flex;
    flex-direction: column;
    gap: 3px;
    min-width: 0;
  }
  .tn {
    font-size: 14px;
    font-weight: 700;
  }
  .td {
    font-size: 12px;
    line-height: 1.45;
    color: var(--text-3);
  }
  .none {
    color: var(--text-3);
    font-size: 13px;
  }
</style>
