import '@fontsource/plus-jakarta-sans/400.css'
import '@fontsource/plus-jakarta-sans/500.css'
import '@fontsource/plus-jakarta-sans/600.css'
import '@fontsource/plus-jakarta-sans/700.css'
import '@fontsource/plus-jakarta-sans/800.css'
import '@fontsource/jetbrains-mono/500.css'
import './app.css'
import { mount } from 'svelte'
import App from './App.svelte'

// Block the WebView's own context menu and browser shortcuts that make no sense in a desktop app.
window.addEventListener('contextmenu', (e) => {
  const t = e.target as HTMLElement
  if (!t.closest('input, textarea')) e.preventDefault()
})
window.addEventListener('keydown', (e) => {
  if ((e.ctrlKey && ['r', 'p', 'f', 'g', 'j', 'u'].includes(e.key.toLowerCase())) || e.key === 'F5' || e.key === 'F3') {
    e.preventDefault()
  }
})

// Never let the WebView open a dropped file itself; the Wails runtime delivers the paths.
for (const type of ['dragover', 'drop'] as const) {
  window.addEventListener(type, (e) => {
    if (e.dataTransfer?.types.includes('Files')) e.preventDefault()
  })
}

const app = mount(App, { target: document.getElementById('app')! })

export default app

// Test hook for automated UI tests in `wails dev` (never present in production builds).
if (import.meta.env.DEV) {
  Promise.all([import('./lib/stores/converter.svelte'), import('./lib/stores/download.svelte'), import('./lib/stores/app.svelte')]).then(
    ([conv, dl, appStore]) => {
      ;(window as any).__kmb = { conv, dl, app: appStore }
    },
  )
}
