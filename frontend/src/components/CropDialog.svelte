<script lang="ts" module>
  import type { CropBox } from '../lib/types'

  type Done = (box: CropBox) => void

  /** Manual crop: drag a box over a video frame (or a picture). Fractions of the picture. */
  export const cropper = $state<{
    open: boolean
    title: string
    path: string
    image: string // picture URL when cropping an image instead of a video frame
    duration: number
    rotate: number
    flipH: boolean
    flipV: boolean
    box: CropBox
    cb: Done | null
  }>({ open: false, title: '', path: '', image: '', duration: 0, rotate: 0, flipH: false, flipV: false, box: { x: 0, y: 0, w: 0, h: 0 }, cb: null })

  export function pickCrop(
    o: { title: string; path?: string; image?: string; duration?: number; rotate?: number; flipH?: boolean; flipV?: boolean; box: CropBox },
    cb: Done,
  ) {
    const box = o.box?.w > 0 ? { ...o.box } : { x: 0, y: 0, w: 1, h: 1 }
    Object.assign(cropper, {
      title: o.title,
      path: o.path ?? '',
      image: o.image ?? '',
      duration: o.duration ?? 0,
      rotate: o.rotate ?? 0,
      flipH: !!o.flipH,
      flipV: !!o.flipV,
      box,
      cb,
      open: true,
    })
  }
</script>

<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'

  let dialog: HTMLDialogElement
  let stage = $state<HTMLDivElement | null>(null)
  let natW = $state(16)
  let natH = $state(9)
  let t = $state(0)
  let shownT = $state(0)
  let timer = 0
  let ratio = $state('')
  let wrapW = $state(900)
  const maxH = $derived(Math.round(window.innerHeight * 0.58))
  const stageW = $derived(Math.max(100, Math.min(wrapW - 24, (maxH * natW) / natH)))
  const stageH = $derived((stageW * natH) / natW)

  $effect(() => {
    if (cropper.open && dialog && !dialog.open) {
      t = shownT = Math.min(cropper.duration / 2, 5)
      ratio = ''
      dialog.showModal()
    }
    if (!cropper.open && dialog?.open) dialog.close()
  })

  const q = (s: string) => encodeURIComponent(s)
  const src = $derived(
    cropper.image ||
      (cropper.path
        ? `/kmb/frame?path=${q(cropper.path)}&t=${shownT.toFixed(2)}&w=1280&rot=${cropper.rotate}&fh=${cropper.flipH ? 1 : 0}&fv=${cropper.flipV ? 1 : 0}`
        : ''),
  )
  const b = $derived(cropper.box)
  const pxW = $derived(Math.round(b.w * natW))
  const pxH = $derived(Math.round(b.h * natH))

  const ratios: { value: string; label: string; r: number }[] = [
    { value: '', label: '', r: 0 },
    { value: '9:16', label: '9:16', r: 9 / 16 },
    { value: '1:1', label: '1:1', r: 1 },
    { value: '4:5', label: '4:5', r: 4 / 5 },
    { value: '16:9', label: '16:9', r: 16 / 9 },
    { value: '4:3', label: '4:3', r: 4 / 3 },
  ]
  const lockR = $derived(ratios.find((x) => x.value === ratio)?.r ?? 0)

  function setRatio(v: string) {
    ratio = v
    const r = ratios.find((x) => x.value === v)?.r ?? 0
    if (!r) return
    // The largest centred box of the ratio.
    const pic = natW / natH
    let w = 1
    let h = pic / r
    if (h > 1) {
      h = 1
      w = r / pic
    }
    cropper.box = { x: (1 - w) / 2, y: (1 - h) / 2, w, h }
  }

  const clamp = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, v))

  function drag(mode: string, e: PointerEvent) {
    e.preventDefault()
    e.stopPropagation()
    if (!stage) return
    const el = e.currentTarget as HTMLElement
    el.setPointerCapture(e.pointerId)
    const rect = stage.getBoundingClientRect()
    const start = { ...cropper.box }
    const x0 = e.clientX
    const y0 = e.clientY
    const min = 0.03
    const move = (ev: PointerEvent) => {
      const dx = (ev.clientX - x0) / rect.width
      const dy = (ev.clientY - y0) / rect.height
      let { x, y, w, h } = start
      if (mode === 'move') {
        x = clamp(x + dx, 0, 1 - w)
        y = clamp(y + dy, 0, 1 - h)
      } else {
        if (mode.includes('w')) {
          const nx = clamp(x + dx, 0, x + w - min)
          w += x - nx
          x = nx
        }
        if (mode.includes('e')) w = clamp(w + dx, min, 1 - x)
        if (mode.includes('n')) {
          const ny = clamp(y + dy, 0, y + h - min)
          h += y - ny
          y = ny
        }
        if (mode.includes('s')) h = clamp(h + dy, min, 1 - y)
        if (lockR) {
          // Keep the ratio: follow the dragged width (or height for top/bottom handles).
          const pic = natW / natH
          if (mode === 'n' || mode === 's') w = (h * lockR) / pic
          else h = (w * pic) / lockR
          if (x + w > 1) {
            w = 1 - x
            h = (w * pic) / lockR
          }
          if (y + h > 1) {
            h = 1 - y
            w = (h * lockR) / pic
          }
          if (mode.includes('n')) y = start.y + start.h - h
          if (mode.includes('w')) x = start.x + start.w - w
          x = clamp(x, 0, 1 - w)
          y = clamp(y, 0, 1 - h)
        }
      }
      cropper.box = { x, y, w, h }
    }
    const up = () => {
      el.removeEventListener('pointermove', move)
      el.removeEventListener('pointerup', up)
    }
    el.addEventListener('pointermove', move)
    el.addEventListener('pointerup', up)
  }

  function scrub(v: number) {
    t = v
    clearTimeout(timer)
    timer = window.setTimeout(() => (shownT = t), 150)
  }

  function done(clear = false) {
    const box = cropper.box
    const whole = box.w > 0.995 && box.h > 0.995
    cropper.cb?.(clear || whole ? { x: 0, y: 0, w: 0, h: 0 } : box)
    cropper.open = false
  }
</script>

<dialog bind:this={dialog} onclose={() => (cropper.open = false)}>
  <div class="box">
    <div class="head">
      <Icon name="crop" size={18} />
      <b class="ellipsis">{L('Atur area crop', 'Set the crop area')} · {cropper.title}</b>
      <button class="btn icon" aria-label={L('Tutup', 'Close')} onclick={() => (cropper.open = false)}><Icon name="x" size={16} /></button>
    </div>
    {#if cropper.open}
      <div class="wrap" bind:clientWidth={wrapW}>
        <div class="stage" bind:this={stage} style="width: {stageW}px; height: {stageH}px">
          {#if src}
            <img
              {src}
              alt=""
              draggable="false"
              onload={(e) => {
                const im = e.currentTarget as HTMLImageElement
                natW = im.naturalWidth || 16
                natH = im.naturalHeight || 9
              }}
            />
          {/if}
          <div class="shade" style="clip-path: polygon(0 0, 100% 0, 100% 100%, 0 100%, 0 0, {b.x * 100}% {b.y * 100}%, {b.x * 100}% {(b.y + b.h) * 100}%, {(b.x + b.w) * 100}% {(b.y + b.h) * 100}%, {(b.x + b.w) * 100}% {b.y * 100}%, {b.x * 100}% {b.y * 100}%)"></div>
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div class="rect" style="left: {b.x * 100}%; top: {b.y * 100}%; width: {b.w * 100}%; height: {b.h * 100}%" onpointerdown={(e) => drag('move', e)}>
            <span class="g g1"></span><span class="g g2"></span><span class="g g3"></span><span class="g g4"></span>
            {#each ['nw', 'n', 'ne', 'e', 'se', 's', 'sw', 'w'] as h}
              <button class="h {h}" aria-label={h} onpointerdown={(e) => drag(h, e)}></button>
            {/each}
          </div>
        </div>
      </div>
      {#if cropper.duration > 0}
        <label class="time">
          <span>{L('Frame pada', 'Frame at')} {Math.floor(t / 60)}:{String(Math.floor(t % 60)).padStart(2, '0')}</span>
          <input type="range" min="0" max={cropper.duration} step="0.1" value={t} oninput={(e) => scrub(+(e.currentTarget as HTMLInputElement).value)} />
        </label>
      {/if}
      <div class="foot">
        <div class="chips" role="radiogroup" aria-label={L('Rasio', 'Ratio')}>
          {#each ratios as r}
            <button role="radio" aria-checked={ratio === r.value} class:on={ratio === r.value} onclick={() => setRatio(r.value)}>{r.value ? r.label : L('Bebas', 'Free')}</button>
          {/each}
        </div>
        <span class="size">{pxW} × {pxH} px</span>
        <span class="grow"></span>
        <button class="btn" onclick={() => done(true)}>{L('Tanpa crop', 'No crop')}</button>
        <button class="btn-primary" onclick={() => done()}><Icon name="check" size={16} />{L('Pakai', 'Apply')}</button>
      </div>
    {/if}
  </div>
</dialog>

<style>
  dialog {
    padding: 0;
    border: 1px solid var(--border-strong);
    border-radius: 16px;
    background: var(--surface);
    color: var(--text);
    width: min(1000px, 92vw);
    box-shadow: 0 24px 64px rgba(0, 0, 0, 0.55);
  }
  dialog::backdrop {
    background: rgba(5, 6, 8, 0.7);
  }
  .box {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 16px 18px 18px;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 10px;
    color: var(--accent);
  }
  .head b {
    flex-grow: 1;
    font-size: 15px;
    color: var(--text);
  }
  .wrap {
    display: flex;
    justify-content: center;
    background: var(--inset);
    border-radius: 12px;
    padding: 12px;
  }
  .stage {
    position: relative;
    flex-shrink: 0;
    user-select: none;
    touch-action: none;
  }
  .stage img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: fill;
  }
  .shade {
    position: absolute;
    inset: 0;
    background: rgba(0, 0, 0, 0.55);
    pointer-events: none;
  }
  .rect {
    position: absolute;
    border: 2px solid var(--accent);
    cursor: move;
    box-sizing: border-box;
  }
  .g {
    position: absolute;
    background: rgba(255, 255, 255, 0.35);
    pointer-events: none;
  }
  .g1,
  .g2 {
    top: 0;
    bottom: 0;
    width: 1px;
  }
  .g1 {
    left: 33.33%;
  }
  .g2 {
    left: 66.66%;
  }
  .g3,
  .g4 {
    left: 0;
    right: 0;
    height: 1px;
  }
  .g3 {
    top: 33.33%;
  }
  .g4 {
    top: 66.66%;
  }
  .h {
    position: absolute;
    width: 14px;
    height: 14px;
    padding: 0;
    border: 2px solid var(--surface);
    border-radius: 4px;
    background: var(--accent);
  }
  .nw { left: -8px; top: -8px; cursor: nwse-resize; }
  .n { left: calc(50% - 7px); top: -8px; cursor: ns-resize; }
  .ne { right: -8px; top: -8px; cursor: nesw-resize; }
  .e { right: -8px; top: calc(50% - 7px); cursor: ew-resize; }
  .se { right: -8px; bottom: -8px; cursor: nwse-resize; }
  .s { left: calc(50% - 7px); bottom: -8px; cursor: ns-resize; }
  .sw { left: -8px; bottom: -8px; cursor: nesw-resize; }
  .w { left: -8px; top: calc(50% - 7px); cursor: ew-resize; }
  .time {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 12px;
    color: var(--text-3);
  }
  .time input {
    width: 100%;
    margin: 0;
    accent-color: var(--accent);
  }
  .foot {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .chips {
    display: flex;
    gap: 6px;
  }
  .chips button {
    height: 34px;
    padding: 0 10px;
    border-radius: 9px;
    border: 1px solid var(--border);
    background: var(--surface-2);
    color: var(--text-4);
    font-size: 12px;
    font-weight: 600;
  }
  .chips button.on {
    border-color: var(--accent);
    background: var(--accent-soft);
    color: var(--accent-text);
    font-weight: 700;
  }
  .size {
    white-space: nowrap;
    font-size: 12px;
    color: var(--text-3);
    font-variant-numeric: tabular-nums;
  }
  .grow {
    flex: 1;
  }
  .btn-primary {
    width: auto;
    height: 40px;
    padding: 0 22px;
    flex-shrink: 0;
  }
</style>
