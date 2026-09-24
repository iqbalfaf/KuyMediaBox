import { L } from '../i18n.svelte'
import { api, errText } from '../api'
import { toolById } from '../pdfTools'
import type { DocInfo, EditItem, FileItem, OcrLanguage, PageRef, PdfEnv, PdfJob, PdfOptions, PdfRect } from '../types'
import { toast } from './app.svelte'
import { Converter } from './converter.svelte'
import { load, save } from './persist'
import { askPassword } from './prompt.svelte'
import { trackBatch } from './tasks.svelte'

/** Which PDF tool is open (null = the tool overview). */
export const pdfNav = $state<{ tool: string | null; query: string }>({ tool: null, query: '' })

const converters = new Map<string, Converter>()

/** The file list of a tool (kept while the app runs, like the other pages). */
export function convFor(toolId: string): Converter {
  let c = converters.get(toolId)
  if (!c) {
    c = new Converter('pdf', toolById(toolId)?.input ?? 'pdf')
    converters.set(toolId, c)
  }
  return c
}

export const defaultOptions: PdfOptions = {
  compress: { level: 'recommended', gray: false },
  rotate: 90,
  protect: { password: '', ownerPassword: '', allowPrint: true, allowCopy: false, allowEdit: false, aes128: false },
  watermark: { type: 'text', text: 'RAHASIA', size: 48, bold: true, color: '#d03030', image: '', scale: 35, opacity: 0.3, angle: 45, position: 'mc', pages: '', under: false },
  numbers: { position: 'bc', margin: 28, start: 1, pages: '', format: '{n}', size: 11, color: '#000000', bold: false, mirror: false },
  export: { mode: 'pages', format: 'jpg', dpi: 150, quality: 90, pages: '' },
  ocr: { lang: '', pages: '', skipText: true },
  crop: { mode: 'margins', top: 10, right: 10, bottom: 10, left: 10, box: { page: 1, x: 0.1, y: 0.1, w: 0.8, h: 0.8 }, pages: '' },
  split: { mode: 'ranges', ranges: '', every: 1 },
  pages: '',
  separate: false,
  html: { pageSize: 'a4', orientation: 'portrait', margin: 'normal', width: 1280, onePage: false, background: true },
  images: { pageSize: 'a4', orientation: 'auto', margin: 'small', quality: 90, combine: true },
}

function mergeDefaults(saved: PdfOptions): PdfOptions {
  const out = structuredClone(defaultOptions) as any
  for (const k of Object.keys(out)) {
    const v = (saved as any)[k]
    if (v && typeof v === 'object' && !Array.isArray(v)) out[k] = { ...out[k], ...v }
    else if (v !== undefined) out[k] = v
  }
  return out
}

/** Tool options, remembered between sessions (passwords are never stored). */
export const pdfOpts = $state<PdfOptions>(mergeDefaults(load('kmb.pdf', defaultOptions)))

export function persistOptions() {
  const o = $state.snapshot(pdfOpts) as PdfOptions
  o.protect = { ...o.protect, password: '', ownerPassword: '' }
  save('kmb.pdf', o)
}

export const pdfEnv = $state<{ value: PdfEnv | null; langs: OcrLanguage[]; langsLoaded: boolean }>({ value: null, langs: [], langsLoaded: false })

let warmed = false

/** Starts the PDF engine and checks helper programs the first time the PDF page opens. */
export async function initPdf(force = false) {
  if (!warmed) {
    warmed = true
    api.pdfWarmup().catch(() => {})
  }
  if (!pdfEnv.value || force) {
    try {
      pdfEnv.value = await api.pdfEnvironment()
    } catch {
      /* shown as missing */
    }
  }
}

export async function loadOcrLanguages() {
  if (pdfEnv.langsLoaded) return
  try {
    pdfEnv.langs = (await api.pdfOcrLanguages()) ?? []
  } catch (e) {
    toast(errText(e), 'err')
  } finally {
    pdfEnv.langsLoaded = true
  }
}

export function jobs(conv: Converter, items: FileItem[]): PdfJob[] {
  return items.map((it) => ({ id: it.id, path: it.path, password: conv.passwords[it.id] ?? '' }))
}

/** Runs a per-file tool for the given items. */
export function startBatch(toolId: string, conv: Converter, items: FileItem[]) {
  persistOptions()
  return conv.start(items, () => api.startPdf(toolId, jobs(conv, items), $state.snapshot(pdfOpts) as PdfOptions))
}

/** A single task that is not tied to a list row (combine and page editors). */
export const single = $state<Record<string, string>>({})

export async function startCombine(toolId: string, conv: Converter, items: FileItem[]) {
  persistOptions()
  conv.starting = true
  try {
    const ref = await api.startPdfCombine(toolId, jobs(conv, items), $state.snapshot(pdfOpts) as PdfOptions)
    single[toolId] = ref.taskId
    trackBatch('pdf', [ref.taskId])
  } catch (e) {
    toast(errText(e), 'err')
  } finally {
    conv.starting = false
  }
}

export async function startEdit(req: {
  tool: string
  sources: { path: string; password: string }[]
  pages?: PageRef[]
  items?: EditItem[]
  redact?: { boxes: PdfRect[]; dpi: number; color: string }
  crop?: PdfOptions['crop']
}): Promise<boolean> {
  try {
    const ref = await api.startPdfEdit(req)
    single[req.tool] = ref.taskId
    trackBatch('pdf', [ref.taskId])
    return true
  } catch (e) {
    toast(errText(e), 'err')
    return false
  }
}

/** Opens a single document for the page tools, asking for its password when needed. */
export const docState = $state<Record<string, { path: string; password: string } | null>>({})

export function lockedHint(): string {
  return L('PDF ini dikunci password', 'This PDF is password protected')
}

/** Where files dropped on the window go while a PDF tool is open. */
export const pdfDrop: { fn: ((paths: string[]) => void) | null } = { fn: null }

/** Form state that is never saved (password confirmation). */
export const pdfForm = $state({ confirmPw: '' })

/** Makes sure every locked PDF in items has a (correct) password; returns the items to run. */
export async function ensurePasswords(conv: Converter, items: FileItem[]): Promise<FileItem[]> {
  const ok: FileItem[] = []
  for (const it of items) {
    if (!it.locked || conv.passwords[it.id]) {
      ok.push(it)
      continue
    }
    let wrong = false
    for (;;) {
      const pw = await askPassword(it.name, wrong)
      if (pw === null) break
      try {
        await api.pdfDoc(it.path, pw)
        conv.passwords[it.id] = pw
        ok.push(it)
        break
      } catch {
        wrong = true
      }
    }
  }
  return ok
}

/** An open document of a page tool. */
export interface OpenDoc {
  path: string
  name: string
  password: string
  info: DocInfo
}

/** Opens a PDF for the page tools, asking for the password when it is locked. */
export async function openDoc(path: string): Promise<OpenDoc | null> {
  const name = path.split(/[\/]/).pop() ?? path
  try {
    const info = await api.pdfDoc(path, '')
    return { path, name, password: '', info }
  } catch (e) {
    if (errText(e) !== 'password') {
      toast(`${name}: ${errText(e)}`, 'err')
      return null
    }
  }
  let wrong = false
  for (;;) {
    const pw = await askPassword(name, wrong)
    if (pw === null) return null
    try {
      const info = await api.pdfDoc(path, pw)
      return { path, name, password: pw, info }
    } catch {
      wrong = true
    }
  }
}

/** Lets the user choose one PDF and opens it. */
export async function pickDoc(): Promise<OpenDoc | null> {
  try {
    const list = await api.pickFiles('pdf')
    const it = list.find((x) => !x.error)
    if (!it) {
      if (list.length) toast(list[0].error, 'err')
      return null
    }
    return openDoc(it.path)
  } catch (e) {
    toast(errText(e), 'err')
    return null
  }
}

/** Formats 1-based page numbers as compact ranges ("1-3, 7"). */
export function rangesText(pages: number[]): string {
  const s = [...new Set(pages)].sort((a, b) => a - b)
  const out: string[] = []
  for (let i = 0; i < s.length; i++) {
    let j = i
    while (j + 1 < s.length && s[j + 1] === s[j] + 1) j++
    out.push(j > i ? `${s[i]}-${s[j]}` : `${s[i]}`)
    i = j
  }
  return out.join(', ')
}

/** A page card in the page tools. src -1 = blank page. */
export interface Card {
  key: string
  src: number
  page: number // 1-based
  rotate: number
  w: number
  h: number
}

/** Page-tool workspaces, kept while the app runs. */
export const pageTools = $state<Record<string, { sources: OpenDoc[]; cards: Card[] }>>({})

let cardSeq = 0
export function cardsOf(doc: OpenDoc, src: number): Card[] {
  return doc.info.pages.map((p, i) => ({ key: `c${++cardSeq}`, src, page: i + 1, rotate: 0, w: p.w, h: p.h }))
}
export function newKey(): string {
  return `c${++cardSeq}`
}

/** Runs a per-file tool on one open document as a single task. */
export async function startSingle(tool: string, doc: OpenDoc): Promise<boolean> {
  persistOptions()
  try {
    const refs = await api.startPdf(tool, [{ id: 'doc', path: doc.path, password: doc.password }], $state.snapshot(pdfOpts) as PdfOptions)
    single[tool] = refs[0].taskId
    trackBatch('pdf', [refs[0].taskId])
    return true
  } catch (e) {
    toast(errText(e), 'err')
    return false
  }
}

/** A saved signature (this session only). */
export interface Signature {
  url: string
  w: number
  h: number
}

/** Page-editor workspaces (edit, sign, redact, crop), kept while the app runs. */
export const editors = $state<Record<string, { doc: OpenDoc | null; items: EditItem[]; page: number }>>({})
export const signatures = $state<Signature[]>([])
