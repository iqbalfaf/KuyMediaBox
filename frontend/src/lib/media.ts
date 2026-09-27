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

const frameRatios: Record<string, [number, number]> = { '9:16': [9, 16], '1:1': [1, 1], '4:5': [4, 5], '16:9': [16, 9], '4:3': [4, 3] }
const evenRound = (v: number) => Math.round(v / 2) * 2
const evenFloor = (v: number) => Math.floor(v / 2) * 2

/** Picture size after rotation and the manual crop (mirrors mediaconv.contentSize). */
export function contentSize(w: number, h: number, o: VideoOptions): [number, number] {
  if (o.rotate === 90 || o.rotate === 270) [w, h] = [h, w]
  if (o.crop?.w > 0 && w && h) return [Math.max(2, evenFloor(w * o.crop.w)), Math.max(2, evenFloor(h * o.crop.h))]
  return [w, h]
}

/** Canvas of a frame choice before scaling (mirrors mediaconv.frameArea). */
function frameArea(w: number, h: number, o: VideoOptions): [number, number] {
  const r = frameRatios[o.frame ?? '']
  if (!r || !w || !h) return [w, h]
  const want = r[0] / r[1]
  if (o.frameFit === 'crop') {
    if (w / h > want) return [Math.max(2, evenFloor(h * want)), h - (h % 2)]
    return [w - (w % 2), Math.max(2, evenFloor(w / want))]
  }
  const short = Math.min(w, h)
  return want >= 1 ? [evenRound(short * want), short - (short % 2)] : [short - (short % 2), evenRound(short / want)]
}

/** Output size after crop, frame and scaling (never upscales), mirrors mediaconv.OutputSize. */
export function outputSize(w: number, h: number, o: VideoOptions): [number, number] {
  let target = targetShort(o)
  if (o.format === 'gif' && target === 0) target = 480
  if (o.codec === 'copy' && o.format !== 'gif') return [w, h]
  ;[w, h] = frameArea(...contentSize(w, h, o), o)
  if (!w || !h || !target || Math.min(w, h) <= target) return [w, h]
  // ffmpeg's "-2" rounds the scaled side to the nearest even number.
  if (w >= h) return [evenRound((w * target) / h), target]
  return [target, evenRound((h * target) / w)]
}

export const lossyAudio: Record<string, boolean> = { mp3: true, m4a: true, ogg: true, opus: true, flac: false, wav: false }
