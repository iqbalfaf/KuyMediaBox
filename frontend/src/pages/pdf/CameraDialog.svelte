<script lang="ts">
  import { L } from '../../lib/i18n.svelte'
  import Icon from '../../components/Icon.svelte'
  import { api, errText } from '../../lib/api'
  import { toast } from '../../lib/stores/app.svelte'
  import type { FileItem } from '../../lib/types'

  let { open = $bindable(false), onshot }: { open: boolean; onshot: (it: FileItem) => void } = $props()

  let dialog: HTMLDialogElement
  let video = $state<HTMLVideoElement>()
  let stream: MediaStream | null = null
  let devices = $state<MediaDeviceInfo[]>([])
  let deviceId = $state('')
  let error = $state('')
  let count = $state(0)
  let flash = $state(false)

  async function startCam() {
    stopCam()
    error = ''
    try {
      stream = await navigator.mediaDevices.getUserMedia({
        video: deviceId ? { deviceId: { exact: deviceId }, width: { ideal: 3840 }, height: { ideal: 2160 } } : { width: { ideal: 3840 }, height: { ideal: 2160 } },
        audio: false,
      })
      if (video) video.srcObject = stream
      devices = (await navigator.mediaDevices.enumerateDevices()).filter((d) => d.kind === 'videoinput')
      if (!deviceId && devices.length) deviceId = stream.getVideoTracks()[0]?.getSettings().deviceId ?? devices[0].deviceId
    } catch (e) {
      error = L('Kamera tidak bisa dibuka. Pastikan kamera terhubung dan izin kamera untuk aplikasi desktop aktif di Pengaturan Windows › Privasi › Kamera.', "The camera can't be opened. Make sure it's connected and camera access for desktop apps is on in Windows Settings › Privacy › Camera.")
      console.warn(e)
    }
  }

  function stopCam() {
    stream?.getTracks().forEach((t) => t.stop())
    stream = null
  }

  $effect(() => {
    if (open && dialog && !dialog.open) {
      count = 0
      dialog.showModal()
      startCam()
    }
    if (!open && dialog?.open) {
      stopCam()
      dialog.close()
    }
  })

  async function shoot() {
    if (!video || !video.videoWidth) return
    const c = document.createElement('canvas')
    c.width = video.videoWidth
    c.height = video.videoHeight
    c.getContext('2d')!.drawImage(video, 0, 0)
    flash = true
    setTimeout(() => (flash = false), 150)
    try {
      const it = await api.pdfSaveCapture(c.toDataURL('image/jpeg', 0.92))
      if (it) {
        onshot(it)
        count++
      }
    } catch (e) {
      toast(errText(e), 'err')
    }
  }
</script>

<dialog bind:this={dialog} onclose={() => (open = false)}>
  <div class="box">
    <div class="head">
      <b>{L('Ambil foto dokumen', 'Photograph documents')}</b>
      {#if devices.length > 1}
        <select class="dev" bind:value={deviceId} onchange={startCam} aria-label={L('Kamera', 'Camera')}>
          {#each devices as d, i (d.deviceId)}<option value={d.deviceId}>{d.label || `${L('Kamera', 'Camera')} ${i + 1}`}</option>{/each}
        </select>
      {/if}
      <button class="btn icon" aria-label={L('Tutup', 'Close')} onclick={() => (open = false)}><Icon name="x" size={16} /></button>
    </div>
    <div class="view" class:flash>
      {#if error}
        <p class="err">{error}</p>
      {:else}
        <!-- svelte-ignore a11y_media_has_caption -->
        <video bind:this={video} autoplay playsinline muted></video>
      {/if}
    </div>
    <div class="foot">
      <span class="hint">{count ? L(`${count} foto ditambahkan`, `${count} photo${count === 1 ? '' : 's'} added`) : L('Letakkan dokumen di tempat terang, lalu ambil foto per halaman.', 'Put the document in good light and take one photo per page.')}</span>
      <button class="btn" onclick={() => (open = false)}>{L('Selesai', 'Done')}</button>
      <button class="btn-accent" onclick={shoot} disabled={!!error}><Icon name="camera" size={16} />{L('Ambil foto', 'Take photo')}</button>
    </div>
  </div>
</dialog>

<style>
  dialog {
    padding: 0;
    border: 1px solid var(--border-strong);
    border-radius: 16px;
    background: var(--surface);
    color: var(--text);
    width: min(860px, 92vw);
    box-shadow: 0 24px 64px rgba(0, 0, 0, 0.55);
  }
  dialog::backdrop {
    background: rgba(5, 6, 8, 0.7);
  }
  .box {
    display: flex;
    flex-direction: column;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 14px 14px 12px 20px;
  }
  .head b {
    flex-grow: 1;
  }
  .dev {
    height: 34px;
    max-width: 280px;
    border-radius: 8px;
    border: 1px solid var(--border);
    background: var(--surface-2);
    color: var(--text);
    padding: 0 8px;
  }
  .view {
    background: #000;
    aspect-ratio: 16 / 10;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: opacity 0.15s;
  }
  .view.flash {
    opacity: 0.3;
  }
  video {
    width: 100%;
    height: 100%;
    object-fit: contain;
  }
  .err {
    max-width: 460px;
    text-align: center;
    color: var(--warn);
    font-size: 13px;
    line-height: 1.5;
  }
  .foot {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 14px 20px;
  }
  .foot .hint {
    flex-grow: 1;
  }
</style>
