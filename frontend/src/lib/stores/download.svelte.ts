import { L } from '../i18n.svelte'
import { api, errText } from '../api'
import type { Collection, DownloadOptions, Entry, Link } from '../types'
import { settings, toast } from './app.svelte'
import { load, save } from './persist'
import { isActive, tasks, trackBatch } from './tasks.svelte'

export type Scope = 'all' | 'latest' | 'since'

export interface LinkRow {
  id: string
  raw: string
  link: Link
  status: 'loading' | 'ready' | 'error'
  error: string
  col: Collection | null
  opts: DownloadOptions
  selected: Record<string, boolean>
  types: Record<string, boolean> // channel tabs: videos, shorts, streams
  scope: Scope
  latestN: number
  since: string // yyyy-mm-dd
  range: string
  taskOf: Record<string, string>
}

const youtubeDefaults: DownloadOptions = {
  mode: 'video', quality: '1080', container: 'mp4', audioFormat: 'mp3', audioQuality: 'auto',
  embed: true, skipExisting: true, numbering: true,
}
const spotifyDefaults: DownloadOptions = {
  mode: 'audio', quality: '1080', container: 'mp4', audioFormat: 'mp3', audioQuality: 'auto',
  embed: true, skipExisting: true, numbering: true,
}

export const dl = $state<{ rows: LinkRow[]; active: string; input: string; starting: boolean }>({
  rows: [],
  active: '',
  input: '',
  starting: false,
})

let rowSeq = 0

export function activeRow(): LinkRow | undefined {
  return dl.rows.find((r) => r.id === dl.active) ?? dl.rows[0]
}

function optsKey(source: string) {
  return source === 'spotify' ? 'kmb.dl.spotify' : 'kmb.dl.youtube'
}

export function rememberOpts(row: LinkRow) {
  save(optsKey(row.link.source), row.opts)
}

export const typeLabel: Record<string, string> = {
  video: 'Video', playlist: 'Playlist', channel: 'Channel', get track() { return L('Lagu', 'Track') }, album: 'Album', unknown: 'Link',
}

export const tabLabel: Record<string, string> = { get videos() { return L('Video', 'Videos') }, shorts: 'Shorts', streams: 'Live' }

/** Adds every link found in text and reads them one by one. */
export async function addLinks(text: string) {
  let links: Link[]
  try {
    links = await api.detectLinks(text)
  } catch (e) {
    toast(errText(e), 'err')
    return
  }
  if (links.length === 0) {
    toast(L('Tidak ada link yang dikenali. Tempel link YouTube atau Spotify.', 'No recognizable links. Paste a YouTube or Spotify link.'), 'err')
    return
  }
  const known = new Set(dl.rows.map((r) => r.link.url))
  let added = 0
  for (const link of links) {
    if (known.has(link.url)) continue
    known.add(link.url)
    const defaults = link.source === 'spotify' ? spotifyDefaults : youtubeDefaults
    const opts = load(optsKey(link.source), defaults)
    if (settings.value) opts.skipExisting = opts.skipExisting ?? settings.value.skipDownloaded
    const row: LinkRow = {
      id: `l${++rowSeq}`,
      raw: link.url,
      link,
      status: 'loading',
      error: '',
      col: null,
      opts,
      selected: {},
      types: { videos: true, shorts: false, streams: false },
      scope: 'all',
      latestN: 20,
      since: '',
      range: '',
      taskOf: {},
    }
    dl.rows.push(row)
    if (!dl.active) dl.active = row.id
    added++
    analyze(row.id)
  }
  if (added === 0) toast(L('Link sudah ada di daftar', 'The link is already in the list'), 'info')
}

function findRow(id: string) {
  return dl.rows.find((r) => r.id === id)
}

export async function analyze(id: string) {
  const row = findRow(id)
  if (!row) return
  row.status = 'loading'
  row.error = ''
  if (row.link.type === 'unknown') {
    row.status = 'error'
    row.error = row.link.source === 'spotify' ? L('Gunakan link lagu, album, atau playlist Spotify', 'Use a Spotify track, album or playlist link') : L('Link tidak dikenali', 'Link not recognized')
    return
  }
  try {
    const col = await api.analyzeLink(row.link.url)
    const r = findRow(id) // the row may have been removed meanwhile
    if (!r) {
      api.forgetCollection(col.key)
      return
    }
    r.col = col
    r.status = 'ready'
    if (col.type === 'playlist' || col.type === 'album') r.range = col.entries.length ? `1-${col.entries.length}` : ''
    if (r.link.type === 'channel') {
      // Default to the tab that has content.
      const counts = col.tabCounts ?? {}
      r.types = { videos: (counts.videos ?? 0) > 0, shorts: false, streams: false }
      if (!r.types.videos) r.types = { videos: false, shorts: (counts.shorts ?? 0) > 0, streams: (counts.streams ?? 0) > 0 }
    }
    applyScope(r)
  } catch (e) {
    const r = findRow(id)
    if (!r) return
    r.status = 'error'
    r.error = errText(e)
  }
}

export function removeRow(id: string) {
  const row = findRow(id)
  if (!row) return
  for (const tid of Object.values(row.taskOf)) {
    if (isActive(tasks[tid])) api.cancelTask(tid)
  }
  if (row.col) api.forgetCollection(row.col.key)
  dl.rows = dl.rows.filter((r) => r.id !== id)
  if (dl.active === id) dl.active = dl.rows[0]?.id ?? ''
}

/** Entries shown for a row (channel: only the chosen content types). */
export function visible(row: LinkRow): Entry[] {
  if (!row.col) return []
  if (row.link.type !== 'channel') return row.col.entries
  return row.col.entries.filter((e) => row.types[e.tab])
}

export function selectable(e: Entry, row: LinkRow): boolean {
  return !e.unavailable && !(row.opts.skipExisting && e.archived)
}

export function selectedEntries(row: LinkRow): Entry[] {
  return visible(row).filter((e) => row.selected[e.id] && selectable(e, row))
}

/** Recomputes the selection from scope/range filters. */
export function applyScope(row: LinkRow) {
  const list = visible(row)
  const sel: Record<string, boolean> = {}
  if (row.link.type === 'channel') {
    const perTab: Record<string, number> = {}
    const since = row.since.replace(/-/g, '')
    for (const e of list) {
      if (!selectable(e, row)) continue
      if (row.scope === 'latest') {
        perTab[e.tab] = (perTab[e.tab] ?? 0) + 1
        if (perTab[e.tab] > Math.max(1, row.latestN)) continue
      } else if (row.scope === 'since') {
        if (!since || !e.date || e.date < since) continue
      }
      sel[e.id] = true
    }
  } else {
    for (const e of list) if (selectable(e, row)) sel[e.id] = true
  }
  row.selected = sel
}

export function setAll(row: LinkRow, on: boolean) {
  const sel: Record<string, boolean> = {}
  if (on) for (const e of visible(row)) if (selectable(e, row)) sel[e.id] = true
  row.selected = sel
}

/** Applies a "1-20" / "3,5,7-9" range to a playlist or album. */
export function applyRange(row: LinkRow, positions: Set<number>) {
  const sel: Record<string, boolean> = {}
  for (const e of visible(row)) if (positions.has(e.index) && selectable(e, row)) sel[e.id] = true
  row.selected = sel
}

/** Entries of a row that would be queued now (selected, not already queued/done). */
export function toQueue(row: LinkRow): Entry[] {
  return selectedEntries(row).filter((e) => {
    const t = tasks[row.taskOf[e.id]]
    if (!row.taskOf[e.id]) return true
    if (!t) return false
    return t.status === 'failed' || t.status === 'canceled'
  })
}

export function pendingCount(): number {
  let n = 0
  for (const r of dl.rows) if (r.status === 'ready') n += toQueue(r).length
  return n
}

export function runningCount(): number {
  let n = 0
  for (const r of dl.rows) for (const tid of Object.values(r.taskOf)) if (isActive(tasks[tid])) n++
  return n
}

export async function startAll() {
  if (dl.starting) return
  dl.starting = true
  let queued = 0
  try {
    for (const row of dl.rows) {
      if (row.status !== 'ready' || !row.col) continue
      const list = toQueue(row)
      if (list.length === 0) continue
      try {
        const refs = await api.startDownloads(row.col.key, list.map((e) => e.id), $state.snapshot(row.opts) as DownloadOptions)
        for (const r of refs) row.taskOf[r.itemId] = r.taskId
        trackBatch('download', refs.map((r) => r.taskId))
        queued += refs.length
      } catch (e) {
        toast(`${row.col.title}: ${errText(e)}`, 'err')
      }
    }
    if (queued === 0) toast(L('Belum ada item baru yang dipilih', 'No new items selected'), 'info')
  } finally {
    dl.starting = false
  }
}

export function cancelAll() {
  api.cancelKind('download')
}
