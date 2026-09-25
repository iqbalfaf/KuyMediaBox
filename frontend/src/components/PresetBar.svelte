<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'
  import { toast } from '../lib/stores/app.svelte'
  import { deletePreset, mergeInto, savePreset, userPresets, type Preset } from '../lib/stores/presets.svelte'
  import { askText } from '../lib/stores/prompt.svelte'

  let {
    module,
    target,
    builtins = [],
    onapply,
  }: {
    /** Storage key of the module: image, video, audio, download.youtube, … */
    module: string
    /** The live settings object presets are read from and applied to. */
    target: any
    builtins?: Preset[]
    onapply?: () => void
  } = $props()

  let chosen = $state('')
  let select = $state<HTMLSelectElement>()
  const list = $derived([...builtins.map((b) => ({ ...b, builtin: true })), ...userPresets(module)])
  const current = $derived(list.find((p) => p.id === chosen))

  function apply(id: string) {
    chosen = id
    const p = list.find((x) => x.id === id)
    if (!p) return
    mergeInto(target, $state.snapshot(p.value))
    onapply?.()
    toast(L(`Preset "${p.name}" dipakai`, `Preset "${p.name}" applied`), 'ok')
  }

  async function saveAs() {
    const name = await askText(
      L('Simpan preset', 'Save preset'),
      L('Nama preset (pengaturan sekarang akan disimpan)', 'Preset name (the current settings are saved)'),
      current && !current.builtin ? current.name : '',
      L('mis. WA Video 16MB', 'e.g. WhatsApp video 16MB'),
    )
    if (!name) return
    const p = savePreset(module, name, $state.snapshot(target))
    chosen = p.id
    toast(L(`Preset "${name}" disimpan`, `Preset "${name}" saved`), 'ok')
  }

  function remove() {
    if (!current || current.builtin) return
    const name = current.name
    deletePreset(module, current.id)
    chosen = ''
    if (select) select.value = ''
    toast(L(`Preset "${name}" dihapus`, `Preset "${name}" deleted`), 'info')
  }
</script>

<div class="presets">
  <span class="ic"><Icon name="bookmark" size={16} /></span>
  <div class="wrap">
    <select bind:this={select} aria-label={L('Preset', 'Preset')} value={chosen} onchange={(e) => apply((e.currentTarget as HTMLSelectElement).value)}>
      <option value="" disabled>{L('Pilih preset…', 'Choose a preset…')}</option>
      {#if builtins.length}
        <optgroup label={L('Bawaan', 'Built-in')}>
          {#each list.filter((p) => p.builtin) as p (p.id)}<option value={p.id}>{p.name}</option>{/each}
        </optgroup>
      {/if}
      {#if list.some((p) => !p.builtin)}
        <optgroup label={L('Preset saya', 'My presets')}>
          {#each list.filter((p) => !p.builtin) as p (p.id)}<option value={p.id}>{p.name}</option>{/each}
        </optgroup>
      {/if}
    </select>
  </div>
  <button class="mini" title={L('Simpan pengaturan sekarang sebagai preset', 'Save the current settings as a preset')} aria-label={L('Simpan preset', 'Save preset')} onclick={saveAs}><Icon name="plus" size={16} /></button>
  <button class="mini" title={L('Hapus preset ini', 'Delete this preset')} aria-label={L('Hapus preset', 'Delete preset')} disabled={!current || current.builtin} onclick={remove}><Icon name="trash" size={15} /></button>
</div>

<style>
  .presets {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 4px 4px 4px 10px;
    border-radius: 12px;
    background: var(--inset);
    border: 1px solid var(--border);
    flex-shrink: 0;
  }
  .ic {
    display: flex;
    color: var(--accent-text-2);
    margin-right: 4px;
  }
  .wrap {
    flex-grow: 1;
    min-width: 0;
  }
  select {
    width: 100%;
    height: 32px;
    border: 0;
    background: transparent;
    color: var(--text);
    font-size: 13px;
    font-weight: 600;
    outline: none;
  }
  option,
  optgroup {
    background: var(--popup);
    color: var(--text);
  }
  .mini {
    width: 32px;
    height: 32px;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--text-2);
  }
  .mini:hover:not(:disabled) {
    background: var(--hover);
    color: var(--text);
  }
  .mini:disabled {
    opacity: 0.35;
  }
</style>
