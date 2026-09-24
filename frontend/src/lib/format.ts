import { L, locale } from './i18n.svelte'

const nfCache = new Map<string, Intl.NumberFormat>()
function nf(digits: number): Intl.NumberFormat {
  const key = locale() + digits
  let f = nfCache.get(key)
  if (!f) {
    f = new Intl.NumberFormat(locale(), { maximumFractionDigits: digits })
    nfCache.set(key, f)
  }
  return f
}

export function bytes(n: number): string {
  if (!n || n < 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024
    i++
  }
  return `${i === 0 ? nf(0).format(n) : nf(1).format(n)} ${units[i]}`
}

export function duration(sec: number): string {
  if (!sec || sec < 0 || !isFinite(sec)) return ''
  const s = Math.round(sec)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const r = s % 60
  const pad = (v: number) => String(v).padStart(2, '0')
  return h > 0 ? `${h}:${pad(m)}:${pad(r)}` : `${pad(m)}:${pad(r)}`
}

const unit = {
  sec: () => L('detik', 'sec'),
  min: () => L('menit', 'min'),
  hr: () => L('jam', 'hr'),
}

/** "1 jam 3 menit" / "1 hr 3 min" style total length. */
export function longDuration(sec: number): string {
  const m = Math.round(sec / 60)
  if (m < 1) return `${Math.round(sec)} ${unit.sec()}`
  const h = Math.floor(m / 60)
  const r = m % 60
  if (h === 0) return `${m} ${unit.min()}`
  return r ? `${h} ${unit.hr()} ${r} ${unit.min()}` : `${h} ${unit.hr()}`
}

export function eta(sec: number): string {
  if (!isFinite(sec) || sec <= 0) return ''
  if (sec < 60) return `${Math.max(1, Math.round(sec))} ${unit.sec()}`
  const m = Math.round(sec / 60)
  if (m < 60) return `${m} ${unit.min()}`
  return `${Math.floor(m / 60)} ${unit.hr()} ${m % 60} ${unit.min()}`
}

/** YYYYMMDD → "20 Sep 2026" (or "Sep 20, 2026" in English) */
export function ymd(s: string): string {
  if (!/^\d{8}$/.test(s)) return ''
  const d = new Date(Number(s.slice(0, 4)), Number(s.slice(4, 6)) - 1, Number(s.slice(6, 8)))
  return new Intl.DateTimeFormat(locale(), { day: 'numeric', month: 'short', year: 'numeric' }).format(d)
}

export function pct(p: number): string {
  return `${Math.round(Math.max(0, Math.min(1, p)) * 100)}%`
}

export const codecLabel: Record<string, string> = {
  h264: 'H.264', hevc: 'H.265', h265: 'H.265', vp9: 'VP9', vp8: 'VP8', av1: 'AV1', mpeg4: 'MPEG-4',
  mpeg2video: 'MPEG-2', mjpeg: 'MJPEG', prores: 'ProRes', wmv3: 'WMV', gif: 'GIF',
  aac: 'AAC', mp3: 'MP3', opus: 'Opus', vorbis: 'Vorbis', flac: 'FLAC', alac: 'ALAC', ac3: 'AC-3', eac3: 'E-AC-3',
  pcm_s16le: 'PCM', pcm_s24le: 'PCM', pcm_s32le: 'PCM', pcm_f32le: 'PCM', wmav2: 'WMA', amr_nb: 'AMR',
}

export function codec(c: string): string {
  if (c === 'copy') return L('Tanpa encode ulang', 'No re-encode')
  return codecLabel[c] ?? c.toUpperCase()
}

export function fps(v: number): string {
  if (!v) return ''
  return `${nf(0).format(Math.round(v))} fps`
}

export function khz(hz: number): string {
  if (!hz) return ''
  return `${nf(1).format(hz / 1000)} kHz`
}

/** Stable pastel tile colour from a string. */
const tiles = [
  ['#2B3A55', '#B9C8E4'],
  ['#3B2F4A', '#D2C2E6'],
  ['#2B4448', '#BCE0E4'],
  ['#4A3B2B', '#E8D2B8'],
  ['#2F4A3E', '#C0E2D0'],
]
export function tile(seed: string): { bg: string; fg: string } {
  let h = 0
  for (let i = 0; i < seed.length; i++) h = (h * 31 + seed.charCodeAt(i)) | 0
  const [bg, fg] = tiles[Math.abs(h) % tiles.length]
  return { bg, fg }
}

export function initials(s: string): string {
  const words = s.replace(/[^\p{L}\p{N} ]/gu, ' ').trim().split(/\s+/).filter(Boolean)
  if (words.length === 0) return '?'
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase()
  return (words[0][0] + words[1][0]).toUpperCase()
}

/** Parses "1-20", "1–20", "3,5,7-9" into a set of 1-based positions. */
export function parseRange(text: string, max: number): Set<number> | null {
  const out = new Set<number>()
  const parts = text.replace(/[–—]/g, '-').split(/[,;\s]+/).filter(Boolean)
  if (parts.length === 0) return null
  for (const p of parts) {
    const m = p.match(/^(\d+)(?:-(\d+))?$/)
    if (!m) return null
    let a = Number(m[1])
    let b = m[2] ? Number(m[2]) : a
    if (a > b) [a, b] = [b, a]
    for (let i = Math.max(1, a); i <= Math.min(max, b); i++) out.add(i)
  }
  return out
}
