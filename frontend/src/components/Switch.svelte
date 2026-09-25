<script lang="ts">
  let {
    checked = $bindable(false),
    label,
    hint = '',
    onchange,
  }: { checked: boolean; label: string; hint?: string; onchange?: (v: boolean) => void } = $props()
</script>

<div class="row">
  <div class="text">
    <span class="label-text">{label}</span>
    {#if hint}<span class="hint-text">{hint}</span>{/if}
  </div>
  <button
    role="switch"
    aria-checked={checked}
    aria-label={label}
    class:on={checked}
    onclick={() => {
      checked = !checked
      onchange?.(checked)
    }}
  >
    <span></span>
  </button>
</div>

<style>
  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }
  .text {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .label-text {
    font-size: 13px;
    font-weight: 600;
  }
  .hint-text {
    font-size: 12px;
    color: var(--text-3);
  }
  button {
    width: 42px;
    height: 24px;
    flex-shrink: 0;
    padding: 0;
    border: 0;
    border-radius: 999px;
    background: var(--switch-off);
    position: relative;
    transition: background 0.15s;
  }
  button span {
    position: absolute;
    top: 3px;
    left: 3px;
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: var(--text-2);
    transition: left 0.15s, background 0.15s;
  }
  button.on {
    background: var(--accent);
  }
  button.on span {
    left: 21px;
    background: var(--accent-ink);
  }
</style>
