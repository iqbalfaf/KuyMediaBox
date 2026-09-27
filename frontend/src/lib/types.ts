export type Kind = 'image' | 'video' | 'audio' | 'download' | 'pdf' | 'subtitle'
export type Page = 'image' | 'video' | 'audio' | 'subtitle' | 'flows' | 'download' | 'pdf' | 'history' | 'settings'

export type TaskStatus = 'queued' | 'running' | 'done' | 'failed' | 'canceled' | 'skipped'

export interface TaskInfo {
  id: string
  kind: Kind
  title: string
  status: TaskStatus
  progress: number
  message: string
  detail: string
  input: string
  inSize: number
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
  subCodec: string
  subFile: string
  tags: Record<string, string> | null
  cue: string
  cueTracks: number
  error: string
}

export type Lang = "id" | "en"

export type OutputMode = 'default' | 'subfolder' | 'same' | 'custom'
export type OutputKind = 'image' | 'video' | 'audio' | 'download' | 'pdf' | 'subtitle'

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
  keepMetadata: boolean
  targetKB: number
  icoSizes: number[]
  rotate: number
  flipH: boolean
  flipV: boolean
  crop: string
  cropBox: CropBox
  watermark: ImageWatermark
  aiUpscale: 0 | 2 | 3 | 4
  aiModel: 'photo' | 'anime'
  removeBg: boolean
  bgModel: 'general' | 'people' | 'fast'
  bgMask: boolean
  pngCompress: '' | 'lossless' | 'small'
}

export interface SlideOptions {
  format: 'gif' | 'webp' | 'mp4'
  seconds: number
  size: number
  ratio: '' | '1:1' | '16:9' | '9:16' | '4:5'
  fit: 'contain' | 'cover' | 'blur'
  background: string
  fade: number
  music: string
}

export interface CollageJob {
  layout: { cols: number; width: number; gap: number; cell: string; fit: 'cover' | 'contain'; background: string; radius: number }
  format: 'jpg' | 'png' | 'webp'
  quality: number
}

export interface SubscriptionInfo {
  id: string
  url: string
  title: string
  source: string
  type: string
  thumbnail: string
  mode: 'video' | 'audio'
  tabs: string[]
  everyHours: number
  enabled: boolean
  lastCheck: number
  nextCheck: number
  lastNew: number
  totalNew: number
  lastError: string
  checking: boolean
}

export interface PendingSummary {
  batches: number
  items: number
  titles: string[]
}

export interface ModelStatus {
  kind: 'whisper' | 'bgremove'
  id: string
  sizeMB: number
  installed: boolean
  busy: boolean
  progress: number
  error: string
}

export interface SubtitleJob {
  model: string
  language: string
  translate: boolean
  formats: string[]
  maxLen: number
  video: 'none' | 'embed' | 'burn'
}

export interface ImageWatermark {
  enabled: boolean
  type: 'text' | 'image'
  text: string
  bold: boolean
  color: string
  image: string
  size: number
  opacity: number
  angle: number
  position: string
  margin: number
}

export interface VideoOptions {
  format: string
  codec: string
  quality: 'hemat' | 'seimbang' | 'tinggi'
  manual: boolean
  crf: number
  preset: 'fast' | 'medium' | 'slow'
  resolution: 'original' | '2160' | '1440' | '1080' | '720' | '480' | 'custom'
  custom: number
  targetMB: number
  bitrateK: number
  hw: '' | 'nvenc' | 'qsv' | 'amf'
  trimStart: string
  trimEnd: string
  fps: string
  fpsCustom: number
  audioMode: 'auto' | 'copy' | 'aac' | 'mp3' | 'opus' | 'mute'
  audioBitrate: number
  rotate: number
  flipH: boolean
  flipV: boolean
  subtitles: 'none' | 'embed' | 'burn'
  speed: number
  reverse: boolean
  crop: CropBox
  frame: '' | '9:16' | '1:1' | '4:5' | '16:9' | '4:3'
  frameFit: 'crop' | 'blur' | 'pad'
  stabilize: boolean
  denoise: Denoise
  music: VideoMusic
  watermark: ImageWatermark
}

export interface SheetOptions {
  cols: number
  rows: number
  width: number
  format: 'jpg' | 'png'
  times: boolean
}

export type Denoise = 'off' | 'light' | 'medium' | 'strong'

/** Manual crop in fractions (0..1) of the rotated frame; w = 0 means none. */
export interface CropBox {
  x: number
  y: number
  w: number
  h: number
}

export interface VideoMusic {
  file: string
  mode: 'mix' | 'replace'
  volume: number
  original: number
  duck: boolean
  loop: boolean
}

export interface AudioOptions {
  format: string
  bitrate: number
  vbr: boolean
  vbrLevel: 'best' | 'high' | 'medium' | 'small'
  channels: 'source' | 'stereo' | 'mono'
  sampleRate: 'source' | '44100' | '48000'
  keepMetadata: boolean
  trimStart: string
  trimEnd: string
  fadeIn: number
  fadeOut: number
  normalize: boolean
  loudness: number
  removeSilence: boolean
  speed: number
  pitch: number
  denoise: Denoise
}

export interface AudioTags {
  title: string
  artist: string
  album: string
  albumArtist: string
  year: string
  genre: string
  track: string
  cover: string
  removeCover: boolean
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
  theme: 'dark' | 'light' | 'system'
  parallel: Record<string, number>
  cookiesBrowser: string
  cookiesFile: string
  nameTemplate: string
  spotifyTemplate: string
  spotifyLogin: boolean
  clipboardWatch: boolean
  tray: boolean
  downloadLimitKB: number
  watch: WatchRule[]
  workflows: Workflow[]
  toolPaths: Record<string, string>
}

export interface WatchRule {
  id: string
  dir: string
  kind: 'image' | 'video' | 'audio' | 'flow'
  options: any
  enabled: boolean
}

export interface HistoryEntry {
  id: string
  time: number
  started: number
  kind: Kind
  title: string
  input: string
  output: string
  inSize: number
  outSize: number
  status: TaskStatus
  message: string
  detail: string
}

export interface AfterQueue {
  action: 'none' | 'sleep' | 'shutdown'
  pending: boolean
  seconds: number
}

export interface OpenRequest {
  page: '' | 'image' | 'video' | 'audio' | 'pdf' | 'subtitle'
  paths: string[]
}

export interface CertInfo {
  name: string
  email: string
  issuer: string
  selfSigned: boolean
  notBefore: string
  notAfter: string
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

export type Source = 'youtube' | 'spotify' | 'tiktok' | 'instagram' | 'facebook' | 'x' | 'pinterest' | 'soundcloud' | 'twitch' | 'reddit' | 'bilibili' | 'other'

export interface Link {
  source: Source | ''
  type: 'video' | 'playlist' | 'channel' | 'track' | 'album' | 'artist' | 'post' | 'profile' | 'board' | 'search' | 'story' | 'unknown'
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
  source: string
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
  quality: 'best' | '2160' | '1440' | '1080' | '720' | '480'
  container: 'mp4' | 'mkv'
  audioFormat: 'mp3' | 'm4a' | 'opus' | 'flac' | 'wav'
  audioQuality: 'auto' | '96' | '128' | '160' | '192' | '256' | '320'
  embed: boolean
  skipExisting: boolean
  numbering: boolean
  imageFormat: 'original' | 'jpg'
  subtitles: 'none' | 'file' | 'embed'
  subLangs: string
  sectionStart: string
  sectionEnd: string
  sponsorBlock: 'off' | 'mark' | 'remove'
  playlist: boolean
  lyrics: boolean
  /** Workflow run on every downloaded file ('' = none). */
  workflow: string
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
  digisign: { certFile: string; name: string; reason: string; location: string; contact: string; visible: boolean; position: string; page: string; tsa: string }
  pdfaCheck: { flavour: string }
  headerFooter: {
    topLeft: string
    topCenter: string
    topRight: string
    bottomLeft: string
    bottomCenter: string
    bottomRight: string
    size: number
    color: string
    bold: boolean
    margin: number
    line: boolean
    pages: string
    skipFirst: boolean
    mirror: boolean
  }
  nup: { mode: 'nup' | 'booklet'; n: number; paper: string; border: boolean; margin: number }
  semicolon: boolean
}

export interface PdfMeta {
  title: string
  author: string
  subject: string
  keywords: string
  creator: string
}

export interface PdfBookmark {
  title: string
  page: number
  kids: PdfBookmark[]
}

export interface PdfDetails {
  meta: PdfMeta
  bookmarks: PdfBookmark[]
  pages: number
}

export interface PdfFormField {
  id: string
  name: string
  kind: 'text' | 'date' | 'check' | 'radio' | 'combo' | 'list'
  value: string
  values: string[] | null
  checked: boolean
  options: string[] | null
  multiline: boolean
  multi: boolean
  locked: boolean
  page: number
  format: string
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

/** One step of a workflow: a module with the settings it had when the step was saved. */
export interface FlowStep {
  kind: 'image' | 'video' | 'audio' | 'subtitle' | 'pdf'
  /** PDF tool id. */
  tool: string
  /** Preset id the step was made from ('current' = the page settings). */
  preset: string
  label: string
  job: any
}

export interface Workflow {
  id: string
  name: string
  steps: FlowStep[]
}

/** A file that went through a workflow. */
export interface FlowResult {
  workflow: string
  input: string
  output: string
  error: string
}
