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
