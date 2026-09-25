/** A small modal that asks for a PDF password. */
export const pwPrompt = $state<{ open: boolean; name: string; wrong: boolean; resolve: ((v: string | null) => void) | null }>({
  open: false,
  name: '',
  wrong: false,
  resolve: null,
})

/** Asks for the password of a file; resolves null when cancelled. */
export function askPassword(name: string, wrong = false): Promise<string | null> {
  pwPrompt.resolve?.(null)
  return new Promise((resolve) => {
    pwPrompt.name = name
    pwPrompt.wrong = wrong
    pwPrompt.resolve = resolve
    pwPrompt.open = true
  })
}

export function answerPassword(v: string | null) {
  const r = pwPrompt.resolve
  pwPrompt.resolve = null
  pwPrompt.open = false
  r?.(v)
}

/** A small modal that asks for one line of text (preset names, links, …). */
export const textPrompt = $state<{
  open: boolean
  title: string
  label: string
  value: string
  placeholder: string
  ok: string
  resolve: ((v: string | null) => void) | null
}>({ open: false, title: '', label: '', value: '', placeholder: '', ok: '', resolve: null })

/** Asks for a text; resolves null when cancelled. */
export function askText(title: string, label: string, value = '', placeholder = '', ok = ''): Promise<string | null> {
  textPrompt.resolve?.(null)
  return new Promise((resolve) => {
    Object.assign(textPrompt, { title, label, value, placeholder, ok, resolve, open: true })
  })
}

export function answerText(v: string | null) {
  const r = textPrompt.resolve
  textPrompt.resolve = null
  textPrompt.open = false
  r?.(v)
}
