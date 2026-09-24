export type Kind = 'image' | 'video' | 'audio' | 'download' | 'pdf'
export type Page = 'image' | 'video' | 'audio' | 'download' | 'pdf' | 'settings'

export type TaskStatus = 'queued' | 'running' | 'done' | 'failed' | 'canceled' | 'skipped'

export interface TaskInfo {
  id: string
  kind: Kind
  title: string
  status: TaskStatus
  progress: number
  message: string
  detail: string
  output: string
  outSize: number
  started: number
  finished: number
  seq: number
}

export interface FileItem {
  id: string
  path: string
  name: string
  ext: string
  size: number
  width: number
  height: number
  duration: number
  format: string
  videoCodec: string
  fps: number
  audioCodec: string
  sampleRate: number
  bitsPerSample: number
  channels: number
  hasVideo: boolean
  hasAudio: boolean
  hasCover: boolean
  pages: number
  encrypted: boolean
  locked: boolean
  error: string
}

export type Lang = "id" | "en"

export type OutputMode = 'default' | 'subfolder' | 'same' | 'custom'
export type OutputKind = 'image' | 'video' | 'audio' | 'download' | 'pdf'

export interface OutputSpec {
  mode: OutputMode
  dir: string
}

export interface ImageOptions {
  format: string
  quality: number
  resizeMode: 'original' | 'longest' | 'percent' | 'box'
  longest: number
  percent: number
  width: number
  height: number
  background: string
  autoRotate: boolean
}

export interface VideoOptions {
  format: string
  codec: string
  quality: 'hemat' | 'seimbang' | 'tinggi'
  manual: boolean
  crf: number
  preset: 'fast' | 'medium' | 'slow'
  resolution: 'original' | '1080' | '720' | '480' | 'custom'
  custom: number
}

export interface AudioOptions {
  format: string
  bitrate: number
  channels: 'source' | 'stereo' | 'mono'
  sampleRate: 'source' | '44100' | '48000'
  keepMetadata: boolean
}

export interface Settings {
  outputs: Record<OutputKind, OutputSpec>
  downloadSubfolders: boolean
  suffix: string
  conflict: 'rename' | 'skip' | 'overwrite'
  notify: boolean
  skipDownloaded: boolean
  autoUpdate: boolean
  language: Lang
  toolPaths: Record<string, string>
}

export interface UpdateInfo {
  current: string
  latest: string
  available: boolean
  notes: string
  url: string
  publishedAt: string
  mode: 'portable' | 'installer'
  assetName: string
  assetSize: number
}

export interface ToolStatus {
  id: string
  name: string
  description: string
  found: boolean
  path: string
  version: string
  source: string
  runtime: string
  latest: string
  updateAvailable: boolean
  busy: boolean
  progress: number
  error: string
  required: string
  optional: boolean
}

export interface Capabilities {
  ffmpeg: boolean
  encoders: Record<string, boolean>
}

export type Source = 'youtube' | 'spotify' | 'tiktok' | 'instagram' | 'facebook' | 'other'

export interface Link {
  source: Source | ''
  type: 'video' | 'playlist' | 'channel' | 'track' | 'album' | 'post' | 'profile' | 'unknown'
  url: string
  id: string
  photo: boolean
  short: boolean
}

export interface Entry {
  id: string
  url: string
  title: string
  artist: string
  album: string
  duration: number
  date: string
  index: number
  thumbnail: string
  tab: string
  kind: '' | 'video' | 'audio' | 'image'
  archived: boolean
  unavailable: boolean
}

export interface Collection {
  key: string
  source: Source
  type: string
  url: string
  title: string
  subtitle: string
  thumbnail: string
  entries: Entry[]
  tabCounts: Record<string, number>
}

export interface DownloadOptions {
  mode: 'video' | 'audio'
  quality: 'best' | '1080' | '720' | '480'
  container: 'mp4' | 'mkv'
  audioFormat: 'mp3' | 'm4a' | 'opus' | 'flac' | 'wav'
  audioQuality: 'auto' | '96' | '128' | '160' | '192' | '256' | '320'
  embed: boolean
  skipExisting: boolean
  numbering: boolean
  imageFormat: 'original' | 'jpg'
}

export interface JobRef {
  itemId: string
  taskId: string
}

// ---- PDF tools ------------------------------------------------------------------------------

export interface PageSize {
  w: number
  h: number
}

export interface DocInfo {
  path: string
  name: string
  pages: PageSize[]
  encrypted: boolean
}

export interface PdfRect {
  page: number
  x: number
  y: number
  w: number
  h: number
}

export interface PdfChange {
  kind: 'added' | 'removed'
  text: string
  page: number
  boxes: PdfRect[]
}

export interface CompareResult {
  pagesA: number
  pagesB: number
  changes: PdfChange[]
  added: number
  removed: number
  same: boolean
}

export interface OcrLanguage {
  tag: string
  name: string
}

export interface PdfEnv {
  office: { word: boolean; excel: boolean; powerpoint: boolean; libreoffice: string }
  browser: string
}

export interface PdfJob {
  id: string
  path: string
  password: string
}

export interface PdfOptions {
  compress: { level: 'extreme' | 'recommended' | 'low'; gray: boolean }
  rotate: number
  protect: { password: string; ownerPassword: string; allowPrint: boolean; allowCopy: boolean; allowEdit: boolean; aes128: boolean }
  watermark: {
    type: 'text' | 'image'
    text: string
    size: number
    bold: boolean
    color: string
    image: string
    scale: number
    opacity: number
    angle: number
    position: string
    pages: string
    under: boolean
  }
  numbers: { position: string; margin: number; start: number; pages: string; format: string; size: number; color: string; bold: boolean; mirror: boolean }
  export: { mode: 'pages' | 'extract'; format: 'jpg' | 'png'; dpi: number; quality: number; pages: string }
  ocr: { lang: string; pages: string; skipText: boolean }
  crop: { mode: 'margins' | 'box'; top: number; right: number; bottom: number; left: number; box: PdfRect; pages: string }
  split: { mode: 'ranges' | 'every' | 'all'; ranges: string; every: number }
  pages: string
  separate: boolean
  html: { pageSize: string; orientation: string; margin: string; width: number; onePage: boolean; background: boolean }
  images: { pageSize: string; orientation: string; margin: string; quality: number; combine: boolean }
}

/** One page of an organised document. src -1 = blank page. */
export interface PageRef {
  src: number
  page: number
  rotate: number
  w: number
  h: number
}

/** An object drawn on a page (points, display space, origin top-left). */
export interface EditItem {
  kind: 'text' | 'rect' | 'ellipse' | 'line' | 'ink' | 'image'
  page: number
  x: number
  y: number
  w: number
  h: number
  x2: number
  y2: number
  points: [number, number][]
  text: string
  size: number
  bold: boolean
  align: string
  color: string
  fill: string
  stroke: number
  opacity: number
  angle: number
  imageUrl: string
}
