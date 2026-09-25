import { api, errText, runtime } from '../api'
import type { AfterQueue, HistoryEntry } from '../types'
import { toast } from './app.svelte'

/** Finished tasks, newest first (G-11). */
export const history = $state<{ list: HistoryEntry[]; loaded: boolean }>({ list: [], loaded: false })

let started = false

export async function initHistory() {
  if (started) return
  started = true
  runtime.on('history:add', (e: HistoryEntry) => {
    history.list.unshift(e)
    if (history.list.length > 5000) history.list.length = 5000
  })
  await reloadHistory()
}

export async function reloadHistory() {
  try {
    history.list = (await api.getHistory()) ?? []
  } catch (e) {
    toast(errText(e), 'err')
  } finally {
    history.loaded = true
  }
}

export async function removeHistory(ids: string[]) {
  try {
    await api.removeHistory(ids)
    const drop = new Set(ids)
    history.list = history.list.filter((e) => !drop.has(e.id))
  } catch (e) {
    toast(errText(e), 'err')
  }
}

export async function clearHistory() {
  try {
    await api.clearHistory()
    history.list = []
  } catch (e) {
    toast(errText(e), 'err')
  }
}

/** What happens when the whole queue is done (G-14). Not saved: a new session starts with "none". */
export const after = $state<AfterQueue>({ action: 'none', pending: false, seconds: 0 })

let afterStarted = false
let tick: ReturnType<typeof setInterval> | null = null

export async function initAfter() {
  if (afterStarted) return
  afterStarted = true
  runtime.on('after:changed', (a: AfterQueue) => setAfter(a))
  try {
    setAfter(await api.getAfterQueue())
  } catch {
    /* backend not ready */
  }
}

function setAfter(a: AfterQueue) {
  Object.assign(after, a)
  if (tick) clearInterval(tick)
  tick = null
  if (a.pending) {
    tick = setInterval(() => {
      if (after.seconds > 0) after.seconds--
    }, 1000)
  }
}

export async function chooseAfter(action: AfterQueue['action']) {
  try {
    setAfter(await api.setAfterQueue(action))
  } catch (e) {
    toast(errText(e), 'err')
  }
}

export async function cancelAfter() {
  try {
    setAfter(await api.cancelAfterQueue())
  } catch (e) {
    toast(errText(e), 'err')
  }
}
