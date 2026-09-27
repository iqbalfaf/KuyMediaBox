import { api, errText, runtime } from '../api'
import { L } from '../i18n.svelte'
import { toast } from './app.svelte'
import type { ModelStatus } from '../types'

/** Downloadable AI models (speech recognition, background removal), by kind. */
export const modelStore = $state<Record<string, ModelStatus[]>>({ whisper: [], bgremove: [] })

let listening = false

export async function loadModels(kind: 'whisper' | 'bgremove') {
  if (!listening) {
    listening = true
    runtime.on('models:changed', (list: ModelStatus[]) => {
      if (list?.length) modelStore[list[0].kind] = list
    })
  }
  try {
    modelStore[kind] = (await api.listModels(kind)) ?? []
  } catch {
    /* the list stays empty */
  }
}

export function modelReady(kind: string, id: string): boolean {
  return !!modelStore[kind]?.find((m) => m.id === id)?.installed
}

export async function downloadModel(kind: string, id: string) {
  try {
    await api.installModel(kind, id)
  } catch (e) {
    toast(`${L('Model gagal diunduh', 'Model download failed')}: ${errText(e)}`, 'err')
  }
}
