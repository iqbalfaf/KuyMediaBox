<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'
  import type { Converter } from '../lib/stores/converter.svelte'

  let {
    conv,
    title,
    subtitle,
    pickLabel,
    formats,
    steps,
  }: {
    conv: Converter
    title: string
    subtitle: string
    pickLabel: string
    formats: string[]
    steps: [string, string][]
  } = $props()
</script>

<section class="card wrap" aria-label={L('Tambahkan file', 'Add files')}>
  <div class="zone">
    <div class="big-ic">
      {#if conv.adding}<Icon name="loader" size={36} stroke={1.8} class="spin" />{:else}<Icon name="upload" size={36} stroke={1.8} />{/if}
    </div>
    <div class="txt">
      <h2>{conv.adding ? L('Membaca file…', 'Reading files…') : title}</h2>
      <p>{subtitle}</p>
    </div>
    <div class="btns">
      <button class="btn-accent big" onclick={() => conv.pickFiles()} disabled={conv.adding}><Icon name="plus" size={16} stroke={2.5} />{pickLabel}</button>
      <button class="btn big" onclick={() => conv.pickFolder()} disabled={conv.adding}><Icon name="folder" size={16} />{L('Pilih folder', 'Choose folder')}</button>
    </div>
    <div class="fmts">
      {#each formats as f}<span>{f}</span>{/each}
    </div>
  </div>
  <ol>
    {#each steps as [a, b], i}
      <li>
        <span class="num">{i + 1}</span>
        <span class="st"><span class="s1">{a}</span><span class="s2">{b}</span></span>
      </li>
    {/each}
  </ol>
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
    align-items: center;
    gap: 6px;
    text-align: center;
  }
  h2 {
    margin: 0;
    font-size: 22px;
    font-weight: 800;
    letter-spacing: -0.01em;
  }
  p {
    margin: 0;
    font-size: 14px;
    color: var(--text-2);
  }
  .btns {
    display: flex;
    gap: 10px;
  }
  .big {
    height: 44px;
    padding: 0 18px;
    border-radius: 12px;
    font-size: 14px;
  }
  .fmts {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 6px;
    max-width: 440px;
  }
  .fmts span {
    height: 24px;
    display: inline-flex;
    align-items: center;
    padding: 0 9px;
    border-radius: 6px;
    background: var(--surface-2);
    color: var(--text-2);
    font-size: 11px;
    font-weight: 700;
  }
  ol {
    margin: 16px 0 0;
    padding: 0;
    list-style: none;
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 12px;
  }
  li {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 14px;
    border-radius: 12px;
    background: var(--surface-2);
  }
  .num {
    width: 28px;
    height: 28px;
    flex-shrink: 0;
    border-radius: 50%;
    background: var(--accent-tint);
    color: var(--accent-text-2);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 13px;
    font-weight: 800;
  }
  .st {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .s1 {
    font-size: 13px;
    font-weight: 700;
  }
  .s2 {
    font-size: 12px;
    color: var(--text-3);
  }
</style>
