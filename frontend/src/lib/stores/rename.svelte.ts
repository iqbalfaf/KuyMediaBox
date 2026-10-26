import { Converter } from './converter.svelte'
import { load, save } from './persist'
import type { RenameOptions, RenamePlan, RenameResult } from '../types'

/** The file list of the bulk rename page (any file type). */
export const renameConv = new Converter('pdf', 'any')

export const renameDefaults: RenameOptions = {
  mode: 'pattern',
  labels: 'Nama Peserta Didik, Nama Siswa, Nama Murid, Nama Lengkap, Nama',
  template: '{nama}',
  pattern: '{file} {n}',
  start: 1,
  step: 1,
  digits: 2,
  find: '',
  with: '',
  regex: false,
  matchCase: false,
  prefix: '',
  suffix: '',
  cutStart: 0,
  cutEnd: 0,
  case: 'keep',
  tidy: false,
  extLower: false,
}

/** Options, remembered between sessions. */
export const renameOpts = $state<RenameOptions>(load('kmb.rename', renameDefaults))

export const renameState = $state<{
  /** Proposed names by item id, and the names typed by hand. */
  plans: Record<string, RenamePlan>
  edits: Record<string, string>
  /** Results of the last run by item id, and the run itself (for undo). */
  results: Record<string, RenameResult>
  last: RenameResult[]
  /** Copy into this folder instead of renaming in place. */
  copy: boolean
  copyDir: string
}>({ plans: {}, edits: {}, results: {}, last: [], copy: false, copyDir: '' })

export function persistRename() {
  save('kmb.rename', $state.snapshot(renameOpts))
}
