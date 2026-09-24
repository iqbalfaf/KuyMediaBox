<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'
  import { runtime } from '../lib/api'
  import { detail, toast } from '../lib/stores/app.svelte'

  let dialog: HTMLDialogElement

  $effect(() => {
    if (detail.open && dialog && !dialog.open) dialog.showModal()
    if (!detail.open && dialog?.open) dialog.close()
  })

  async function copy() {
    const text = `${detail.title}\n${detail.message}\n\n${detail.text}`
    try {
      await runtime.clipboardSet(text)
      toast(L('Detail disalin', 'Details copied'), 'ok')
    } catch {
      toast(L('Gagal menyalin', "Couldn't copy"), 'err')
    }
  }
</script>

<dialog bind:this={dialog} onclose={() => (detail.open = false)} onclick={(e) => e.target === dialog && (detail.open = false)}>
  <div class="box">
    <div class="head">
      <span class="ic"><Icon name="alert" /></span>
      <div class="t">
        <b class="ellipsis">{detail.title}</b>
        <span>{detail.message}</span>
      </div>
      <button class="btn icon" aria-label={L('Tutup', 'Close')} onclick={() => (detail.open = false)}><Icon name="x" size={16} /></button>
    </div>
    <pre>{detail.text || L('Tidak ada detail tambahan.', 'No additional details.')}</pre>
    <div class="foot">
      <button class="btn" onclick={copy}><Icon name="copy" size={16} />{L('Salin detail', 'Copy details')}</button>
      <button class="btn-accent" onclick={() => (detail.open = false)}>{L('Tutup', 'Close')}</button>
    </div>
  </div>
</dialog>

<style>
  dialog {
    padding: 0;
    border: 1px solid var(--border-strong);
    border-radius: 16px;
    background: var(--surface);
    color: var(--text);
    width: min(720px, 90vw);
    box-shadow: 0 24px 64px rgba(0, 0, 0, 0.55);
  }
  dialog::backdrop {
    background: rgba(5, 6, 8, 0.6);
  }
  .box {
    display: flex;
    flex-direction: column;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 16px 16px 12px 20px;
  }
  .ic {
    width: 36px;
    height: 36px;
    border-radius: 10px;
    background: var(--err-soft);
    color: var(--err);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }
  .t {
    flex-grow: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .t span {
    font-size: 13px;
    color: var(--err);
  }
  pre {
    margin: 0 20px;
    padding: 14px;
    max-height: 50vh;
    overflow: auto;
    border-radius: 10px;
    background: var(--inset);
    border: 1px solid var(--border);
    font-family: var(--mono);
    font-size: 12px;
    line-height: 1.55;
    color: var(--text-4);
    white-space: pre-wrap;
    word-break: break-word;
    user-select: text;
  }
  .foot {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding: 14px 20px 18px;
  }
</style>
