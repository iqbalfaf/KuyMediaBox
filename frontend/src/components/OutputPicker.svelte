<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'
  import { api, errText } from '../lib/api'
  import { defaultDirs, fixedFolder, outputOf, setOutput, shortPath, toast } from '../lib/stores/app.svelte'
  import type { OutputKind, OutputMode } from '../lib/types'

  let { kind, placement = 'up' }: { kind: OutputKind; placement?: 'up' | 'down' } = $props()

  let open = $state(false)
  let root = $state<HTMLDivElement>()

  const cur = $derived(outputOf(kind))
  const isDownload = $derived(kind === 'download')
  const defDir = $derived(defaultDirs[kind] ?? '')

  const title = $derived(
    cur.mode === 'custom'
      ? shortPath(cur.dir)
      : cur.mode === 'subfolder'
        ? L('Folder asal › converted', 'Source folder › converted')
        : cur.mode === 'same'
          ? L('Sama dengan folder file asli', 'Same folder as the source file')
          : shortPath(defDir) || L('Folder default', 'Default folder'),
  )
  const sub = $derived(
    cur.mode === 'custom'
      ? L('Folder pilihan Anda', 'Your chosen folder')
      : cur.mode === 'subfolder'
        ? L('Dinamis · subfolder di samping file asli', 'Dynamic · subfolder next to the source file')
        : cur.mode === 'same'
          ? L('Dinamis · di samping file asli', 'Dynamic · next to the source file')
          : L('Folder default', 'Default folder'),
  )
  const fullPath = $derived(fixedFolder(kind))

  async function choose(mode: OutputMode) {
    open = false
    if (mode === 'custom') {
      try {
        const dir = await api.pickDirectory(L('Pilih folder hasil', 'Choose the output folder'), cur.mode === 'custom' ? cur.dir : defDir)
        if (dir) await setOutput(kind, { mode: 'custom', dir })
      } catch (e) {
        toast(errText(e), 'err')
      }
      return
    }
    await setOutput(kind, { mode, dir: '' })
  }

  function onDoc(e: MouseEvent) {
    if (open && root && !root.contains(e.target as Node)) open = false
  }
  function onKey(e: KeyboardEvent) {
    if (open && e.key === 'Escape') open = false
  }
</script>

<svelte:document onclick={onDoc} onkeydown={onKey} />

<div class="wrap" bind:this={root}>
  <button class="picker" onclick={() => (open = !open)} aria-haspopup="menu" aria-expanded={open} title={fullPath || title}>
    <span class="ic"><Icon name="folder" /></span>
    <span class="text">
      <span class="t ellipsis">{title}</span>
      <span class="s ellipsis">{sub}</span>
    </span>
    <span class="change">{L('Ubah', 'Change')}</span>
  </button>
  {#if open}
    <div class="menu {placement}" role="menu">
      <button role="menuitemradio" aria-checked={cur.mode === 'default'} class:on={cur.mode === 'default'} onclick={() => choose('default')}>
        <span class="mt">{L('Folder default', 'Default folder')}</span>
        <span class="ms" title={defDir}>{defDir || '—'}</span>
      </button>
      {#if !isDownload}
        <button role="menuitemradio" aria-checked={cur.mode === 'subfolder'} class:on={cur.mode === 'subfolder'} onclick={() => choose('subfolder')}>
          <span class="mt">{L('Dinamis: folder asal › converted', 'Dynamic: source folder › converted')}</span>
          <span class="ms">{L('Subfolder baru di samping setiap file asli', 'A new subfolder next to each source file')}</span>
        </button>
        <button role="menuitemradio" aria-checked={cur.mode === 'same'} class:on={cur.mode === 'same'} onclick={() => choose('same')}>
          <span class="mt">{L('Dinamis: sama dengan folder file asli', 'Dynamic: same folder as the source file')}</span>
          <span class="ms">{L('Nama file diberi tambahan agar tidak menimpa', 'File names get a suffix so nothing is overwritten')}</span>
        </button>
      {/if}
      <button role="menuitemradio" aria-checked={cur.mode === 'custom'} class:on={cur.mode === 'custom'} onclick={() => choose('custom')}>
        <span class="mt">{cur.mode === 'custom' ? L('Ganti folder pilihan…', 'Change chosen folder…') : L('Pilih folder sendiri…', 'Choose my own folder…')}</span>
        <span class="ms" title={cur.mode === 'custom' ? cur.dir : ''}>{cur.mode === 'custom' ? cur.dir : L('Semua hasil ke satu folder pilihan Anda', 'All results go to one folder you choose')}</span>
      </button>
    </div>
  {/if}
</div>

<style>
  .wrap {
    position: relative;
    min-width: 0;
  }
  .picker {
    width: 100%;
    height: 48px;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 0 12px;
    border-radius: 10px;
    border: 1px solid var(--border);
    background: var(--surface-2);
    text-align: left;
  }
  .picker:hover {
    border-color: var(--border-strong);
  }
  .ic {
    display: flex;
    color: var(--accent);
  }
  .text {
    flex-grow: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  .t {
    font-size: 13px;
    font-weight: 600;
  }
  .s {
    font-size: 11px;
    color: var(--text-3);
  }
  .change {
    font-size: 12px;
    font-weight: 700;
    color: var(--accent-text-2);
  }
  .menu {
    position: absolute;
    left: 0;
    right: 0;
    z-index: 20;
    padding: 6px;
    border-radius: 12px;
    background: var(--popup);
    border: 1px solid var(--border-strong);
    box-shadow: 0 12px 32px var(--shadow);
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 280px;
  }
  .menu.up {
    bottom: calc(100% + 6px);
  }
  .menu.down {
    top: calc(100% + 6px);
  }
  .menu button {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
    padding: 9px 10px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    text-align: left;
    min-width: 0;
  }
  .menu button:hover {
    background: var(--hover);
  }
  .menu button.on .mt {
    color: var(--accent-text);
  }
  .menu button.on .mt::before {
    content: '● ';
    font-size: 9px;
    vertical-align: 2px;
  }
  .mt {
    font-size: 13px;
    font-weight: 700;
  }
  .ms {
    font-size: 11px;
    color: var(--text-3);
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
