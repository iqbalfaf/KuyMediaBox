import { L } from './i18n.svelte'
import type { VideoOptions } from './types'

/** Codecs each container accepts, first = default (mirrors internal/mediaconv). */
export const videoFormats: Record<string, string[]> = {
  mp4: ['h264', 'h265', 'av1', 'copy'],
  mkv: ['h264', 'h265', 'vp9', 'av1', 'copy'],
  webm: ['vp9', 'av1', 'copy'],
  mov: ['h264', 'h265', 'copy'],
  avi: ['h264', 'copy'],
  gif: [],
}

export function codecOptionLabel(c: string): string {
  switch (c) {
    case 'h264':
      return L('H.264 — paling kompatibel', 'H.264 — most compatible')
    case 'h265':
      return L('H.265 — lebih kecil', 'H.265 — smaller')
    case 'vp9':
      return L('VP9 — untuk web', 'VP9 — for the web')
    case 'av1':
      return L('AV1 — paling kecil, lambat', 'AV1 — smallest, slow')
    case 'copy':
      return L('Salin tanpa encode ulang (tercepat)', 'Copy without re-encoding (fastest)')
  }
  return c
}

export const crfTable: Record<string, [number, number, number]> = {
  h264: [28, 23, 18],
  h265: [30, 26, 22],
  vp9: [38, 32, 26],
  av1: [40, 34, 27],
}

export function crfFor(o: VideoOptions): number {
  if (o.manual && o.crf > 0) return o.crf
  const t = crfTable[o.codec] ?? crfTable.h264
  return o.quality === 'hemat' ? t[0] : o.quality === 'tinggi' ? t[2] : t[1]
}

export function maxCrf(codec: string): number {
  return codec === 'vp9' || codec === 'av1' ? 63 : 51
}

export function targetShort(o: VideoOptions): number {
  switch (o.resolution) {
    case '2160':
      return 2160
    case '1440':
      return 1440
    case '1080':
      return 1080
    case '720':
      return 720
    case '480':
      return 480
    case 'custom':
      return o.custom >= 16 ? o.custom - (o.custom % 2) : 0
  }
  return o.format === 'gif' ? 480 : 0
}

/** Output size after scaling (never upscales), mirrors mediaconv.OutputSize. */
export function outputSize(w: number, h: number, o: VideoOptions): [number, number] {
  let target = targetShort(o)
  if (o.format === 'gif' && target === 0) target = 480
  if (o.codec === 'copy' && o.format !== 'gif') return [w, h]
  if (o.rotate === 90 || o.rotate === 270) [w, h] = [h, w]
  if (!w || !h || !target || Math.min(w, h) <= target) return [w, h]
  // ffmpeg's "-2" rounds the scaled side to the nearest even number.
  if (w >= h) return [Math.round((w * target) / h / 2) * 2, target]
  return [target, Math.round((h * target) / w / 2) * 2]
}

export const lossyAudio: Record<string, boolean> = { mp3: true, m4a: true, ogg: true, opus: true, flac: false, wav: false }
