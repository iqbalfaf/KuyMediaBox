<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'
  import { answerPassword, pwPrompt } from '../lib/stores/prompt.svelte'

  let dialog: HTMLDialogElement
  let value = $state('')
  let show = $state(false)
  let input = $state<HTMLInputElement>()

  $effect(() => {
    if (pwPrompt.open && dialog && !dialog.open) {
      value = ''
      show = false
      dialog.showModal()
      setTimeout(() => input?.focus(), 30)
    }
    if (!pwPrompt.open && dialog?.open) dialog.close()
  })

  function submit(e: Event) {
    e.preventDefault()
    if (value) answerPassword(value)
  }
</script>

<dialog bind:this={dialog} onclose={() => pwPrompt.open && answerPassword(null)} onclick={(e) => e.target === dialog && answerPassword(null)}>
  <form class="box" onsubmit={submit}>
    <div class="head">
      <span class="ic"><Icon name="lock" /></span>
      <div class="t">
        <b>{L('PDF dikunci password', 'Password protected PDF')}</b>
        <span class="ellipsis" title={pwPrompt.name}>{pwPrompt.name}</span>
      </div>
    </div>
    <div class="field">
      <input bind:this={input} class="text-input" type={show ? 'text' : 'password'} bind:value placeholder={L('Password untuk membuka', 'Password to open')} autocomplete="off" />
      <button type="button" class="btn icon" aria-label={show ? L('Sembunyikan', 'Hide') : L('Tampilkan', 'Show')} onclick={() => (show = !show)}><Icon name={show ? 'eyeOff' : 'search'} size={16} /></button>
    </div>
    {#if pwPrompt.wrong}<p class="bad">{L('Password salah, coba lagi.', 'Wrong password, try again.')}</p>{/if}
    <p class="hint">{L('Password hanya dipakai di komputer ini dan tidak disimpan.', 'The password stays on this computer and is not saved.')}</p>
    <div class="foot">
      <button type="button" class="btn" onclick={() => answerPassword(null)}>{L('Batal', 'Cancel')}</button>
      <button type="submit" class="btn-accent" disabled={!value}>{L('Buka', 'Open')}</button>
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
    width: min(440px, 90vw);
    box-shadow: 0 24px 64px rgba(0, 0, 0, 0.55);
  }
  dialog::backdrop {
    background: rgba(5, 6, 8, 0.6);
  }
  .box {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 18px 20px;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .ic {
    width: 36px;
    height: 36px;
    border-radius: 10px;
    background: var(--accent-tint);
    color: var(--accent);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }
  .t {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .t span {
    font-size: 12px;
    color: var(--text-3);
  }
  .field {
    display: flex;
    gap: 8px;
  }
  .field .btn.icon {
    height: 40px;
    width: 40px;
  }
  .bad {
    margin: 0;
    font-size: 12px;
    font-weight: 600;
    color: var(--err);
  }
  .foot {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
</style>
