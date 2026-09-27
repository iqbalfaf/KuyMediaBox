import { L } from '../i18n.svelte'
import { runtime } from '../api'
import { toast } from './app.svelte'
import type { FlowResult } from '../types'

/** Files that went through a workflow in this session, newest first. */
export const flowResults = $state<{ list: (FlowResult & { at: number })[] }>({ list: [] })

/** Where files dropped on the window go while the workflow page is open. */
export const flowDrop: { fn: ((paths: string[]) => void) | null } = { fn: null }

let started = false
export function initFlows() {
  if (started) return
  started = true
  runtime.on('flow:done', (r: FlowResult) => {
    flowResults.list = [{ ...r, at: Date.now() }, ...flowResults.list].slice(0, 100)
    if (r.error) {
      const name = r.input ? r.input.split(/[\/]/).pop() : ''
      toast(`${L('Alur kerja', 'Workflow')} "${r.workflow}"${name ? ` · ${name}` : ''}: ${r.error}`, 'err')
    }
  })
}
