<script lang="ts">
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
    onstart,
    oncancel,
  }: {
    queueMore?: number
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
  const verb = $derived(kind === 'download' ? 'Mengunduh' : 'Mengonversi')

  function openOutput() {
    if (lastOutput) api.revealFile(lastOutput)
  }
</script>

<div class="footer">
  {#if running}
    <div class="line">
      <span class="t">{verb} {Math.min(sum.total, sum.finished + 1)} dari {sum.total}</span>
      <span class="eta">{sum.etaSeconds ? `Sisa ± ${eta(sum.etaSeconds)}` : 'Menghitung…'}</span>
    </div>
    <div class="bar big"><div style="width: {sum.progress * 100}%"></div></div>
    {#if queueMore > 0}
      <button class="btn-accent more" onclick={onstart} disabled={busy}>
        <Icon name="plus" size={14} stroke={2.5} />Tambah ke antrian ({queueMore})
      </button>
    {/if}
    <div class="btns">
      <button class="btn grow" onclick={openOutput} disabled={!lastOutput}>Buka folder hasil</button>
      <button class="btn danger grow" onclick={oncancel}>Batalkan semua</button>
    </div>
  {:else}
    {#if sum.total > 0 && sum.finished === sum.total}
      <div class="summary">
        <span>
          Terakhir: <b class="ok">{sum.done} berhasil</b>{#if sum.failed}&nbsp;· <b class="err">{sum.failed} gagal</b>{/if}{#if sum.skipped}&nbsp;· {sum.skipped} dilewati{/if}{#if sum.canceled}&nbsp;· {sum.canceled} dibatalkan{/if}
        </span>
        {#if lastOutput}<button class="link" onclick={openOutput}>Buka folder</button>{/if}
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
    background: #1a1e26;
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
