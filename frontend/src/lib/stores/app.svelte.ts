import { api, errText, runtime } from '../api'
import { setLang } from '../i18n.svelte'
import type { Capabilities, OutputKind, OutputSpec, Page, Settings, ToolStatus } from '../types'

export const nav = $state<{ page: Page }>({ page: 'image' })

export const settings = $state<{ value: Settings | null }>({ value: null })

/** Default result folder per module (Pictures\KuyMediaBox, Videos\KuyMediaBox, …). */
export const defaultDirs = $state<Record<string, string>>({})

/** The saved result-folder setting of a module. */
export function outputOf(kind: OutputKind): OutputSpec {
  return settings.value?.outputs?.[kind] ?? { mode: 'default', dir: '' }
}

/** The fixed folder a module writes to, or '' when it is dynamic (next to each source file). */
export function fixedFolder(kind: OutputKind): string {
  const o = outputOf(kind)
  if (o.mode === 'custom') return o.dir
  if (o.mode === 'default') return defaultDirs[kind] ?? ''
  return ''
}

export async function setOutput(kind: OutputKind, spec: OutputSpec) {
  if (!settings.value) return
  await saveSettings({ outputs: { ...settings.value.outputs, [kind]: spec } })
}

/** "C:\Users\a\Pictures\KuyMediaBox" → "Pictures › KuyMediaBox" */
export function shortPath(p: string): string {
  if (!p) return ''
  const parts = p.split(/[\\/]+/).filter(Boolean)
  return parts.length <= 2 ? parts.join(' › ') : parts.slice(-2).join(' › ')
}

export const toolState = $state<{ list: ToolStatus[]; loaded: boolean }>({ list: [], loaded: false })

export const caps = $state<Capabilities>({ ffmpeg: false, encoders: {} })

export function tool(id: string): ToolStatus | undefined {
  return toolState.list.find((t) => t.id === id)
}

export function hasTool(id: string): boolean {
  return !!tool(id)?.found
}

/** Number of tools that need attention (missing or outdated). */
export function toolAttention(): { missing: number; updates: number } {
  let missing = 0
  let updates = 0
  for (const t of toolState.list) {
    if (!t.found) missing++
    else if (t.updateAvailable) updates++
  }
  return { missing, updates }
}

export async function refreshCaps() {
  try {
    const c = await api.getCapabilities()
    caps.ffmpeg = c.ffmpeg
    caps.encoders = c.encoders ?? {}
  } catch {
    /* ignore */
  }
}

let started = false
let lastFFmpegPath = ''

export async function initApp() {
  if (started) return
  started = true
  runtime.on('tools:changed', (list: ToolStatus[]) => {
    toolState.list = list
    toolState.loaded = true
    const ff = list.find((t) => t.id === 'ffmpeg')
    const path = ff?.found ? ff.path : ''
    if (path !== lastFFmpegPath) {
      lastFFmpegPath = path
      refreshCaps()
    }
  })
  try {
    const [s, dirs] = await Promise.all([api.getSettings(), api.getDefaultDirs()])
    Object.assign(defaultDirs, dirs)
    settings.value = s
    setLang(s.language)
  } catch (e) {
    toast(errText(e), 'err')
  }
  try {
    toolState.list = await api.getTools()
  } catch {
    /* filled by events */
  }
}

export async function saveSettings(patch: Partial<Settings>) {
  if (!settings.value) return
  try {
    settings.value = await api.saveSettings({ ...settings.value, ...patch })
    setLang(settings.value.language)
  } catch (e) {
    toast(errText(e), 'err')
  }
}

// ---- toasts ------------------------------------------------------------------------------

export interface Toast {
  id: number
  text: string
  tone: 'ok' | 'err' | 'info'
}

export const toasts = $state<Toast[]>([])
let toastSeq = 0

export function toast(text: string, tone: Toast['tone'] = 'info') {
  const id = ++toastSeq
  toasts.push({ id, text, tone })
  setTimeout(() => dismiss(id), tone === 'err' ? 7000 : 4000)
}

export function dismiss(id: number) {
  const i = toasts.findIndex((t) => t.id === id)
  if (i >= 0) toasts.splice(i, 1)
}

// ---- error detail dialog -------------------------------------------------------------------

export const detail = $state<{ open: boolean; title: string; message: string; text: string }>({
  open: false,
  title: '',
  message: '',
  text: '',
})

export function showDetail(title: string, message: string, text: string) {
  detail.title = title
  detail.message = message
  detail.text = text
  detail.open = true
}
