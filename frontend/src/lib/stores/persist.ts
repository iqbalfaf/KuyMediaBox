/** Reads a JSON value from localStorage merged over defaults (never throws). */
export function load<T extends object>(key: string, defaults: T): T {
  try {
    const raw = localStorage.getItem(key)
    if (!raw) return structuredClone(defaults)
    const parsed = JSON.parse(raw)
    if (parsed && typeof parsed === 'object') return { ...structuredClone(defaults), ...parsed }
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
