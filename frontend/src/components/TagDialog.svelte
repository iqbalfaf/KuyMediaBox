<script lang="ts" module>
  import type { AudioTags, FileItem } from '../lib/types'

  /** Tag editor state (AUD-07): the file being edited and a callback that stores the result. */
  export const tagEdit = $state<{
    open: boolean
    item: FileItem | null
    tags: AudioTags
    count: number
    save: ((t: AudioTags, all: boolean) => void) | null
  }>({ open: false, item: null, tags: emptyTags(), count: 0, save: null })

  export function emptyTags(): AudioTags {
    return { title: '', artist: '', album: '', albumArtist: '', year: '', genre: '', track: '', cover: '', removeCover: false }
  }

  /** Tags read from the file (ffprobe) as editable fields. */
  export function tagsFromFile(it: FileItem): AudioTags {
    const t = it.tags ?? {}
    return {
      ...emptyTags(),
      title: t.title ?? '',
      artist: t.artist ?? '',
      album: t.album ?? '',
      albumArtist: t.album_artist ?? '',
      year: (t.date ?? '').slice(0, 4),
      genre: t.genre ?? '',
      track: t.track ?? '',
    }
  }

  export function editTags(item: FileItem, current: AudioTags, count: number, save: (t: AudioTags, all: boolean) => void) {
    Object.assign(tagEdit, { open: true, item, tags: { ...current }, count, save })
  }
</script>

<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'
  import { api, errText, imageUrl } from '../lib/api'
  import { toast } from '../lib/stores/app.svelte'

  let dialog: HTMLDialogElement
  let t = $state<AudioTags>(emptyTags())

  $effect(() => {
    if (tagEdit.open && dialog && !dialog.open) {
      t = { ...tagEdit.tags }
      dialog.showModal()
    }
    if (!tagEdit.open && dialog?.open) dialog.close()
  })

  async function pickCover() {
    try {
      const p = await api.pickFile(L('Pilih gambar cover', 'Choose a cover picture'), L('Gambar', 'Images'), '*.jpg;*.jpeg;*.png;*.webp')
      if (p) {
        t.cover = p
        t.removeCover = false
      }
    } catch (e) {
      toast(errText(e), 'err')
    }
  }

  function save(all: boolean) {
    tagEdit.save?.($state.snapshot(t), all)
    tagEdit.open = false
  }

  const fields = $derived<{ key: keyof AudioTags; label: string; wide?: boolean }[]>([
    { key: 'title', label: L('Judul', 'Title'), wide: true },
    { key: 'artist', label: L('Artis', 'Artist') },
    { key: 'albumArtist', label: L('Artis album', 'Album artist') },
    { key: 'album', label: 'Album', wide: true },
    { key: 'year', label: L('Tahun', 'Year') },
    { key: 'track', label: L('Nomor lagu', 'Track no.') },
    { key: 'genre', label: 'Genre', wide: true },
  ])
</script>

<dialog bind:this={dialog} onclose={() => (tagEdit.open = false)} onclick={(e) => e.target === dialog && (tagEdit.open = false)}>
  <div class="box">
    <div class="head">
      <span class="ic"><Icon name="tag" /></span>
      <div class="ht">
        <b>{L('Edit tag lagu', 'Edit song tags')}</b>
        <span class="ellipsis" title={tagEdit.item?.path}>{tagEdit.item?.name}</span>
      </div>
      <button class="btn icon" aria-label={L('Tutup', 'Close')} onclick={() => (tagEdit.open = false)}><Icon name="x" size={16} /></button>
    </div>
    <div class="content">
      <div class="cover">
        <div class="art">
          {#if t.cover}
            <img src={imageUrl(t.cover, 240)} alt="" />
          {:else if tagEdit.item?.hasCover && !t.removeCover}
            <span class="has"><Icon name="image" size={28} /><span>{L('Cover asli', 'Original cover')}</span></span>
          {:else}
            <span class="has"><Icon name="music" size={28} /><span>{L('Tanpa cover', 'No cover')}</span></span>
          {/if}
        </div>
        <button class="btn" onclick={pickCover}><Icon name="image" size={14} />{L('Ganti cover', 'Change cover')}</button>
        {#if t.cover || (tagEdit.item?.hasCover && !t.removeCover)}
          <button class="link" onclick={() => { t.cover = ''; t.removeCover = true }}>{L('Hapus cover', 'Remove cover')}</button>
        {/if}
      </div>
      <div class="grid">
        {#each fields as f (f.key)}
          <label class:wide={f.wide}>
            <span>{f.label}</span>
            <input class="text-input" value={t[f.key] as string} oninput={(e) => ((t as any)[f.key] = (e.currentTarget as HTMLInputElement).value)} />
          </label>
        {/each}
      </div>
    </div>
    <p class="hint">{L('Tag ditulis ke file hasil. Kolom yang dikosongkan akan dihapus dari file. Cover didukung untuk MP3, M4A & FLAC.', 'Tags are written to the result. Empty fields are removed from the file. Covers work for MP3, M4A & FLAC.')}</p>
    <div class="foot">
      {#if tagEdit.count > 1}
        <button class="btn" onclick={() => save(true)} title={L('Artis, album, tahun, genre & cover dipakai untuk semua file', 'Artist, album, year, genre & cover go to every file')}>{L(`Album/artis untuk semua (${tagEdit.count})`, `Album/artist for all (${tagEdit.count})`)}</button>
      {/if}
      <span class="sp"></span>
      <button class="btn" onclick={() => (tagEdit.open = false)}>{L('Batal', 'Cancel')}</button>
      <button class="btn-accent" onclick={() => save(false)}>{L('Simpan tag', 'Save tags')}</button>
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
    width: min(640px, 92vw);
    box-shadow: 0 24px 64px rgba(0, 0, 0, 0.55);
  }
  dialog::backdrop {
    background: rgba(5, 6, 8, 0.6);
  }
  .box {
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 18px 20px;
  }
  .head {
    display: flex;
    align-items: center;
    gap: 12px;
  }
  .ic {
    width: 36px;
    height: 36px;
    border-radius: 10px;
    background: var(--accent-tint);
    color: var(--accent);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }
  .ht {
    flex-grow: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .ht span {
    font-size: 12px;
    color: var(--text-3);
  }
  .content {
    display: flex;
    gap: 16px;
  }
  .cover {
    width: 150px;
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
    align-items: stretch;
  }
  .art {
    width: 150px;
    height: 150px;
    border-radius: 12px;
    background: var(--surface-2);
    border: 1px solid var(--border);
    overflow: hidden;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .art img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .has {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--text-3);
  }
  .grid {
    flex-grow: 1;
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px;
    align-content: start;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }
  label.wide {
    grid-column: span 2;
  }
  label span {
    font-size: 11px;
    font-weight: 700;
    color: var(--text-3);
  }
  .foot {
    display: flex;
    gap: 8px;
    align-items: center;
  }
  .sp {
    flex-grow: 1;
  }
  .link {
    padding: 0;
    border: 0;
    background: none;
    font-size: 12px;
    font-weight: 700;
    color: var(--accent-text-2);
  }
</style>
