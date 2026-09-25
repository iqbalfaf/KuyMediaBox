// Thin typed wrapper around the generated Wails bindings.
import { L } from './i18n.svelte'
import * as App from '../../wailsjs/go/main/App'
import * as RT from '../../wailsjs/runtime/runtime'
import type {
  AfterQueue, AudioOptions, AudioTags, Capabilities, CertInfo, Collection, CompareResult, DocInfo, DownloadOptions, EditItem, Entry, FileItem,
  HistoryEntry, ImageOptions, JobRef, Link, OcrLanguage, OpenRequest, PageRef, PdfEnv, PdfJob, PdfOptions, PdfRect, Settings, TaskInfo,
  ToolStatus, UpdateInfo, VideoOptions,
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
  startVideo: (
    items: { id: string; path: string }[],
    job: { mode: string; video: VideoOptions; audio: AudioOptions; frameEvery: number; frameFormat: string },
  ): Promise<JobRef[]> => call.StartVideo(items, job),
  startAudio: (items: { id: string; path: string; tags?: AudioTags | null }[], job: { mode: string; options: AudioOptions }): Promise<JobRef[]> =>
    call.StartAudio(items, job),
  getHWEncoders: (): Promise<Record<string, string[]>> => call.GetHWEncoders(),
  pickFile: (title: string, filterName: string, pattern: string): Promise<string> => call.PickFile(title, filterName, pattern),
  openFileDefault: (path: string): Promise<void> => call.OpenFileDefault(path),
  pathExists: (paths: string[]): Promise<Record<string, boolean>> => call.PathExists(paths),

  getHistory: (): Promise<HistoryEntry[]> => call.GetHistory(),
  removeHistory: (ids: string[]): Promise<void> => call.RemoveHistory(ids),
  clearHistory: (): Promise<void> => call.ClearHistory(),

  getAfterQueue: (): Promise<AfterQueue> => call.GetAfterQueue(),
  setAfterQueue: (action: string): Promise<AfterQueue> => call.SetAfterQueue(action),
  cancelAfterQueue: (): Promise<AfterQueue> => call.CancelAfterQueue(),

  getSendTo: (): Promise<boolean> => call.GetSendTo(),
  setSendTo: (on: boolean): Promise<boolean> => call.SetSendTo(on),
  takeLaunchFiles: (): Promise<OpenRequest | null> => call.TakeLaunchFiles(),
  routePaths: (paths: string[]): Promise<OpenRequest> => call.RoutePaths(paths),
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
  setEntrySource: (key: string, id: string, url: string): Promise<Entry> => call.SetEntrySource(key, id, url),

  pdfWarmup: (): Promise<void> => call.PdfWarmup(),
  pdfDoc: (path: string, password = ''): Promise<DocInfo> => call.PdfDoc(path, password),
  pdfFind: (path: string, query: string, matchCase: boolean): Promise<PdfRect[]> => call.PdfFind(path, query, matchCase),
  pdfCompare: (a: string, b: string): Promise<CompareResult> => call.PdfCompare(a, b),
  pdfOcrLanguages: (): Promise<OcrLanguage[]> => call.PdfOcrLanguages(),
  pdfEnvironment: (): Promise<PdfEnv> => call.PdfEnvironment(),
  pdfScan: (): Promise<FileItem | null> => call.PdfScan(),
  pdfSaveCapture: (dataUrl: string): Promise<FileItem | null> => call.PdfSaveCapture(dataUrl),
  startPdf: (tool: string, items: PdfJob[], o: PdfOptions, secret = ''): Promise<JobRef[]> => call.StartPdf(tool, items, o, secret),
  certInfo: (path: string, password: string): Promise<CertInfo> => call.CertInfo(path, password),
  createCertificate: (name: string, email: string, org: string, years: number, password: string): Promise<string> =>
    call.CreateCertificate(name, email, org, years, password),
  startPdfCombine: (tool: string, items: PdfJob[], o: PdfOptions): Promise<JobRef> => call.StartPdfCombine(tool, items, o),
  startPdfEdit: (req: {
    tool: string
    sources: { path: string; password: string }[]
    pages?: PageRef[]
    items?: EditItem[]
    redact?: { boxes: PdfRect[]; dpi: number; color: string }
    crop?: PdfOptions['crop']
  }): Promise<JobRef> => call.StartPdfEdit(req),
}

/** URL of a rendered page preview (0-based page, width in pixels). */
export function pageUrl(path: string, page: number, width: number, bust = ''): string {
  return `/kmb/page?path=${encodeURIComponent(path)}&i=${page}&w=${Math.round(width)}${bust ? '&v=' + bust : ''}`
}

/** URL of an image file preview. */
export function imageUrl(path: string, width: number): string {
  return `/kmb/img?path=${encodeURIComponent(path)}&w=${Math.round(width)}`
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
