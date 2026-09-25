<script lang="ts">
  import { L } from '../../lib/i18n.svelte'
  import Icon from '../../components/Icon.svelte'
  import Segmented from '../../components/Segmented.svelte'

  let { open = $bindable(false), onsave }: { open: boolean; onsave: (dataUrl: string, w: number, h: number) => void } = $props()

  let dialog: HTMLDialogElement
  let mode = $state<'draw' | 'type' | 'upload'>('draw')
  let color = $state('#1a1a1a')
  let canvas = $state<HTMLCanvasElement>()
  let drawn = $state(false)
  let name = $state('')
  let font = $state('Segoe Script')
  let upload = $state('')
  const fonts = ['Segoe Script', 'Lucida Handwriting', 'Ink Free', 'Brush Script MT', 'Segoe Print']
  const colors = ['#1a1a1a', '#1d3fb5', '#b01c1c']

  $effect(() => {
    if (open && dialog && !dialog.open) {
      drawn = false
      upload = ''
      dialog.showModal()
      setTimeout(clear, 20)
    }
    if (!open && dialog?.open) dialog.close()
  })

  // ---- drawing ----
  let last: { x: number; y: number } | null = null
  function pos(e: PointerEvent) {
    const r = canvas!.getBoundingClientRect()
    return { x: ((e.clientX - r.left) * canvas!.width) / r.width, y: ((e.clientY - r.top) * canvas!.height) / r.height }
  }
  function down(e: PointerEvent) {
    canvas!.setPointerCapture(e.pointerId)
    last = pos(e)
  }
  function moveDraw(e: PointerEvent) {
    if (!last) return
    const p = pos(e)
    const ctx = canvas!.getContext('2d')!
    ctx.strokeStyle = color
    ctx.lineWidth = 5
    ctx.lineCap = 'round'
    ctx.lineJoin = 'round'
    ctx.beginPath()
    ctx.moveTo(last.x, last.y)
    ctx.lineTo(p.x, p.y)
    ctx.stroke()
    last = p
    drawn = true
  }
  function clear() {
    if (!canvas) return
    canvas.getContext('2d')!.clearRect(0, 0, canvas.width, canvas.height)
    drawn = false
  }

  /** Crops transparent borders so the signature is placed tightly. */
  function trimmed(src: HTMLCanvasElement): HTMLCanvasElement {
    const ctx = src.getContext('2d')!
    const { width, height } = src
    const data = ctx.getImageData(0, 0, width, height).data
    let x0 = width, y0 = height, x1 = -1, y1 = -1
    for (let y = 0; y < height; y++) {
      for (let x = 0; x < width; x++) {
        if (data[(y * width + x) * 4 + 3] > 10) {
          if (x < x0) x0 = x
          if (x > x1) x1 = x
          if (y < y0) y0 = y
          if (y > y1) y1 = y
        }
      }
    }
    if (x1 < 0) return src
    const pad = 6
    x0 = Math.max(0, x0 - pad)
    y0 = Math.max(0, y0 - pad)
    x1 = Math.min(width - 1, x1 + pad)
    y1 = Math.min(height - 1, y1 + pad)
    const out = document.createElement('canvas')
    out.width = x1 - x0 + 1
    out.height = y1 - y0 + 1
    out.getContext('2d')!.drawImage(src, x0, y0, out.width, out.height, 0, 0, out.width, out.height)
    return out
  }

  function typed(): HTMLCanvasElement {
    const c = document.createElement('canvas')
    const size = 96
    const ctx = c.getContext('2d')!
    ctx.font = `${size}px "${font}"`
    const w = Math.ceil(ctx.measureText(name).width) + 40
    c.width = w
    c.height = size * 1.6
    const ctx2 = c.getContext('2d')!
    ctx2.font = `${size}px "${font}"`
    ctx2.fillStyle = color
    ctx2.textBaseline = 'middle'
    ctx2.fillText(name, 20, c.height / 2)
    return trimmed(c)
  }

  function onFile(e: Event) {
    const f = (e.currentTarget as HTMLInputElement).files?.[0]
    if (!f) return
    const r = new FileReader()
    r.onload = () => (upload = String(r.result))
    r.readAsDataURL(f)
  }

  const ready = $derived(mode === 'draw' ? drawn : mode === 'type' ? name.trim() !== '' : upload !== '')

  function save() {
    if (mode === 'upload') {
      const img = new Image()
      img.onload = () => {
        onsave(upload, img.naturalWidth, img.naturalHeight)
        open = false
      }
      img.src = upload
      return
    }
    const c = mode === 'draw' ? trimmed(canvas!) : typed()
    onsave(c.toDataURL('image/png'), c.width, c.height)
    open = false
  }
</script>

<dialog bind:this={dialog} onclose={() => (open = false)}>
  <div class="box">
    <div class="head">
      <b>{L('Buat tanda tangan', 'Create a signature')}</b>
      <button class="btn icon" aria-label={L('Tutup', 'Close')} onclick={() => (open = false)}><Icon name="x" size={16} /></button>
    </div>
    <Segmented
      label={L('Cara membuat', 'How to create')}
      bind:value={mode}
      options={[
        { value: 'draw', label: L('Gambar', 'Draw'), icon: 'pencil' },
        { value: 'type', label: L('Ketik', 'Type'), icon: 'type' },
        { value: 'upload', label: L('Unggah gambar', 'Upload image'), icon: 'image' },
      ]}
    />
    {#if mode !== 'upload'}
      <div class="colors">
        {#each colors as c}
          <button class="sw" class:on={color === c} style="background: {c}" aria-label={c} onclick={() => (color = c)}></button>
        {/each}
      </div>
    {/if}
    {#if mode === 'draw'}
      <div class="pad">
        <canvas bind:this={canvas} width="1200" height="400" onpointerdown={down} onpointermove={moveDraw} onpointerup={() => (last = null)} onpointercancel={() => (last = null)}></canvas>
        <span class="line"></span>
        <button class="btn small clear" onclick={clear}><Icon name="undo" size={14} />{L('Hapus', 'Clear')}</button>
      </div>
      <p class="hint">{L('Tanda tangan dengan mouse, touchpad, atau pena di area putih.', 'Sign with the mouse, touchpad or a pen in the white area.')}</p>
    {:else if mode === 'type'}
      <input class="text-input" bind:value={name} placeholder={L('Ketik nama Anda', 'Type your name')} />
      <div class="fonts">
        {#each fonts as f}
          <button class="font" class:on={font === f} style="font-family: '{f}'; color: {color}" onclick={() => (font = f)}>{name || L('Tanda Tangan', 'Signature')}</button>
        {/each}
      </div>
    {:else}
      <label class="upload">
        {#if upload}<img src={upload} alt="" />{:else}<Icon name="upload" size={28} /><span>{L('Pilih gambar tanda tangan (PNG transparan paling bagus)', 'Choose a signature picture (transparent PNG works best)')}</span>{/if}
        <input type="file" accept="image/png,image/jpeg" onchange={onFile} />
      </label>
    {/if}
    <div class="foot">
      <button class="btn" onclick={() => (open = false)}>{L('Batal', 'Cancel')}</button>
      <button class="btn-accent" disabled={!ready} onclick={save}><Icon name="check" size={16} stroke={2.5} />{L('Simpan & pakai', 'Save & use')}</button>
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
    width: min(680px, 92vw);
    box-shadow: 0 24px 64px rgba(0, 0, 0, 0.55);
  }
  dialog::backdrop {
    background: rgba(5, 6, 8, 0.65);
  }
  .box {
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 16px 20px 18px;
  }
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .colors {
    display: flex;
    gap: 8px;
  }
  .sw {
    width: 26px;
    height: 26px;
    border-radius: 50%;
    border: 2px solid transparent;
    padding: 0;
  }
  .sw.on {
    border-color: var(--accent);
    box-shadow: inset 0 0 0 2px var(--surface);
  }
  .pad {
    position: relative;
    border-radius: 12px;
    overflow: hidden;
    background: #fff;
  }
  canvas {
    display: block;
    width: 100%;
    aspect-ratio: 3 / 1;
    touch-action: none;
    cursor: crosshair;
  }
  .line {
    position: absolute;
    left: 8%;
    right: 8%;
    bottom: 26%;
    border-bottom: 1px dashed #c8c8c8;
    pointer-events: none;
  }
  .clear {
    position: absolute;
    top: 8px;
    right: 8px;
    height: 30px;
    font-size: 12px;
  }
  .fonts {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
    gap: 8px;
  }
  .font {
    height: 64px;
    border-radius: 10px;
    border: 2px solid transparent;
    background: #fff;
    font-size: 26px;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    padding: 0 12px;
  }
  .font.on {
    border-color: var(--accent);
  }
  .upload {
    position: relative;
    min-height: 160px;
    border-radius: 12px;
    border: 2px dashed var(--border-strong);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 10px;
    color: var(--text-3);
    font-size: 13px;
    cursor: pointer;
    background: var(--canvas);
    text-align: center;
    padding: 12px;
  }
  .upload img {
    max-height: 150px;
    max-width: 100%;
    background: #fff;
  }
  .upload input {
    position: absolute;
    inset: 0;
    opacity: 0;
    cursor: pointer;
  }
  .foot {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
</style>
