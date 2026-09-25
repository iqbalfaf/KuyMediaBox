<script lang="ts">
  import { L } from '../lib/i18n.svelte'

  let { value = $bindable(), rows = ['t', 'm', 'b'], label }: { value: string; rows?: string[]; label: string } = $props()

  const cols = ['l', 'c', 'r']
  const rowName: Record<string, () => string> = { t: () => L('atas', 'top'), m: () => L('tengah', 'middle'), b: () => L('bawah', 'bottom') }
  const colName: Record<string, () => string> = { l: () => L('kiri', 'left'), c: () => L('tengah', 'centre'), r: () => L('kanan', 'right') }
</script>

<div class="grid" role="radiogroup" aria-label={label} style="grid-template-rows: repeat({rows.length}, 22px)">
  {#each rows as r}
    {#each cols as c}
      {@const v = r + c}
      <button role="radio" aria-checked={value === v} aria-label={`${rowName[r]()} ${colName[c]()}`} title={`${rowName[r]()} ${colName[c]()}`} class:on={value === v} onclick={() => (value = v)}><span></span></button>
    {/each}
  {/each}
</div>

<style>
  .grid {
    display: grid;
    grid-template-columns: repeat(3, 34px);
    gap: 4px;
    padding: 6px;
    border-radius: 10px;
    background: var(--inset);
    border: 1px solid var(--border);
    width: max-content;
  }
  button {
    border: 0;
    border-radius: 5px;
    background: var(--surface-2);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0;
  }
  button span {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--dot);
  }
  button:hover span {
    background: var(--text-3);
  }
  button.on {
    background: var(--accent-soft);
  }
  button.on span {
    background: var(--accent);
  }
</style>
