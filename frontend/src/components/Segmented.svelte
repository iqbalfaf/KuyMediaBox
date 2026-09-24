<script lang="ts" generics="T extends string | number">
  let {
    options,
    value = $bindable(),
    onchange,
    label = '',
  }: {
    options: { value: T; label: string; icon?: string; disabled?: boolean }[]
    value: T
    onchange?: (v: T) => void
    label?: string
  } = $props()

  import Icon from './Icon.svelte'
</script>

<div class="seg" role="radiogroup" aria-label={label} style="grid-template-columns: repeat({options.length}, minmax(0, 1fr))">
  {#each options as o (o.value)}
    <button
      role="radio"
      aria-checked={o.value === value}
      class:on={o.value === value}
      disabled={o.disabled}
      onclick={() => {
        value = o.value
        onchange?.(o.value)
      }}
    >
      {#if o.icon}<Icon name={o.icon} size={16} />{/if}
      {o.label}
    </button>
  {/each}
</div>

<style>
  .seg {
    display: grid;
    gap: 4px;
    padding: 4px;
    border-radius: 12px;
    background: var(--inset);
    border: 1px solid var(--border);
  }
  button {
    height: 34px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--text-2);
    font-size: 13px;
    font-weight: 600;
    white-space: nowrap;
    padding: 0 4px;
    transition: background 0.15s, color 0.15s;
  }
  button:hover:not(.on):not(:disabled) {
    color: var(--text);
  }
  button.on {
    background: #2a303b;
    color: var(--text);
    font-weight: 700;
  }
  button:disabled {
    opacity: 0.35;
  }
</style>
