<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import { answerText, textPrompt } from '../lib/stores/prompt.svelte'

  let dialog: HTMLDialogElement
  let value = $state('')
  let input = $state<HTMLInputElement>()

  $effect(() => {
    if (textPrompt.open && dialog && !dialog.open) {
      value = textPrompt.value
      dialog.showModal()
      setTimeout(() => input?.select(), 30)
    }
    if (!textPrompt.open && dialog?.open) dialog.close()
  })

  function submit(e: Event) {
    e.preventDefault()
    if (value.trim()) answerText(value.trim())
  }
</script>

<dialog bind:this={dialog} onclose={() => textPrompt.open && answerText(null)} onclick={(e) => e.target === dialog && answerText(null)}>
  <form class="box" onsubmit={submit}>
    <b>{textPrompt.title}</b>
    <label class="label" for="kmb-text-prompt">{textPrompt.label}</label>
    <input id="kmb-text-prompt" bind:this={input} class="text-input" bind:value placeholder={textPrompt.placeholder} autocomplete="off" />
    <div class="foot">
      <button type="button" class="btn" onclick={() => answerText(null)}>{L('Batal', 'Cancel')}</button>
      <button type="submit" class="btn-accent" disabled={!value.trim()}>{textPrompt.ok || L('Simpan', 'Save')}</button>
    </div>
  </form>
</dialog>

<style>
  dialog {
    padding: 0;
    border: 1px solid var(--border-strong);
    border-radius: 16px;
    background: var(--surface);
    color: var(--text);
    width: min(460px, 90vw);
    box-shadow: 0 24px 64px rgba(0, 0, 0, 0.55);
  }
  dialog::backdrop {
    background: rgba(5, 6, 8, 0.6);
  }
  .box {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 18px 20px;
  }
  b {
    font-size: 15px;
  }
  .foot {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 4px;
  }
</style>
