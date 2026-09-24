<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'
  import { dismiss, toasts } from '../lib/stores/app.svelte'
</script>

<div class="stack" aria-live="polite">
  {#each toasts as t (t.id)}
    <div class="toast {t.tone}">
      <Icon name={t.tone === 'ok' ? 'check' : t.tone === 'err' ? 'alert' : 'info'} size={16} />
      <span>{t.text}</span>
      <button aria-label={L('Tutup', 'Close')} onclick={() => dismiss(t.id)}><Icon name="x" size={14} /></button>
    </div>
  {/each}
</div>

<style>
  .stack {
    position: fixed;
    right: 20px;
    bottom: 20px;
    z-index: 50;
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-width: 420px;
  }
  .toast {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 12px 10px 12px 14px;
    border-radius: 12px;
    background: #20252e;
    border: 1px solid var(--border-strong);
    box-shadow: 0 12px 32px rgba(0, 0, 0, 0.45);
    font-size: 13px;
    line-height: 1.45;
    animation: in 0.18s ease-out;
    user-select: text;
  }
  .toast :global(svg) {
    margin-top: 1px;
  }
  .toast.ok {
    color: var(--ok);
  }
  .toast.err {
    color: var(--err);
  }
  .toast.info {
    color: var(--info);
  }
  .toast span {
    flex-grow: 1;
    color: var(--text);
  }
  button {
    border: 0;
    background: none;
    color: var(--text-3);
    padding: 2px;
    border-radius: 6px;
  }
  button:hover {
    color: var(--text);
  }
  @keyframes in {
    from {
      transform: translateY(8px);
      opacity: 0;
    }
  }
</style>
