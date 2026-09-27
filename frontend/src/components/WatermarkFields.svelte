<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import Segmented from './Segmented.svelte'
  import Switch from './Switch.svelte'
  import PositionGrid from './PositionGrid.svelte'
  import Icon from './Icon.svelte'
  import { api, errText } from '../lib/api'
  import { toast } from '../lib/stores/app.svelte'
  import type { ImageWatermark } from '../lib/types'

  let { wm = $bindable(), hint }: { wm: ImageWatermark; hint: string } = $props()
  const uid = Math.random().toString(36).slice(2, 8)

  async function pickLogo() {
    try {
      const p = await api.pickFile(L('Pilih gambar logo', 'Choose the logo picture'), L('Gambar', 'Images'), '*.png;*.jpg;*.jpeg;*.webp;*.bmp;*.gif')
      if (p) wm.image = p
    } catch (e) {
      toast(errText(e), 'err')
    }
  }
</script>

<div class="sec">
  <Switch bind:checked={wm.enabled} label="Watermark" {hint} />
  {#if wm.enabled}
    <Segmented
      label={L('Jenis watermark', 'Watermark type')}
      bind:value={wm.type}
      options={[
        { value: 'text', label: L('Teks', 'Text'), icon: 'type' },
        { value: 'image', label: 'Logo', icon: 'image' },
      ]}
    />
    {#if wm.type === 'text'}
      <div class="inline">
        <input class="text-input" aria-label={L('Teks watermark', 'Watermark text')} bind:value={wm.text} placeholder="© Nama" />
        <input class="color" type="color" aria-label={L('Warna teks', 'Text colour')} bind:value={wm.color} />
        <button class="tbtn" class:on={wm.bold} aria-pressed={wm.bold} title={L('Tebal', 'Bold')} onclick={() => (wm.bold = !wm.bold)}><b>B</b></button>
      </div>
    {:else}
      <div class="inline">
        <button class="btn grow" onclick={pickLogo}><Icon name="image" size={16} />{wm.image ? L('Ganti logo', 'Change logo') : L('Pilih logo…', 'Choose logo…')}</button>
      </div>
      {#if wm.image}<span class="hint ellipsis" title={wm.image}>{wm.image.split(/[\\/]/).pop()}</span>{/if}
    {/if}
    <div class="wm">
      <div class="wm-pos">
        <PositionGrid label={L('Posisi', 'Position')} bind:value={wm.position} />
        <button class="mchip" class:on={wm.position === 'tile'} onclick={() => (wm.position = wm.position === 'tile' ? 'br' : 'tile')}>{L('Berulang', 'Tiled')}</button>
      </div>
      <div class="wm-sliders">
        <label class="t12" for="wm-size-{uid}">{wm.type === 'text' ? L('Ukuran huruf', 'Text size') : L('Lebar logo', 'Logo width')} · {wm.size}%</label>
        <input id="wm-size-{uid}" type="range" min="1" max={wm.type === 'text' ? 30 : 100} bind:value={wm.size} />
        <label class="t12" for="wm-op-{uid}">{L('Transparansi', 'Opacity')} · {Math.round(wm.opacity * 100)}%</label>
        <input id="wm-op-{uid}" type="range" min="0.05" max="1" step="0.05" bind:value={wm.opacity} />
        <label class="t12" for="wm-ang-{uid}">{L('Kemiringan', 'Angle')} · {wm.angle}°</label>
        <input id="wm-ang-{uid}" type="range" min="-90" max="90" step="5" bind:value={wm.angle} />
      </div>
    </div>
  {/if}
</div>

<style>
  .sec {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .inline {
    display: flex;
    gap: 8px;
    align-items: center;
  }
  .grow {
    flex: 1;
  }
  .color {
    width: 40px;
    height: 40px;
    padding: 2px;
    border-radius: 10px;
    border: 1px solid var(--border);
    background: var(--surface-2);
    flex-shrink: 0;
  }
  .tbtn {
    width: 36px;
    height: 34px;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 9px;
    border: 1px solid var(--border);
    background: var(--surface-2);
    color: var(--text-2);
  }
  .tbtn.on {
    border-color: var(--accent);
    background: var(--accent-soft);
    color: var(--accent-text);
  }
  .mchip {
    height: 32px;
    padding: 0 8px;
    border-radius: 9px;
    border: 1px solid var(--border);
    background: var(--surface-2);
    color: var(--text-4);
    font-size: 12px;
    font-weight: 600;
    white-space: nowrap;
  }
  .mchip.on {
    border-color: var(--accent);
    background: var(--accent-soft);
    color: var(--accent-text);
    font-weight: 700;
  }
  .wm {
    display: flex;
    gap: 12px;
  }
  .wm-pos {
    display: flex;
    flex-direction: column;
    gap: 6px;
    align-items: stretch;
  }
  .wm-sliders {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }
  .t12 {
    font-size: 12px;
    color: var(--text-3);
  }
  input[type='range'] {
    width: 100%;
    margin: 0;
    accent-color: var(--accent);
  }
</style>
