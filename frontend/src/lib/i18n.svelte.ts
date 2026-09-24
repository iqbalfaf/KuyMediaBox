import type { Lang } from './types'

const KEY = 'kmb.lang'

function initial(): Lang {
  try {
    if (localStorage.getItem(KEY) === 'en') return 'en'
  } catch {
    /* storage unavailable */
  }
  return 'id'
}

/** The UI language. Saved settings win; localStorage only avoids a flash of the wrong language at start. */
export const i18n = $state<{ lang: Lang }>({ lang: initial() })

document.documentElement.lang = i18n.lang

/** Picks the Indonesian or English text. Reactive when called from markup or $derived. */
export function L(id: string, en: string): string {
  return i18n.lang === 'en' ? en : id
}

export function setLang(lang: Lang | undefined) {
  const l: Lang = lang === 'en' ? 'en' : 'id'
  i18n.lang = l
  document.documentElement.lang = l
  try {
    localStorage.setItem(KEY, l)
  } catch {
    /* storage unavailable */
  }
}

/** BCP 47 locale for Intl formatting. */
export function locale(): string {
  return i18n.lang === 'en' ? 'en-US' : 'id-ID'
}
