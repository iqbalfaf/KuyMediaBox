import { api, errText, runtime } from '../api'
import type { UpdateInfo } from '../types'
import { toast } from './app.svelte'

export const upd = $state<{
  version: string
  info: UpdateInfo | null
  open: boolean
  checking: boolean
  installing: boolean
  stage: 'download' | 'install' | ''
  progress: number
  error: string
}>({ version: '', info: null, open: false, checking: false, installing: false, stage: '', progress: 0, error: '' })

let started = false

export async function initUpdate() {
  if (started) return
  started = true
  runtime.on('update:available', (info: UpdateInfo) => {
    upd.info = info
    upd.open = true // show once on startup; afterwards reachable from the sidebar
  })
  runtime.on('update:progress', (p: { stage: 'download' | 'install'; progress: number }) => {
    upd.stage = p.stage
    upd.progress = p.progress
  })
  try {
    upd.version = await api.getVersion()
  } catch {
    /* ignore */
  }
}

/** Manual check from the Settings page. */
export async function checkNow() {
  if (upd.checking) return
  upd.checking = true
  try {
    const info = await api.checkUpdate()
    upd.info = info
    if (info.available) upd.open = true
    else toast(`KuyMediaBox sudah versi terbaru (v${info.current})`, 'ok')
  } catch (e) {
    toast(`Cek update gagal: ${errText(e)}`, 'err')
  } finally {
    upd.checking = false
  }
}

export async function installNow() {
  if (upd.installing) return
  upd.installing = true
  upd.error = ''
  upd.stage = 'download'
  upd.progress = 0
  try {
    await api.installUpdate()
    upd.stage = 'install' // the app quits and restarts by itself
  } catch (e) {
    upd.error = errText(e)
    upd.installing = false
    upd.stage = ''
  }
}
