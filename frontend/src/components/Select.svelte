<script lang="ts" generics="T extends string | number">
  import Icon from './Icon.svelte'

  let {
    options,
    value = $bindable(),
    label,
    onchange,
  }: {
    options: { value: T; label: string; disabled?: boolean }[]
    value: T
    label: string
    onchange?: (v: T) => void
  } = $props()
</script>

<div class="wrap">
  <select
    aria-label={label}
    value={String(value)}
    onchange={(e) => {
      const raw = (e.currentTarget as HTMLSelectElement).value
      const found = options.find((o) => String(o.value) === raw)
      if (found) {
        value = found.value
        onchange?.(found.value)
      }
    }}
  >
    {#each options as o (o.value)}
      <option value={String(o.value)} disabled={o.disabled}>{o.label}</option>
    {/each}
  </select>
  <span class="chev"><Icon name="chevronDown" size={16} /></span>
</div>

<style>
  .wrap {
    position: relative;
    flex-grow: 1;
    min-width: 0;
  }
  select {
    width: 100%;
    height: 40px;
    appearance: none;
    background: var(--surface-2);
    border: 1px solid var(--border);
    border-radius: 10px;
    color: var(--text);
    padding: 0 34px 0 12px;
    font-size: 13px;
    font-weight: 600;
  }
  select:focus {
    outline: none;
    border-color: var(--accent);
  }
  option {
    background: #1f242d;
    color: var(--text);
  }
  .chev {
    position: absolute;
    right: 12px;
    top: 12px;
    pointer-events: none;
    color: var(--text-3);
  }
</style>
