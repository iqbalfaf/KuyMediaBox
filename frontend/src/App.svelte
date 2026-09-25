<script lang="ts">
  import { L } from './lib/i18n.svelte'
  import { onMount } from 'svelte'
  import Sidebar from './components/Sidebar.svelte'
  import Toasts from './components/Toasts.svelte'
  import DetailModal from './components/DetailModal.svelte'
  import UpdateDialog from './components/UpdateDialog.svelte'
  import ImagePage from './pages/ImagePage.svelte'
  import VideoPage from './pages/VideoPage.svelte'
  import AudioPage from './pages/AudioPage.svelte'
  import DownloadPage from './pages/DownloadPage.svelte'
  import SettingsPage from './pages/SettingsPage.svelte'
  import PdfPage from './pages/PdfPage.svelte'
  import PasswordDialog from './components/PasswordDialog.svelte'
  import TextDialog from './components/TextDialog.svelte'
  import TagDialog from './components/TagDialog.svelte'
  import CompareDialog from './components/CompareDialog.svelte'
  import AfterQueueBanner from './components/AfterQueueBanner.svelte'
  import HistoryPage from './pages/HistoryPage.svelte'
  import { initAfter, initHistory } from './lib/stores/history.svelte'
  import { openRequest } from './lib/stores/open.svelte'
  import type { OpenRequest } from './lib/types'
  import { convFor, pdfDrop, pdfNav } from './lib/stores/pdf.svelte'
  import { toolById } from './lib/pdfTools'
  import { api, runtime } from './lib/api'
  import { initApp, nav, settings, toast } from './lib/stores/app.svelte'
  import { initTasks } from './lib/stores/tasks.svelte'
  import { audioConv, imageConv, videoConv } from './lib/stores/converter.svelte'
  import { addLinks } from './lib/stores/download.svelte'
  import { initUpdate } from './lib/stores/update.svelte'

  function batchTitle(kind: string): string {
    switch (kind) {
      case 'image':
        return L('Konversi gambar selesai', 'Image conversion finished')
      case 'video':
        return L('Konversi video selesai', 'Video conversion finished')
      case 'audio':
        return L('Konversi audio selesai', 'Audio conversion finished')
      case 'download':
        return L('Download selesai', 'Download finished')
      case 'pdf':
        return L('Alat PDF selesai', 'PDF tools finished')
    }
    return L('Tugas selesai', 'Tasks finished')
  }

  onMount(() => {
    initApp()
    initTasks()
    initUpdate()
    initHistory()
    initAfter()

    // Files from Explorer ("Send to") — at start and while running.
    api.takeLaunchFiles().then(openRequest).catch(() => {})
    runtime.on('files:open', (req: OpenRequest) => openRequest(req))
    runtime.on('watch:queued', (w: { dir: string; count: number }) =>
      toast(L(`Folder pantauan: ${w.count} file baru diproses`, `Watched folder: ${w.count} new file${w.count === 1 ? '' : 's'} queued`), 'info'),
    )
    runtime.on('watch:error', (w: { dir: string; error: string }) => toast(`${L('Folder pantauan', 'Watched folder')}: ${w.error}`, 'err'))

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
        case 'pdf':
          if (pdfDrop.fn) pdfDrop.fn(paths)
          else if (!pdfNav.tool) {
            // On the overview a drop of PDFs opens the most likely tool: merge for several, compress for one.
            const pdfs = paths.filter((p) => p.toLowerCase().endsWith('.pdf'))
            if (pdfs.length) {
              pdfNav.tool = pdfs.length > 1 ? 'merge' : 'compress'
              convFor(pdfNav.tool).addPaths(pdfs)
            } else toast(L('Pilih alat PDF dulu, lalu tarik file ke sana.', 'Pick a PDF tool first, then drop the files on it.'), 'info')
          } else if (toolById(pdfNav.tool)) convFor(pdfNav.tool).addPaths(paths)
          break
        case 'download':
          toast(L('Di halaman Download, tempel link (bukan file). Pindah ke Gambar/Video/Audio untuk konversi file.', 'The Download page takes links, not files. Switch to Images/Video/Audio to convert files.'), 'info')
          break
        default:
          toast(L('Buka halaman Gambar, Video, Audio, atau PDF untuk menambahkan file.', 'Open the Images, Video, Audio or PDF page to add files.'), 'info')
      }
    })

    runtime.on('batch:done', (b: { kind: string; done: number; failed: number; skipped: number; canceled: number }) => {
      if (b.done + b.failed + b.skipped === 0) return
      const parts = [`${b.done} ${L('berhasil', 'succeeded')}`]
      if (b.failed) parts.push(`${b.failed} ${L('gagal', 'failed')}`)
      if (b.skipped) parts.push(`${b.skipped} ${L('dilewati', 'skipped')}`)
      toast(`${batchTitle(b.kind)}: ${parts.join(' · ')}`, b.failed ? 'err' : 'ok')
    })
  })

  // Clipboard monitor (DL-16): copied links go to the Download page. Text already on the
  // clipboard when monitoring starts is ignored.
  let clipTimer: ReturnType<typeof setInterval> | null = null
  let lastClip: string | null = null
  $effect(() => {
    const on = !!settings.value?.clipboardWatch
    if (on && !clipTimer) {
      lastClip = null
      clipTimer = setInterval(async () => {
        let text = ''
        try {
          text = ((await runtime.clipboardText()) ?? '').trim()
        } catch {
          return
        }
        if (lastClip === null) {
          lastClip = text
          return
        }
        if (text === lastClip || !text) return
        lastClip = text
        if (text.length > 20000 || !/(https?:\/\/|www\.)\S+/i.test(text)) return
        await addLinks(text)
      }, 1500)
    } else if (!on && clipTimer) {
      clearInterval(clipTimer)
      clipTimer = null
    }
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
    {:else if nav.page === 'pdf'}
      <PdfPage />
    {:else if nav.page === 'history'}
      <HistoryPage />
    {:else}
      <SettingsPage />
    {/if}
    <div class="drop-overlay" aria-hidden="true">
      <div class="drop-box">{L('Lepaskan file untuk menambahkan', 'Drop files to add them')}</div>
    </div>
  </main>
</div>

<Toasts />
<DetailModal />
<UpdateDialog />
<PasswordDialog />
<TextDialog />
<TagDialog />
<CompareDialog />
<AfterQueueBanner />

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
