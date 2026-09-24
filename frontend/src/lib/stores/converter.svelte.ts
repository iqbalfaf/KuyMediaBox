import { api, errText } from '../api'
import type { FileItem, JobRef, Kind, TaskInfo } from '../types'
import { toast } from './app.svelte'
import { isActive, tasks, trackBatch } from './tasks.svelte'

export type ItemState = 'invalid' | 'ready' | 'queued' | 'running' | 'done' | 'failed' | 'canceled' | 'skipped'

/** File list + task mapping for one converter page (image, video or audio). */
export class Converter {
  kind: Kind
  items = $state<FileItem[]>([])
  taskOf = $state<Record<string, string>>({})
  adding = $state(false)
  starting = $state(false)

  constructor(kind: Kind) {
    this.kind = kind
  }

  task(item: FileItem): TaskInfo | undefined {
    const id = this.taskOf[item.id]
    return id ? tasks[id] : undefined
  }

  state(item: FileItem): ItemState {
    if (item.error) return 'invalid'
    const t = this.task(item)
    if (!t) return this.taskOf[item.id] ? 'queued' : 'ready'
    return t.status
  }

  get running(): boolean {
    return this.items.some((it) => {
      const id = this.taskOf[it.id]
      return !!id && (!tasks[id] || isActive(tasks[id]))
    })
  }

  get totalSize(): number {
    return this.items.reduce((s, it) => s + (it.size || 0), 0)
  }

  /** Items that the Start button would process. */
  pending(): FileItem[] {
    const valid = this.items.filter((it) => !it.error)
    const fresh = valid.filter((it) => {
      const s = this.state(it)
      return s === 'ready' || s === 'failed' || s === 'canceled'
    })
    return fresh.length > 0 ? fresh : valid.filter((it) => !isActive(this.task(it)))
  }

  /** Items never started (or failed/canceled) — what "Tambah ke antrian" adds during a run. */
  fresh(): FileItem[] {
    return this.items.filter((it) => {
      const s = this.state(it)
      return s === 'ready' || s === 'failed' || s === 'canceled'
    })
  }

  hasUnprocessed(): boolean {
    return this.items.some((it) => {
      const s = this.state(it)
      return s === 'ready' || s === 'failed' || s === 'canceled'
    })
  }

  merge(list: FileItem[]) {
    const known = new Set(this.items.map((it) => it.path.toLowerCase()))
    const fresh = list.filter((it) => !known.has(it.path.toLowerCase()))
    this.items.push(...fresh)
    const skipped = list.length - fresh.length
    if (list.length === 0) toast('Tidak ada file yang cocok untuk halaman ini', 'info')
    else if (skipped > 0 && fresh.length === 0) toast('File sudah ada di daftar', 'info')
    const bad = fresh.filter((it) => it.error).length
    if (bad > 0) toast(`${bad} file tidak bisa dibaca dan ditandai merah`, 'err')
  }

  async addPaths(paths: string[]) {
    if (paths.length === 0) return
    this.adding = true
    try {
      this.merge(await api.addPaths(this.kind, paths))
    } catch (e) {
      toast(errText(e), 'err')
    } finally {
      this.adding = false
    }
  }

  async pickFiles() {
    this.adding = true
    try {
      const list = await api.pickFiles(this.kind)
      if (list.length) this.merge(list)
    } catch (e) {
      toast(errText(e), 'err')
    } finally {
      this.adding = false
    }
  }

  async pickFolder() {
    this.adding = true
    try {
      const list = await api.pickFolder(this.kind)
      if (list.length) this.merge(list)
      else if (list.length === 0) {
        /* cancelled or empty folder: nothing to do */
      }
    } catch (e) {
      toast(errText(e), 'err')
    } finally {
      this.adding = false
    }
  }

  remove(item: FileItem) {
    const t = this.task(item)
    if (isActive(t)) {
      api.cancelTask(t!.id)
      return
    }
    this.items = this.items.filter((it) => it.id !== item.id)
    delete this.taskOf[item.id]
  }

  clear() {
    if (this.running) {
      toast('Tunggu konversi selesai atau batalkan dulu', 'info')
      return
    }
    this.items = []
    this.taskOf = {}
  }

  clearFinished() {
    this.items = this.items.filter((it) => {
      const s = this.state(it)
      const gone = s === 'done' || s === 'skipped'
      if (gone) delete this.taskOf[it.id]
      return !gone
    })
  }

  /** Starts the given items through a backend call returning job refs. */
  async start(items: FileItem[], run: (jobs: { id: string; path: string }[]) => Promise<JobRef[]>) {
    if (items.length === 0 || this.starting) return
    this.starting = true
    try {
      const refs = await run(items.map((it) => ({ id: it.id, path: it.path })))
      for (const r of refs) this.taskOf[r.itemId] = r.taskId
      trackBatch(this.kind, refs.map((r) => r.taskId))
    } catch (e) {
      toast(errText(e), 'err')
    } finally {
      this.starting = false
    }
  }

  cancelAll() {
    api.cancelKind(this.kind)
  }

  /** The output folder of the most recent finished file, for "Buka folder hasil". */
  lastOutput(): string {
    let best = ''
    let at = 0
    for (const it of this.items) {
      const t = this.task(it)
      if (t && t.output && t.finished >= at) {
        best = t.output
        at = t.finished
      }
    }
    return best
  }
}

export const imageConv = new Converter('image')
export const videoConv = new Converter('video')
export const audioConv = new Converter('audio')
