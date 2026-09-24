<script lang="ts">
  import { onMount } from 'svelte'
  import Sidebar from './components/Sidebar.svelte'
  import Toasts from './components/Toasts.svelte'
  import DetailModal from './components/DetailModal.svelte'
  import ImagePage from './pages/ImagePage.svelte'
  import VideoPage from './pages/VideoPage.svelte'
  import AudioPage from './pages/AudioPage.svelte'
  import DownloadPage from './pages/DownloadPage.svelte'
  import SettingsPage from './pages/SettingsPage.svelte'
  import { runtime } from './lib/api'
  import { initApp, nav, toast } from './lib/stores/app.svelte'
  import { initTasks } from './lib/stores/tasks.svelte'
  import { audioConv, imageConv, videoConv } from './lib/stores/converter.svelte'
  import { addLinks } from './lib/stores/download.svelte'

  const labels: Record<string, string> = { image: 'Konversi gambar', video: 'Konversi video', audio: 'Konversi audio', download: 'Download' }

  onMount(() => {
    initApp()
    initTasks()

    runtime.onFileDrop((_x, _y, paths) => {
      switch (nav.page) {
        case 'image':
          imageConv.addPaths(paths)
          break
        case 'video':
          videoConv.addPaths(paths)
          break
        case 'audio':
          audioConv.addPaths(paths)
          break
        case 'download':
          toast('Di halaman Download, tempel link (bukan file). Pindah ke Gambar/Video/Audio untuk konversi file.', 'info')
          break
        default:
          toast('Buka halaman Gambar, Video, atau Audio untuk menambahkan file.', 'info')
      }
    })

    runtime.on('batch:done', (b: { kind: string; done: number; failed: number; skipped: number; canceled: number }) => {
      if (b.done + b.failed + b.skipped === 0) return
      const parts = [`${b.done} berhasil`]
      if (b.failed) parts.push(`${b.failed} gagal`)
      if (b.skipped) parts.push(`${b.skipped} dilewati`)
      toast(`${labels[b.kind] ?? 'Tugas'} selesai: ${parts.join(' · ')}`, b.failed ? 'err' : 'ok')
    })
  })

  // Ctrl+V anywhere on the Download page adds links from the clipboard.
  function onPaste(e: ClipboardEvent) {
    if (nav.page !== 'download') return
    const target = e.target as HTMLElement
    if (target.closest('input, textarea')) return
    const text = e.clipboardData?.getData('text') ?? ''
    if (text.trim()) {
      e.preventDefault()
      addLinks(text)
    }
  }
</script>

<svelte:window onpaste={onPaste} />

<div class="shell">
  <Sidebar />
  <main class="main" style="--wails-drop-target: drop">
    {#if nav.page === 'image'}
      <ImagePage />
    {:else if nav.page === 'video'}
      <VideoPage />
    {:else if nav.page === 'audio'}
      <AudioPage />
    {:else if nav.page === 'download'}
      <DownloadPage />
    {:else}
      <SettingsPage />
    {/if}
    <div class="drop-overlay" aria-hidden="true">
      <div class="drop-box">Lepaskan file untuk menambahkan</div>
    </div>
  </main>
</div>

<Toasts />
<DetailModal />

<style>
  .shell {
    height: 100%;
    display: flex;
    background: var(--bg);
  }
  .main {
    flex-grow: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    position: relative;
  }
  .drop-overlay {
    position: absolute;
    inset: 8px;
    border: 2px dashed var(--accent);
    border-radius: 18px;
    background: rgba(255, 122, 69, 0.08);
    display: none;
    align-items: center;
    justify-content: center;
    pointer-events: none;
    z-index: 30;
  }
  :global(.wails-drop-target-active) .drop-overlay {
    display: flex;
  }
  .drop-box {
    padding: 14px 22px;
    border-radius: 12px;
    background: var(--accent);
    color: var(--accent-ink);
    font-weight: 800;
    font-size: 15px;
  }
</style>
