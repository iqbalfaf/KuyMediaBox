export type Kind = 'image' | 'video' | 'audio' | 'download'
export type Page = 'image' | 'video' | 'audio' | 'download' | 'settings'

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
  error: string
}

export type OutputMode = 'default' | 'subfolder' | 'same' | 'custom'
export type OutputKind = 'image' | 'video' | 'audio' | 'download'

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
}

export interface Capabilities {
  ffmpeg: boolean
  encoders: Record<string, boolean>
}

export interface Link {
  source: 'youtube' | 'spotify' | 'other' | ''
  type: 'video' | 'playlist' | 'channel' | 'track' | 'album' | 'unknown'
  url: string
  id: string
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
  archived: boolean
  unavailable: boolean
}

export interface Collection {
  key: string
  source: 'youtube' | 'spotify' | 'other'
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
  audioFormat: 'mp3' | 'm4a' | 'opus' | 'flac'
  audioQuality: 'auto' | '192' | '320'
  embed: boolean
  skipExisting: boolean
  numbering: boolean
}

export interface JobRef {
  itemId: string
  taskId: string
}
