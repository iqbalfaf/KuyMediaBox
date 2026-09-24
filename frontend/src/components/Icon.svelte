<script lang="ts">
  import { filled, icons } from '../lib/icons'

  let { name, size = 18, stroke = 2, class: cls = '' }: { name: string; size?: number; stroke?: number; class?: string } = $props()
  const isFilled = $derived(name in filled)
</script>

{#if isFilled}
  <svg class={cls} width={size} height={size} viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">{@html filled[name]}</svg>
{:else}
  <svg
    class={cls}
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width={stroke}
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true">{@html icons[name] ?? ''}</svg
  >
{/if}

<style>
  svg {
    flex-shrink: 0;
    display: block;
  }
</style>
