<script lang="ts" module>
  import { parseTime } from './TrimFields.svelte'

  type Done = (start: string, end: string) => void

  /** Visual trim: a player (or frame previews) with a timeline and start/end handles. */
  export const trim = $state<{
    open: boolean
    title: string
    path: string
    duration: number
    hasVideo: boolean
    thumb: string
    start: number
    end: number
    cb: Done | null
  }>({ open: false, title: '', path: '', duration: 0, hasVideo: false, thumb: '', start: 0, end: 0, cb: null })

  /** Opens the dialog. path "" = a remote video (timeline only, with thumb as picture). */
  export function pickTrim(o: { title: string; path?: string; duration: number; hasVideo?: boolean; thumb?: string; start: string; end: string }, cb: Done) {
    const s = parseTime(o.start) ?? 0
    const e = parseTime(o.end) ?? 0
    Object.assign(trim, {
      title: o.title,
      path: o.path ?? '',
      duration: o.duration,
      hasVideo: o.hasVideo ?? true,
      thumb: o.thumb ?? '',
      start: Math.min(s, o.duration),
      end: e > 0 && e <= o.duration ? e : o.duration,
      cb,
      open: true,
    })
  }

  /** Seconds → "1:23.4" (tenths only when needed), the format the trim fields accept. */
  export function fmtTime(sec: number): string {
    sec = Math.max(0, Math.round(sec * 10) / 10)
    const h = Math.floor(sec / 3600)
    const m = Math.floor((sec % 3600) / 60)
    const s = sec % 60
    const ss = (s < 10 ? '0' : '') + (Number.isInteger(s) ? String(s) : s.toFixed(1))
    return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`
  }
</script>

<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'

  let dialog: HTMLDialogElement
  let media = $state<HTMLMediaElement | null>(null)
  let track = $state<HTMLDivElement | null>(null)
  let pos = $state(0)
  let playing = $state(false)
  let playable = $state(true)
  let framePos = $state(0) // position of the frame picture (updated while scrubbing)
  let frameTimer = 0
  let startText = $state('')
  let endText = $state('')

  $effect(() => {
    if (trim.open && dialog && !dialog.open) {
      pos = trim.start
      framePos = trim.start
      playing = false
      playable = !!trim.path
      startText = fmtTime(trim.start)
      endText = fmtTime(trim.end)
      dialog.showModal()
    }
    if (!trim.open && dialog?.open) {
      media?.pause()
      dialog.close()
    }
  })

  const dur = $derived(Math.max(trim.duration, 0.1))
  const pct = (t: number) => `${(Math.max(0, Math.min(t, dur)) / dur) * 100}%`
  const q = (s: string) => encodeURIComponent(s)
  const mediaUrl = $derived(trim.path ? `/kmb/media?path=${q(trim.path)}` : '')
  const frameUrl = $derived(trim.path ? `/kmb/frame?path=${q(trim.path)}&t=${framePos.toFixed(2)}&w=960` : trim.thumb)
  const waveUrl = $derived(trim.path && !trim.hasVideo ? `/kmb/wave?path=${q(trim.path)}&w=1400&h=120&c=ff7a45` : '')

  function seek(t: number) {
    pos = Math.max(0, Math.min(t, dur))
    if (media && playable) media.currentTime = pos
    clearTimeout(frameTimer)
    frameTimer = window.setTimeout(() => (framePos = pos), 120)
  }

  function timeAt(clientX: number): number {
    if (!track) return 0
    const r = track.getBoundingClientRect()
    return ((clientX - r.left) / r.width) * dur
  }

  function drag(kind: 'start' | 'end' | 'head', e: PointerEvent) {
    e.preventDefault()
    e.stopPropagation()
    const el = e.currentTarget as HTMLElement
    el.setPointerCapture(e.pointerId)
    const move = (ev: PointerEvent) => {
      const t = Math.max(0, Math.min(timeAt(ev.clientX), dur))
      if (kind === 'start') setStart(Math.min(t, trim.end - 0.1))
      else if (kind === 'end') setEnd(Math.max(t, trim.start + 0.1))
      seek(kind === 'start' ? trim.start : kind === 'end' ? trim.end : t)
    }
    const up = () => {
      el.removeEventListener('pointermove', move)
      el.removeEventListener('pointerup', up)
    }
    el.addEventListener('pointermove', move)
    el.addEventListener('pointerup', up)
    move(e)
  }

  function setStart(t: number) {
    trim.start = Math.max(0, Math.round(t * 10) / 10)
    startText = fmtTime(trim.start)
  }
  function setEnd(t: number) {
    trim.end = Math.min(dur, Math.round(t * 10) / 10)
    endText = fmtTime(trim.end)
  }

  function typed(which: 'start' | 'end', text: string) {
    const v = parseTime(text)
    if (v === null) return
    if (which === 'start' && v < trim.end) {
      trim.start = v
      seek(v)
    }
    if (which === 'end' && v > trim.start && v <= dur + 0.05) {
      trim.end = Math.min(v, dur)
      seek(trim.end)
    }
  }

  async function toggle() {
    if (!media || !playable) return
    if (playing) {
      media.pause()
      return
    }
    if (pos < trim.start || pos >= trim.end - 0.05) seek(trim.start)
    try {
      await media.play()
    } catch {
      playable = false
    }
  }

  function tick() {
    if (!media) return
    pos = media.currentTime
    if (playing && pos >= trim.end) {
      media.pause()
      pos = trim.end
    }
  }

  function done() {
    const whole = trim.start <= 0.05 && trim.end >= dur - 0.05
    trim.cb?.(whole || trim.start <= 0.05 ? '' : fmtTime(trim.start), whole || trim.end >= dur - 0.05 ? '' : fmtTime(trim.end))
    trim.open = false
  }

  function key(e: KeyboardEvent) {
    const tag = (e.target as HTMLElement)?.tagName
    if (tag === 'INPUT') return
    if (e.key === ' ') {
      e.preventDefault()
      toggle()
    } else if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
      e.preventDefault()
      seek(pos + (e.key === 'ArrowLeft' ? -1 : 1) * (e.shiftKey ? 0.1 : 1))
    } else if (e.key === '[') setStart(Math.min(pos, trim.end - 0.1))
    else if (e.key === ']') setEnd(Math.max(pos, trim.start + 0.1))
  }
</script>

<dialog bind:this={dialog} onclose={() => (trim.open = false)} onkeydown={key}>
  <div class="box">
    <div class="head">
      <Icon name="scissors" size={18} />
      <b class="ellipsis">{L('Potong', 'Trim')} · {trim.title}</b>
      <button class="btn icon" aria-label={L('Tutup', 'Close')} onclick={() => (trim.open = false)}><Icon name="x" size={16} /></button>
    </div>

    {#if trim.open}
      <div class="stage" class:audio={trim.path && !trim.hasVideo}>
        {#if trim.path && trim.hasVideo && playable}
          <!-- svelte-ignore a11y_media_has_caption -->
          <video
            bind:this={media}
            src={mediaUrl}
            preload="auto"
            onloadedmetadata={() => media && (media.currentTime = pos)}
            ontimeupdate={tick}
            onplay={() => (playing = true)}
            onpause={() => (playing = false)}
            onerror={() => (playable = false)}
          ></video>
        {:else if trim.path && !trim.hasVideo}
          <div class="audio-face"><Icon name="music" size={42} /><span>{fmtTime(pos)}</span></div>
          {#if playable}
            <audio
              bind:this={media}
              src={mediaUrl}
              preload="auto"
              onloadedmetadata={() => media && (media.currentTime = pos)}
              ontimeupdate={tick}
              onplay={() => (playing = true)}
              onpause={() => (playing = false)}
              onerror={() => (playable = false)}
            ></audio>
          {/if}
        {:else if frameUrl}
          <img src={frameUrl} alt="" />
        {:else}
          <div class="audio-face"><Icon name="film" size={42} /></div>
        {/if}
      </div>

      <div class="controls">
        <button class="btn icon play" disabled={!playable || !trim.path} aria-label={playing ? L('Jeda', 'Pause') : L('Putar', 'Play')} onclick={toggle}>
          <Icon name={playing ? 'pause' : 'play'} size={16} />
        </button>
        <span class="now">{fmtTime(pos)} / {fmtTime(dur)}</span>
        <span class="grow"></span>
        <button class="btn" onclick={() => setStart(Math.min(pos, trim.end - 0.1))} title="[">{L('Mulai di sini', 'Start here')}</button>
        <button class="btn" onclick={() => setEnd(Math.max(pos, trim.start + 0.1))} title="]">{L('Selesai di sini', 'End here')}</button>
      </div>

      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="track" bind:this={track} onpointerdown={(e) => drag('head', e)} style={waveUrl ? `background-image: url(${waveUrl})` : ''}>
        <div class="dim" style="left: 0; width: {pct(trim.start)}"></div>
        <div class="dim" style="left: {pct(trim.end)}; right: 0"></div>
        <div class="sel" style="left: {pct(trim.start)}; width: calc({pct(trim.end)} - {pct(trim.start)})"></div>
        <div class="head-line" style="left: {pct(pos)}"></div>
        <button class="handle" style="left: {pct(trim.start)}" aria-label={L('Titik mulai', 'Start point')} onpointerdown={(e) => drag('start', e)}></button>
        <button class="handle end" style="left: {pct(trim.end)}" aria-label={L('Titik selesai', 'End point')} onpointerdown={(e) => drag('end', e)}></button>
      </div>

      <div class="foot">
        <label>
          <span>{L('Mulai', 'Start')}</span>
          <input class="text-input" bind:value={startText} oninput={() => typed('start', startText)} onblur={() => (startText = fmtTime(trim.start))} />
        </label>
        <label>
          <span>{L('Selesai', 'End')}</span>
          <input class="text-input" bind:value={endText} oninput={() => typed('end', endText)} onblur={() => (endText = fmtTime(trim.end))} />
        </label>
        <span class="len">{L('Durasi hasil', 'Result length')}: <b>{fmtTime(trim.end - trim.start)}</b></span>
        <span class="grow"></span>
        <button class="btn" onclick={() => { setStart(0); setEnd(dur) }}>{L('Semua', 'Whole')}</button>
        <button class="btn-primary" onclick={done}><Icon name="check" size={16} />{L('Pakai', 'Apply')}</button>
      </div>
      <p class="hint">
        {L('Tarik penanda di timeline, atau putar lalu klik "Mulai/Selesai di sini". Tombol: Spasi = putar, ←/→ = geser 1 dtk (Shift = 0,1 dtk), [ dan ] = set mulai/selesai.', 'Drag the markers on the timeline, or play and click "Start/End here". Keys: Space = play, ←/→ = move 1 s (Shift = 0.1 s), [ and ] = set start/end.')}
        {#if trim.path && !playable && trim.hasVideo}{L(' Format ini tidak bisa diputar di pratinjau, jadi yang tampil gambar dari posisi yang dipilih.', ' This format can\'t be played in the preview, so a picture of the chosen position is shown.')}{/if}
      </p>
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
    width: min(980px, 92vw);
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
  .stage {
    height: min(52vh, 480px);
    border-radius: 12px;
    overflow: hidden;
    background: #000;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .stage.audio {
    height: 150px;
    background: var(--inset);
  }
  video,
  img {
    width: 100%;
    height: 100%;
    object-fit: contain;
  }
  .audio-face {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    color: var(--text-3);
    font-size: 20px;
    font-weight: 800;
    font-variant-numeric: tabular-nums;
  }
  .controls,
  .foot {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .foot {
    align-items: flex-end;
  }
  .grow {
    flex: 1;
  }
  .now,
  .len {
    font-size: 13px;
    color: var(--text-2);
    font-variant-numeric: tabular-nums;
  }
  .len {
    padding-bottom: 12px;
  }
  .track {
    position: relative;
    margin: 0 14px;
    height: 64px;
    border-radius: 10px;
    background: var(--inset);
    background-size: 100% 100%;
    border: 1px solid var(--border);
    cursor: pointer;
    touch-action: none;
  }
  .dim {
    position: absolute;
    top: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.45);
    pointer-events: none;
  }
  .sel {
    position: absolute;
    top: 0;
    bottom: 0;
    border-top: 3px solid var(--accent);
    border-bottom: 3px solid var(--accent);
    background: color-mix(in srgb, var(--accent) 12%, transparent);
    pointer-events: none;
  }
  .head-line {
    position: absolute;
    top: -4px;
    bottom: -4px;
    width: 2px;
    margin-left: -1px;
    background: var(--text);
    pointer-events: none;
  }
  .handle {
    position: absolute;
    top: -2px;
    bottom: -2px;
    width: 14px;
    margin-left: -14px;
    padding: 0;
    border: 0;
    border-radius: 6px 0 0 6px;
    background: var(--accent);
    cursor: ew-resize;
  }
  .handle.end {
    margin-left: 0;
    border-radius: 0 6px 6px 0;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 4px;
    width: 110px;
  }
  label span {
    font-size: 11px;
    color: var(--text-3);
  }
  .btn-primary {
    width: auto;
    height: 40px;
    padding: 0 22px;
    flex-shrink: 0;
  }
  .hint {
    margin: 0;
  }
</style>
