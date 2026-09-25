<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'
  import { api } from '../lib/api'
  import { clock, summarize } from '../lib/stores/tasks.svelte'
  import { eta } from '../lib/format'
  import type { Kind } from '../lib/types'

  let {
    kind,
    running,
    startLabel,
    startIcon = 'play',
    disabled = false,
    disabledHint = '',
    busy = false,
    note = '',
    lastOutput = '',
    queueMore = 0,
    summary = true,
    onstart,
    oncancel,
  }: {
    queueMore?: number
    /** Show the result of the last run (off when it would describe another tool). */
    summary?: boolean
    kind: Kind
    running: boolean
    startLabel: string
    startIcon?: string
    disabled?: boolean
    disabledHint?: string
    busy?: boolean
    note?: string
    lastOutput?: string
    onstart: () => void
    oncancel: () => void
  } = $props()

  const sum = $derived.by(() => {
    void clock.now
    return summarize(kind)
  })
  const verb = $derived(kind === 'download' ? L('Mengunduh', 'Downloading') : L('Mengonversi', 'Converting'))

  function openOutput() {
    if (lastOutput) api.revealFile(lastOutput)
  }
</script>

<div class="footer">
  {#if running}
    <div class="line">
      <span class="t">{verb} {Math.min(sum.total, sum.finished + 1)} {L('dari', 'of')} {sum.total}</span>
      <span class="eta">{sum.etaSeconds ? `${L('Sisa', 'About')} ± ${eta(sum.etaSeconds)}${L('', ' left')}` : L('Menghitung…', 'Estimating…')}</span>
    </div>
    <div class="bar big"><div style="width: {sum.progress * 100}%"></div></div>
    {#if queueMore > 0}
      <button class="btn-accent more" onclick={onstart} disabled={busy}>
        <Icon name="plus" size={14} stroke={2.5} />{L('Tambah ke antrian', 'Add to queue')} ({queueMore})
      </button>
    {/if}
    <div class="btns">
      <button class="btn grow" onclick={openOutput} disabled={!lastOutput}>{L('Buka folder hasil', 'Open output folder')}</button>
      <button class="btn danger grow" onclick={oncancel}>{L('Batalkan semua', 'Cancel all')}</button>
    </div>
  {:else}
    {#if summary && sum.total > 0 && sum.finished === sum.total}
      <div class="summary">
        <span>
          {L('Terakhir', 'Last run')}: <b class="ok">{sum.done} {L('berhasil', 'succeeded')}</b>{#if sum.failed}&nbsp;· <b class="err">{sum.failed} {L('gagal', 'failed')}</b>{/if}{#if sum.skipped}&nbsp;· {sum.skipped} {L('dilewati', 'skipped')}{/if}{#if sum.canceled}&nbsp;· {sum.canceled} {L('dibatalkan', 'canceled')}{/if}
        </span>
        {#if lastOutput}<button class="link" onclick={openOutput}>{L('Buka folder', 'Open folder')}</button>{/if}
      </div>
    {/if}
    <button class="btn-primary" disabled={disabled || busy} onclick={onstart}>
      {#if busy}<Icon name="loader" size={18} class="spin" />{:else}<Icon name={startIcon} size={startIcon === 'play' ? 16 : 18} stroke={2.5} />{/if}
      {startLabel}
    </button>
    {#if disabled && disabledHint}<p class="hint center">{disabledHint}</p>{/if}
    {#if note}<p class="hint center">{note}</p>{/if}
  {/if}
</div>

<style>
  .footer {
    padding: 16px 20px 20px;
    border-top: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: 10px;
    background: var(--surface-3);
    flex-shrink: 0;
  }
  .line {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    gap: 8px;
  }
  .t {
    font-size: 14px;
    font-weight: 700;
  }
  .eta {
    font-size: 12px;
    color: var(--text-2);
    white-space: nowrap;
  }
  .big {
    height: 8px;
  }
  .btns {
    display: flex;
    gap: 8px;
    margin-top: 4px;
  }
  .more {
    width: 100%;
    justify-content: center;
    height: 40px;
    margin-top: 4px;
  }
  .grow {
    flex: 1;
    height: 40px;
    font-weight: 700;
  }
  .summary {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    color: var(--text-2);
  }
  .summary b.ok {
    color: var(--ok);
  }
  .summary b.err {
    color: var(--err);
  }
  .link {
    padding: 0;
    border: 0;
    background: none;
    font-size: 12px;
    font-weight: 700;
    color: var(--accent-text-2);
    white-space: nowrap;
  }
  .center {
    text-align: center;
  }
</style>
