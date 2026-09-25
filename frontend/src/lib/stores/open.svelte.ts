import { L } from '../i18n.svelte'
import { api, errText } from '../api'
import type { OpenRequest } from '../types'
import { nav, toast } from './app.svelte'
import { audioConv, imageConv, videoConv } from './converter.svelte'
import { convFor, pdfNav } from './pdf.svelte'

/** Opens files on the page that fits them (Explorer "Send to", app arguments, downloads). */
export async function openRequest(req: OpenRequest | null) {
  if (!req || req.paths.length === 0) return
  switch (req.page) {
    case 'image':
      nav.page = 'image'
      await imageConv.addPaths(req.paths)
      break
    case 'video':
      nav.page = 'video'
      await videoConv.addPaths(req.paths)
      break
    case 'audio':
      nav.page = 'audio'
      await audioConv.addPaths(req.paths)
      break
    case 'pdf': {
      nav.page = 'pdf'
      const pdfs = req.paths.filter((p) => p.toLowerCase().endsWith('.pdf'))
      pdfNav.tool = pdfs.length > 1 ? 'merge' : 'compress'
      await convFor(pdfNav.tool).addPaths(req.paths)
      break
    }
    default:
      toast(L('File ini tidak bisa diproses KuyMediaBox', "KuyMediaBox can't process these files"), 'info')
  }
}

/** Sends files (e.g. finished downloads) to the matching converter page. */
export async function sendToConverter(paths: string[]) {
  if (paths.length === 0) return
  try {
    const req = await api.routePaths(paths)
    if (!req.page) {
      toast(L('Tidak ada file yang bisa dikonversi', 'No convertible files'), 'info')
      return
    }
    await openRequest(req)
  } catch (e) {
    toast(errText(e), 'err')
  }
}
