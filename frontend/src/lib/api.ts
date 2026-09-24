// Thin typed wrapper around the generated Wails bindings.
import { L } from './i18n.svelte'
import * as App from '../../wailsjs/go/main/App'
import * as RT from '../../wailsjs/runtime/runtime'
import type {
  AudioOptions, Capabilities, Collection, DownloadOptions, FileItem, ImageOptions, JobRef, Link,
  Settings, TaskInfo, ToolStatus, UpdateInfo, VideoOptions,
} from './types'

const call = App as any

export const api = {
  getSettings: (): Promise<Settings> => call.GetSettings(),
  saveSettings: (s: Settings): Promise<Settings> => call.SaveSettings(s),
  pickDirectory: (title: string, start = ''): Promise<string> => call.PickDirectory(title, start),
  openFolder: (dir: string): Promise<void> => call.OpenFolder(dir),
  revealFile: (path: string): Promise<void> => call.RevealFile(path),

  getTools: (): Promise<ToolStatus[]> => call.GetTools(),
  recheckTools: (): Promise<ToolStatus[]> => call.RecheckTools(),
  installTool: (id: string): Promise<void> => call.InstallTool(id),
  pickToolPath: (id: string): Promise<void> => call.PickToolPath(id),
  resetToolPath: (id: string): Promise<void> => call.ResetToolPath(id),
  getCapabilities: (): Promise<Capabilities> => call.GetCapabilities(),

  addPaths: (kind: string, paths: string[]): Promise<FileItem[]> => call.AddPaths(kind, paths),
  pickFiles: (kind: string): Promise<FileItem[]> => call.PickFiles(kind),
  pickFolder: (kind: string): Promise<FileItem[]> => call.PickFolder(kind),
  startImage: (items: { id: string; path: string }[], o: ImageOptions): Promise<JobRef[]> => call.StartImage(items, o),
  startVideo: (items: { id: string; path: string }[], job: { mode: string; video: VideoOptions; audio: AudioOptions }): Promise<JobRef[]> =>
    call.StartVideo(items, job),
  startAudio: (items: { id: string; path: string }[], o: AudioOptions): Promise<JobRef[]> => call.StartAudio(items, o),
  getDefaultDirs: (): Promise<Record<string, string>> => call.GetDefaultDirs(),
  outputFolder: (kind: string): Promise<string> => call.OutputFolder(kind),
  getVersion: (): Promise<string> => call.GetVersion(),
  checkUpdate: (): Promise<UpdateInfo> => call.CheckUpdate(),
  installUpdate: (): Promise<void> => call.InstallUpdate(),

  listTasks: (): Promise<TaskInfo[]> => call.ListTasks(),
  cancelTask: (id: string): Promise<void> => call.CancelTask(id),
  cancelKind: (kind: string): Promise<void> => call.CancelKind(kind),
  forgetTasks: (ids: string[]): Promise<void> => call.ForgetTasks(ids),

  detectLinks: (text: string): Promise<Link[]> => call.DetectLinks(text),
  analyzeLink: (url: string): Promise<Collection> => call.AnalyzeLink(url),
  forgetCollection: (key: string): Promise<void> => call.ForgetCollection(key),
  collectionDir: (key: string): Promise<string> => call.CollectionDir(key),
  startDownloads: (key: string, ids: string[], o: DownloadOptions): Promise<JobRef[]> => call.StartDownloads(key, ids, o),
}

export const runtime = {
  on: (name: string, cb: (...data: any[]) => void) => RT.EventsOn(name, cb),
  minimise: () => RT.WindowMinimise(),
  toggleMaximise: () => RT.WindowToggleMaximise(),
  quit: () => RT.Quit(),
  clipboardText: () => RT.ClipboardGetText(),
  clipboardSet: (t: string) => (RT as any).ClipboardSetText(t) as Promise<boolean>,
  onFileDrop: (cb: (x: number, y: number, paths: string[]) => void) => RT.OnFileDrop(cb, true),
  openURL: (url: string) => RT.BrowserOpenURL(url),
}

/** Turns a rejected Wails promise into readable text. */
export function errText(e: unknown): string {
  if (typeof e === 'string') return e
  if (e instanceof Error) return e.message
  try {
    return JSON.stringify(e)
  } catch {
    return L('Terjadi kesalahan', 'Something went wrong')
  }
}
