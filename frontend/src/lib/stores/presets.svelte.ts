import { load, save } from './persist'

/** A named set of module settings (G-09). Built-in presets can't be deleted. */
export interface Preset {
  id: string
  name: string
  value: any
  builtin?: boolean
}

const key = (module: string) => `kmb.presets.${module}`

export const presetStore = $state<Record<string, Preset[]>>({})

function ensure(module: string) {
  if (!presetStore[module]) presetStore[module] = load<{ list: Preset[] }>(key(module), { list: [] }).list
}

/** User presets of a module. Read-only (safe in markup); loading happens on first write/mount. */
export function userPresets(module: string): Preset[] {
  return presetStore[module] ?? load<{ list: Preset[] }>(key(module), { list: [] }).list
}

export function savePreset(module: string, name: string, value: any): Preset {
  ensure(module)
  const list = presetStore[module].filter((p) => p.name.toLowerCase() !== name.toLowerCase())
  const p: Preset = { id: `p${Date.now()}`, name, value: structuredClone(value) }
  presetStore[module] = [...list, p]
  save(key(module), { list: $state.snapshot(presetStore[module]) })
  return p
}

export function deletePreset(module: string, id: string) {
  ensure(module)
  presetStore[module] = presetStore[module].filter((p) => p.id !== id)
  save(key(module), { list: $state.snapshot(presetStore[module]) })
}

/** Deep-merges a preset over the current settings, so presets saved by older versions keep new fields. */
export function mergeInto(target: any, value: any) {
  for (const k of Object.keys(value ?? {})) {
    const v = value[k]
    if (v && typeof v === 'object' && !Array.isArray(v) && target[k] && typeof target[k] === 'object' && !Array.isArray(target[k])) mergeInto(target[k], v)
    else target[k] = structuredClone(v)
  }
}
