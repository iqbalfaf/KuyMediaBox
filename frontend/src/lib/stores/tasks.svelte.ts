import { api, runtime } from '../api'
import type { Kind, TaskInfo } from '../types'

/** Every task the backend reported, by id. */
export const tasks = $state<Record<string, TaskInfo>>({})

/** The task ids of the current (or last) run per kind, used for aggregate progress. */
export const batches = $state<Record<string, string[]>>({ image: [], video: [], audio: [], download: [] })

export function isActive(t: TaskInfo | undefined): boolean {
  return !!t && (t.status === 'queued' || t.status === 'running')
}

/** Adds newly started task ids to the kind's batch (a new batch starts when the old one is idle). */
export function trackBatch(kind: Kind, ids: string[]) {
  const current = batches[kind] ?? []
  const stillRunning = current.some((id) => isActive(tasks[id]))
  batches[kind] = stillRunning ? [...current, ...ids] : [...ids]
}

export interface BatchSummary {
  total: number
  finished: number
  active: number
  done: number
  failed: number
  skipped: number
  canceled: number
  progress: number
  etaSeconds: number
}

export function summarize(kind: Kind): BatchSummary {
  const ids = batches[kind] ?? []
  const s: BatchSummary = { total: ids.length, finished: 0, active: 0, done: 0, failed: 0, skipped: 0, canceled: 0, progress: 0, etaSeconds: 0 }
  if (ids.length === 0) return s
  let sum = 0
  let started = Infinity
  for (const id of ids) {
    const t = tasks[id]
    if (!t) {
      s.active++
      continue
    }
    if (t.started && t.started < started) started = t.started
    switch (t.status) {
      case 'queued':
        s.active++
        break
      case 'running':
        s.active++
        sum += t.progress > 0 ? Math.min(1, t.progress) : 0
        break
      default:
        s.finished++
        sum += 1
        if (t.status === 'done') s.done++
        else if (t.status === 'failed') s.failed++
        else if (t.status === 'skipped') s.skipped++
        else if (t.status === 'canceled') s.canceled++
    }
  }
  s.progress = sum / ids.length
  if (started !== Infinity && s.progress > 0.02 && s.active > 0) {
    const elapsed = (Date.now() - started) / 1000
    s.etaSeconds = (elapsed / s.progress) * (1 - s.progress)
  }
  return s
}

let started = false

/** Stores a task snapshot unless a newer one (higher seq) is already known. Events can arrive
 *  out of order, and a late "running" must never overwrite "done". */
function apply(info: TaskInfo) {
  const cur = tasks[info.id]
  if (cur && (info.seq ?? 0) < (cur.seq ?? 0)) return
  tasks[info.id] = info
}

async function reconcile() {
  try {
    for (const t of await api.listTasks()) apply(t)
  } catch {
    /* backend not ready yet; events will fill in */
  }
}

/** Subscribes to backend task events (idempotent). */
export async function initTasks() {
  if (started) return
  started = true
  runtime.on('task:update', (info: TaskInfo) => apply(info))
  await reconcile()
  // Safety net: while something runs, re-read the task list now and then so a lost event
  // can never leave the UI showing a finished task as still running.
  setInterval(() => {
    if (Object.values(tasks).some(isActive)) reconcile()
  }, 2000)
}

/** A clock that ticks every second so ETA texts refresh. */
export const clock = $state({ now: Date.now() })
setInterval(() => (clock.now = Date.now()), 1000)
