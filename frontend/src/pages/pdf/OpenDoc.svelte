<script lang="ts">
  import { L } from '../../lib/i18n.svelte'
  import Icon from '../../components/Icon.svelte'

  let { title, subtitle, busy = false, onpick }: { title: string; subtitle: string; busy?: boolean; onpick: () => void } = $props()
</script>

<section class="card wrap" aria-label={title}>
  <div class="zone">
    <div class="big-ic">
      {#if busy}<Icon name="loader" size={36} stroke={1.8} class="spin" />{:else}<Icon name="upload" size={36} stroke={1.8} />{/if}
    </div>
    <div class="txt">
      <h2>{busy ? L('Membuka PDF…', 'Opening the PDF…') : title}</h2>
      <p>{subtitle}</p>
    </div>
    <button class="btn-accent big" onclick={onpick} disabled={busy}><Icon name="plus" size={16} stroke={2.5} />{L('Pilih PDF', 'Choose a PDF')}</button>
  </div>
</section>

<style>
  .wrap {
    flex-grow: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    padding: 16px;
  }
  .zone {
    flex-grow: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 18px;
    border: 2px dashed var(--border-strong);
    border-radius: 14px;
    background: #161a20;
    padding: 20px;
    text-align: center;
  }
  .big-ic {
    width: 80px;
    height: 80px;
    border-radius: 24px;
    background: var(--accent-tint);
    color: var(--accent);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .txt {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  h2 {
    margin: 0;
    font-size: 22px;
    font-weight: 800;
  }
  p {
    margin: 0;
    font-size: 14px;
    color: var(--text-2);
    max-width: 460px;
  }
  .big {
    height: 44px;
    padding: 0 18px;
    border-radius: 12px;
    font-size: 14px;
  }
</style>
