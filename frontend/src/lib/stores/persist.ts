/** Reads a JSON value from localStorage merged over defaults (never throws). */
export function load<T extends object>(key: string, defaults: T): T {
  try {
    const raw = localStorage.getItem(key)
    if (!raw) return structuredClone(defaults)
    const parsed = JSON.parse(raw)
    if (parsed && typeof parsed === 'object') return deepMerge(structuredClone(defaults), parsed)
  } catch {
    /* ignore corrupt or unavailable storage */
  }
  return structuredClone(defaults)
}

export function save(key: string, value: unknown) {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch {
    /* storage full or unavailable: settings just won't persist */
  }
}

function isPlain(v: unknown): v is Record<string, unknown> {
  return !!v && typeof v === 'object' && !Array.isArray(v)
}

/** Saved values win; fields added in newer versions keep their defaults (at any depth). */
function deepMerge<T>(base: T, saved: any): T {
  const out: any = base
  for (const k of Object.keys(saved)) {
    if (isPlain(out[k]) && isPlain(saved[k])) out[k] = deepMerge(out[k], saved[k])
    else if (saved[k] !== undefined) out[k] = saved[k]
  }
  return out
}
