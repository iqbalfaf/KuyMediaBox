<script lang="ts" generics="T extends string | number">
  let {
    options,
    value = $bindable(),
    columns = 4,
    small = false,
    tall = false,
    onchange,
  }: {
    options: { value: T; label: string; sub?: string; disabled?: boolean; title?: string }[]
    value: T
    columns?: number
    small?: boolean
    tall?: boolean
    onchange?: (v: T) => void
  } = $props()
</script>

<div class="chips" style="grid-template-columns: repeat({columns}, minmax(0, 1fr))">
  {#each options as o (o.value)}
    <button
      class="chip"
      class:on={o.value === value}
      class:small
      class:tall
      disabled={o.disabled}
      title={o.title ?? ''}
      aria-pressed={o.value === value}
      onclick={() => {
        value = o.value
        onchange?.(o.value)
      }}
    >
      <span>{o.label}</span>
      {#if o.sub !== undefined}<span class="sub">{o.sub}</span>{/if}
    </button>
  {/each}
</div>

<style>
  .chips {
    display: grid;
    gap: 8px;
  }
  .chip {
    height: 36px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 1px;
    border-radius: 10px;
    border: 1px solid var(--border);
    background: var(--surface-2);
    color: var(--text-4);
    font-size: 13px;
    font-weight: 600;
    transition: border-color 0.15s, background 0.15s;
  }
  .chip.small {
    font-size: 12px;
  }
  .chip.tall {
    height: 40px;
    font-size: 12px;
  }
  .chip:hover:not(:disabled):not(.on) {
    border-color: var(--border-strong);
    background: #252a33;
  }
  .chip.on {
    border-color: var(--accent);
    background: var(--accent-soft);
    color: var(--accent-text);
    font-weight: 700;
  }
  .chip:disabled {
    opacity: 0.35;
  }
  .sub {
    font-size: 10px;
    font-weight: 600;
    color: var(--text-3);
  }
  .chip.on .sub {
    color: var(--accent-text-2);
  }
</style>
