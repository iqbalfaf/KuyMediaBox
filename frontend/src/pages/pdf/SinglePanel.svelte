<script lang="ts">
  import { L } from '../../lib/i18n.svelte'
  import type { Snippet } from 'svelte'
  import OutputPicker from '../../components/OutputPicker.svelte'
  import RunFooter from '../../components/RunFooter.svelte'
  import Icon from '../../components/Icon.svelte'
  import { api } from '../../lib/api'
  import { showDetail } from '../../lib/stores/app.svelte'
  import { single } from '../../lib/stores/pdf.svelte'
  import { isActive, tasks } from '../../lib/stores/tasks.svelte'
  import { bytes } from '../../lib/format'
  import type { PdfTool } from '../../lib/pdfTools'

  // Settings panel of a tool that edits one open document: options, output folder, the
  // result of the last run and the start button.
  let {
    tool,
    startLabel,
    blocked = '',
    onstart,
    children,
  }: { tool: PdfTool; startLabel: string; blocked?: string; onstart: () => Promise<void>; children?: Snippet } = $props()

  const task = $derived(single[tool.id] ? tasks[single[tool.id]] : undefined)
  let starting = $state(false)

  async function start() {
    starting = true
    try {
      await onstart()
    } finally {
      starting = false
    }
  }
</script>

<aside class="card panel" aria-label={L('Pengaturan', 'Settings')}>
  <div class="scroll">
    {@render children?.()}
    <div class="sec">
      <span class="label">{L('Simpan ke', 'Save to')}</span>
      <OutputPicker kind="pdf" />
    </div>
    {#if task && !isActive(task)}
      <div class="res {task.status}">
        {#if task.status === 'done'}
          <Icon name="check" size={16} stroke={3} /><span class="ellipsis" title={task.output}>{task.output.split(/[\\/]/).pop()} · {bytes(task.outSize)}</span>
          <button class="link" onclick={() => api.revealFile(task.output)}>{L('Lihat', 'Show')}</button>
        {:else if task.status === 'failed'}
          <Icon name="alert" size={16} /><span class="ellipsis">{task.message}</span>
          <button class="link" onclick={() => showDetail(tool.name(), task.message, task.detail)}>{L('Detail', 'Details')}</button>
        {:else}
          <span>{task.message}</span>
        {/if}
      </div>
    {/if}
  </div>
  <RunFooter
    kind="pdf"
    running={isActive(task) || starting}
    summary={!!task}
    busy={starting}
    {startLabel}
    disabled={!!blocked}
    disabledHint={blocked}
    lastOutput={task?.output ?? ''}
    onstart={start}
    oncancel={() => task && api.cancelTask(task.id)}
  />
</aside>

<style>
  .panel {
    width: 344px;
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .scroll {
    flex-grow: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 18px 20px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  .sec {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .res {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 12px;
    border-radius: 10px;
    font-size: 12px;
    font-weight: 600;
    min-width: 0;
  }
  .res.done {
    background: var(--ok-soft);
    color: var(--ok);
  }
  .res.failed {
    background: var(--err-soft);
    color: var(--err);
  }
  .res span {
    flex-grow: 1;
  }
  .link {
    padding: 0;
    border: 0;
    background: none;
    font-size: 12px;
    font-weight: 700;
    color: inherit;
    text-decoration: underline;
  }
</style>
