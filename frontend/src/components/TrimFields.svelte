<script lang="ts" module>
  /** "83", "1:23", "01:02:03,5" → seconds; null when invalid, 0 when empty. */
  export function parseTime(s: string): number | null {
    s = s.trim().replace(',', '.')
    if (!s) return 0
    const parts = s.split(':')
    if (parts.length > 3) return null
    let total = 0
    for (let i = 0; i < parts.length; i++) {
      const v = Number(parts[i])
      if (!isFinite(v) || v < 0 || (i > 0 && v >= 60) || parts[i] === '') return null
      total = total * 60 + v
    }
    return total
  }

  export function trimError(start: string, end: string): string {
    const s = parseTime(start)
    const e = parseTime(end)
    if (s === null || e === null) return 'format'
    if (e > 0 && e <= s) return 'order'
    return ''
  }
</script>

<script lang="ts">
  import { L } from '../lib/i18n.svelte'

  let { start = $bindable(), end = $bindable(), label = '' }: { start: string; end: string; label?: string } = $props()
  const err = $derived(trimError(start, end))
</script>

<div class="trim">
  {#if label}<span class="label">{label}</span>{/if}
  <div class="fields">
    <label>
      <span>{L('Mulai', 'Start')}</span>
      <input class="text-input" class:bad={err === 'format' && parseTime(start) === null} bind:value={start} placeholder="0:00" aria-label={L('Waktu mulai', 'Start time')} />
    </label>
    <span class="dash">–</span>
    <label>
      <span>{L('Selesai', 'End')}</span>
      <input class="text-input" class:bad={err !== '' && (err === 'order' || parseTime(end) === null)} bind:value={end} placeholder={L('akhir', 'end')} aria-label={L('Waktu selesai', 'End time')} />
    </label>
    {#if start || end}
      <button class="clear" onclick={() => { start = ''; end = '' }}>{L('Hapus', 'Clear')}</button>
    {/if}
  </div>
  {#if err === 'format'}
    <p class="hint bad-text">{L('Tulis waktu seperti 1:30 atau 00:01:30.', 'Write times like 1:30 or 00:01:30.')}</p>
  {:else if err === 'order'}
    <p class="hint bad-text">{L('Waktu selesai harus setelah waktu mulai.', 'The end must be after the start.')}</p>
  {/if}
</div>

<style>
  .trim {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .fields {
    display: flex;
    align-items: flex-end;
    gap: 8px;
  }
  label {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }
  label span {
    font-size: 11px;
    color: var(--text-3);
  }
  .dash {
    padding-bottom: 10px;
    color: var(--text-3);
  }
  .bad {
    border-color: var(--err);
  }
  .bad-text {
    color: var(--err);
  }
  .clear {
    height: 40px;
    padding: 0 6px;
    border: 0;
    background: none;
    font-size: 12px;
    font-weight: 700;
    color: var(--accent-text-2);
  }
</style>
