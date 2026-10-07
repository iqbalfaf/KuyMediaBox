// Settings of the converter pages outside the pages themselves: their defaults, built-in
// presets and how a page's settings become the job the backend runs. Workflows, watched
// folders and the command line use them too.
import { L } from './i18n.svelte'
import { api } from './api'
import { load, saveHooks } from './stores/persist'
import { mergeInto, userPresets, type Preset } from './stores/presets.svelte'
import type { AudioOptions, CollageJob, ImageOptions, SheetOptions, SlideOptions, SubExtractOptions, SubtitleJob, VideoOptions } from './types'

export type VideoMode = 'video' | 'audio' | 'merge' | 'frames' | 'sheet' | 'subs'
type Mode = VideoMode

export const imageDefaults: { mode: 'convert' | 'anim' | 'collage'; o: ImageOptions; anim: SlideOptions; collage: CollageJob } = {
  mode: 'convert',
  anim: { format: 'mp4', seconds: 2, size: 1080, ratio: '', fit: 'blur', background: '#000000', fade: 0.5, music: '' },
  collage: { layout: { cols: 0, width: 2000, gap: 16, cell: '1:1', fit: 'cover', background: '#ffffff', radius: 0 }, format: 'jpg', quality: 90 },
  o: {
    format: 'jpg', quality: 85, resizeMode: 'longest', longest: 1920, percent: 50, width: 1920, height: 1080, background: '#ffffff', autoRotate: true,
    keepMetadata: false, targetKB: 0, icoSizes: [16, 32, 48, 256], rotate: 0, flipH: false, flipV: false, crop: '', cropBox: { x: 0, y: 0, w: 0, h: 0 },
    watermark: { enabled: false, type: 'text', text: '© ', bold: true, color: '#ffffff', image: '', size: 5, opacity: 0.6, angle: 0, position: 'br', margin: 3 },
    aiUpscale: 0, aiModel: 'photo', removeBg: false, bgModel: 'general', bgMask: false, pngCompress: '',
  },
}

export const imageBuiltins = (): Preset[] => [
  { id: 'b-web', name: L('Web ringan (WEBP, maks. 200 KB)', 'Light web (WEBP, max 200 KB)'), value: { format: 'webp', quality: 80, resizeMode: 'longest', longest: 1600, targetKB: 200, keepMetadata: false } },
  { id: 'b-wa', name: L('Foto WhatsApp (JPG 1600 px)', 'WhatsApp photo (JPG 1600 px)'), value: { format: 'jpg', quality: 82, resizeMode: 'longest', longest: 1600, targetKB: 0 } },
  { id: 'b-ig', name: L('Instagram 4:5 (1080 px)', 'Instagram 4:5 (1080 px)'), value: { format: 'jpg', quality: 90, crop: '4:5', resizeMode: 'box', width: 1080, height: 1350, targetKB: 0 } },
  { id: 'b-ico', name: L('Ikon aplikasi (ICO semua ukuran)', 'App icon (ICO, all sizes)'), value: { format: 'ico', icoSizes: [16, 24, 32, 48, 64, 128, 256], crop: '1:1', targetKB: 0 } },
  { id: 'b-product', name: L('Foto produk tanpa latar (PNG)', 'Product photo, no background (PNG)'), value: { format: 'png', removeBg: true, bgMask: false, resizeMode: 'longest', longest: 2000, targetKB: 0, pngCompress: '' } },
  { id: 'b-upscale', name: L('Perbesar foto kecil 4× (AI)', 'Enlarge small photos 4× (AI)'), value: { format: 'jpg', quality: 92, aiUpscale: 4, aiModel: 'photo', resizeMode: 'original', targetKB: 0 } },
  { id: 'b-500', name: L('Dokumen / formulir (≤ 500 KB)', 'Documents / forms (≤ 500 KB)'), value: { format: 'jpg', quality: 90, resizeMode: 'original', targetKB: 500 } },
]

export const videoDefaults: { mode: Mode; v: VideoOptions; a: AudioOptions; frameEvery: number; frameFormat: 'jpg' | 'png'; sheet: SheetOptions; subs: SubExtractOptions } = {
  mode: 'video',
  v: {
    format: 'mp4', codec: 'h264', quality: 'seimbang', manual: false, crf: 23, preset: 'medium', resolution: '720', custom: 540,
    targetMB: 0, bitrateK: 0, hw: '', trimStart: '', trimEnd: '', fps: 'original', fpsCustom: 25, audioMode: 'auto', audioBitrate: 128,
    rotate: 0, flipH: false, flipV: false, subtitles: 'none',
    speed: 1, reverse: false, crop: { x: 0, y: 0, w: 0, h: 0 }, frame: '', frameFit: 'blur', stabilize: false, denoise: 'off',
    music: { file: '', mode: 'mix', volume: 0.5, original: 1, duck: true, loop: true },
    watermark: { enabled: false, type: 'text', text: '© ', bold: true, color: '#ffffff', image: '', size: 5, opacity: 0.6, angle: 0, position: 'br', margin: 3 },
  },
  a: {
    format: 'mp3', bitrate: 192, vbr: false, vbrLevel: 'high', channels: 'source', sampleRate: 'source', keepMetadata: true,
    trimStart: '', trimEnd: '', fadeIn: 0, fadeOut: 0, normalize: false, loudness: -16, removeSilence: false, speed: 1, pitch: 0, denoise: 'off',
  },
  frameEvery: 5,
  frameFormat: 'jpg',
  sheet: { cols: 4, rows: 5, width: 320, format: 'jpg', times: true },
  subs: { format: 'original', langs: '', fonts: false },
}

export const videoBuiltins = (): Preset[] => [
  { id: 'b-wa', name: L('WhatsApp (maks. 16 MB, 720p)', 'WhatsApp (max 16 MB, 720p)'), value: { mode: 'video', v: { format: 'mp4', codec: 'h264', resolution: '720', targetMB: 15, fps: '30', audioMode: 'aac', audioBitrate: 96 } } },
  { id: 'b-discord', name: L('Discord (maks. 10 MB)', 'Discord (max 10 MB)'), value: { mode: 'video', v: { format: 'mp4', codec: 'h264', resolution: '720', targetMB: 9.5, audioMode: 'aac', audioBitrate: 96 } } },
  { id: 'b-email', name: L('Email (maks. 8 MB, 480p)', 'Email (max 8 MB, 480p)'), value: { mode: 'video', v: { format: 'mp4', codec: 'h264', resolution: '480', targetMB: 7.5, audioMode: 'aac', audioBitrate: 64 } } },
  { id: 'b-yt', name: L('Upload YouTube 1080p', 'YouTube upload 1080p'), value: { mode: 'video', v: { format: 'mp4', codec: 'h264', resolution: '1080', targetMB: 0, quality: 'tinggi', manual: false, audioMode: 'aac', audioBitrate: 192 } } },
  { id: 'b-reels', name: L('Reels/TikTok/Shorts (9:16, latar blur)', 'Reels/TikTok/Shorts (9:16, blurred background)'), value: { mode: 'video', v: { format: 'mp4', codec: 'h264', resolution: '1080', frame: '9:16', frameFit: 'blur', targetMB: 0, bitrateK: 0, quality: 'tinggi', audioMode: 'aac', audioBitrate: 160 } } },
  { id: 'b-gif', name: L('GIF pendek (480p, 12 fps)', 'Short GIF (480p, 12 fps)'), value: { mode: 'video', v: { format: 'gif', resolution: '480', fps: '12' } } },
  { id: 'b-mp3', name: L('Ambil lagu (MP3 320)', 'Grab the song (MP3 320)'), value: { mode: 'audio', a: { format: 'mp3', bitrate: 320 } } },
]

export const audioDefaults: { mode: 'convert' | 'merge' | 'cue'; a: AudioOptions } = {
  mode: 'convert',
  a: {
    format: 'mp3', bitrate: 320, vbr: false, vbrLevel: 'high', channels: 'source', sampleRate: 'source', keepMetadata: true,
    trimStart: '', trimEnd: '', fadeIn: 0, fadeOut: 0, normalize: false, loudness: -16, removeSilence: false, speed: 1, pitch: 0, denoise: 'off',
  },
}

export const audioBuiltins = (): Preset[] => [
  { id: 'b-mp3', name: 'MP3 320 kbps', value: { mode: 'convert', a: { format: 'mp3', bitrate: 320, normalize: false, speed: 1, pitch: 0, fadeIn: 0, fadeOut: 0, removeSilence: false } } },
  { id: 'b-pod', name: L('Podcast (MP3 128 mono, suara rata)', 'Podcast (MP3 128 mono, even volume)'), value: { mode: 'convert', a: { format: 'mp3', bitrate: 128, channels: 'mono', normalize: true, loudness: -16, removeSilence: true } } },
  { id: 'b-ring', name: L('Nada dering (M4A 30 dtk + fade)', 'Ringtone (M4A 30 s + fade)'), value: { mode: 'convert', a: { format: 'm4a', bitrate: 192, trimStart: '0:00', trimEnd: '0:30', fadeIn: 1, fadeOut: 3 } } },
  { id: 'b-flac', name: L('Arsip lossless (FLAC)', 'Lossless archive (FLAC)'), value: { mode: 'convert', a: { format: 'flac', normalize: false, speed: 1, pitch: 0 } } },
  { id: 'b-opus', name: L('Hemat kuota (Opus 96)', 'Data saver (Opus 96)'), value: { mode: 'convert', a: { format: 'opus', bitrate: 96 } } },
]

export const subtitleDefaults: { job: SubtitleJob } = {
  job: { model: 'base', language: 'id', translate: false, formats: ['srt'], maxLen: 42, video: 'none' },
}

export const subtitleBuiltins = (): Preset[] => [
  { id: 'b-id', name: L('Video Indonesia → SRT', 'Indonesian video → SRT'), value: { job: { language: 'id', translate: false, formats: ['srt'], maxLen: 42, video: 'none' } } },
  { id: 'b-en', name: L('Terjemahkan ke Inggris (SRT)', 'Translate to English (SRT)'), value: { job: { language: 'auto', translate: true, formats: ['srt'], maxLen: 42 } } },
  { id: 'b-txt', name: L('Transkrip kuliah/rapat (TXT)', 'Lecture/meeting transcript (TXT)'), value: { job: { formats: ['txt'], maxLen: 0, video: 'none' } } },
  { id: 'b-burn', name: L('Video dengan subtitle tertanam', 'Video with burned-in subtitles'), value: { job: { formats: ['srt'], maxLen: 42, video: 'burn' } } },
]

// ---- page settings → backend jobs ----------------------------------------------------------

/** Modules whose settings can run on files outside their page (presets, workflows, CLI). */
export type ModuleKind = 'image' | 'video' | 'audio' | 'subtitle'
export const moduleKinds: ModuleKind[] = ['image', 'video', 'audio', 'subtitle']

const storeKey: Record<ModuleKind, string> = { image: 'kmb.image', video: 'kmb.video', audio: 'kmb.audio', subtitle: 'kmb.subtitle' }
const defaultsOf: Record<ModuleKind, any> = { image: imageDefaults, video: videoDefaults, audio: audioDefaults, subtitle: subtitleDefaults }

export function moduleBuiltins(kind: ModuleKind): Preset[] {
  return { image: imageBuiltins, video: videoBuiltins, audio: audioBuiltins, subtitle: subtitleBuiltins }[kind]()
}

/** The page's saved settings (defaults when the page was never opened). */
export function moduleState(kind: ModuleKind): any {
  return load<any>(storeKey[kind], defaultsOf[kind])
}

/** The page's settings with a preset applied, the way the preset bar applies it. */
export function presetState(kind: ModuleKind, preset: Preset | null): any {
  const st = moduleState(kind)
  if (preset) mergeInto(kind === 'image' ? st.o : st, structuredClone(preset.value))
  return st
}

/** What the backend's Start… call gets for one file at a time (joining modes become per-file). */
export function moduleJob(kind: ModuleKind, st: any): any {
  switch (kind) {
    case 'image':
      return st.o
    case 'video': {
      const mode = st.mode === 'merge' ? 'video' : st.mode
      return { mode, video: st.v, audio: st.a, frameEvery: st.frameEvery, frameFormat: st.frameFormat, sheet: st.sheet, subs: st.subs }
    }
    case 'audio':
      return { mode: 'convert', options: st.a }
    case 'subtitle':
      return st.job
  }
}

/** A short description of a job ("Video → MP4 H264 720p"). */
export function describeJob(kind: string, job: any): string {
  const up = (v: unknown, d: string) => String(v || d).toUpperCase()
  switch (kind) {
    case 'image':
      return `${L('Gambar', 'Image')} → ${up(job?.format, 'jpg')}${job?.removeBg ? ' · ' + L('tanpa latar', 'no background') : ''}${job?.aiUpscale ? ` · AI ${job.aiUpscale}×` : ''}`
    case 'video':
      if (job?.mode === 'audio') return `Video → ${up(job?.audio?.format, 'mp3')}`
      if (job?.mode === 'frames') return L('Video → gambar per frame', 'Video → frame pictures')
      if (job?.mode === 'sheet') return L('Video → lembar kontak', 'Video → contact sheet')
      if (job?.mode === 'subs') return `Video → subtitle (${job?.subs?.format && job.subs.format !== 'original' ? up(job.subs.format, '') : L('asli', 'original')})`
      return `Video → ${up(job?.video?.format, 'mp4')} ${up(job?.video?.codec, '')}${job?.video?.resolution && job.video.resolution !== 'original' ? ' ' + job.video.resolution + 'p' : ''}`.trim()
    case 'audio':
      return `Audio → ${up(job?.options?.format, 'mp3')}${job?.options?.normalize ? ' · ' + L('volume rata', 'even volume') : ''}${job?.options?.denoise && job.options.denoise !== 'off' ? ' · ' + L('kurangi noise', 'denoise') : ''}`
    case 'subtitle':
      return `Subtitle → ${(job?.formats ?? ['srt']).map((f: string) => f.toUpperCase()).join(', ')}${job?.translate ? ' · ' + L('terjemah Inggris', 'to English') : ''}`
    case 'pdf':
      return `PDF · ${job?.tool ?? ''}`
  }
  return kind
}

/** One preset (or the page's current settings) as the backend stores it for the CLI. */
export interface Recipe {
  kind: ModuleKind
  id: string
  name: string
  builtin: boolean
  job: any
}

/** Every module's current settings and presets, resolved into jobs. */
export function recipes(): Recipe[] {
  const out: Recipe[] = []
  // The PDF tools' settings, for "kmb pdf <tool>" (tools read their own part).
  out.push({ kind: 'pdf' as ModuleKind, id: 'current', name: L('Pengaturan Alat PDF', 'PDF tool settings'), builtin: true, job: load<any>('kmb.pdf', {}) })
  for (const kind of moduleKinds) {
    out.push({ kind, id: 'current', name: L('Pengaturan halaman sekarang', 'Current page settings'), builtin: true, job: moduleJob(kind, moduleState(kind)) })
    for (const p of [...moduleBuiltins(kind).map((b) => ({ ...b, builtin: true })), ...userPresets(kind)]) {
      out.push({ kind, id: p.id, name: p.name, builtin: !!p.builtin, job: moduleJob(kind, presetState(kind, p)) })
    }
  }
  return out
}

let syncTimer: ReturnType<typeof setTimeout> | undefined
/** Sends the resolved presets to the backend a moment after settings change. */
export function scheduleRecipeSync(delay = 1500) {
  clearTimeout(syncTimer)
  syncTimer = setTimeout(() => {
    api.syncRecipes(recipes()).catch(() => {})
  }, delay)
}

saveHooks.push((key) => {
  if (/^kmb\.(image|video|audio|subtitle|pdf)$|^kmb\.presets\./.test(key)) scheduleRecipeSync()
})
