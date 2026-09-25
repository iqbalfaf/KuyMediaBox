<script lang="ts">
  import { L } from '../../lib/i18n.svelte'
  import Icon from '../../components/Icon.svelte'
  import Switch from '../../components/Switch.svelte'
  import Select from '../../components/Select.svelte'
  import { api, errText } from '../../lib/api'
  import { toast } from '../../lib/stores/app.svelte'
  import { pdfForm, pdfOpts } from '../../lib/stores/pdf.svelte'
  import type { CertInfo } from '../../lib/types'

  const o = pdfOpts
  let info = $state<CertInfo | null>(null)
  let checking = $state(false)
  let showPw = $state(false)

  // Creating a personal certificate.
  let creating = $state(false)
  let nName = $state('')
  let nEmail = $state('')
  let nOrg = $state('')
  let nPw = $state('')
  let nPw2 = $state('')
  let busy = $state(false)

  async function pickCert() {
    try {
      const p = await api.pickFile(L('Pilih sertifikat', 'Choose a certificate'), L('Sertifikat (*.pfx, *.p12)', 'Certificates (*.pfx, *.p12)'), '*.pfx;*.p12')
      if (p) {
        o.digisign.certFile = p
        info = null
        pdfForm.certPassword = ''
      }
    } catch (e) {
      toast(errText(e), 'err')
    }
  }

  async function check() {
    if (!o.digisign.certFile || !pdfForm.certPassword) return
    checking = true
    try {
      info = await api.certInfo(o.digisign.certFile, pdfForm.certPassword)
    } catch (e) {
      info = null
      toast(errText(e), 'err')
    } finally {
      checking = false
    }
  }

  async function create() {
    if (!nName.trim() || nPw.length < 4 || nPw !== nPw2) return
    busy = true
    try {
      const p = await api.createCertificate(nName.trim(), nEmail.trim(), nOrg.trim(), 5, nPw)
      if (p) {
        o.digisign.certFile = p
        pdfForm.certPassword = nPw
        creating = false
        toast(L('Sertifikat dibuat dan dipilih', 'Certificate created and selected'), 'ok')
        await check()
      }
    } catch (e) {
      toast(errText(e), 'err')
    } finally {
      busy = false
    }
  }

  const certName = $derived(o.digisign.certFile ? o.digisign.certFile.split(/[\\/]/).pop() : '')
</script>

<div class="note"><Icon name="award" size={16} /><span>{L('Tanda tangan digital memakai sertifikat (PKCS#7). Pembaca PDF bisa memeriksa siapa penanda tangan dan apakah isi dokumen berubah setelah ditandatangani.', 'A digital signature uses a certificate (PKCS#7). PDF readers can check who signed and whether the document changed after signing.')}</span></div>

<div class="sec">
  <span class="label">{L('Sertifikat', 'Certificate')}</span>
  <div class="inline">
    <button class="btn grow" onclick={pickCert}><Icon name="file" size={16} /><span class="ellipsis">{certName || L('Pilih file .pfx / .p12…', 'Choose a .pfx / .p12 file…')}</span></button>
  </div>
  {#if o.digisign.certFile}
    <div class="inline">
      <input class="text-input" type={showPw ? 'text' : 'password'} bind:value={pdfForm.certPassword} placeholder={L('Password sertifikat', 'Certificate password')} autocomplete="off" onblur={check} onkeydown={(e) => e.key === 'Enter' && check()} />
      <button class="btn icon" aria-label={showPw ? L('Sembunyikan', 'Hide') : L('Tampilkan', 'Show')} onclick={() => (showPw = !showPw)}><Icon name={showPw ? 'eyeOff' : 'eye'} size={16} /></button>
    </div>
    {#if checking}
      <p class="hint">{L('Memeriksa sertifikat…', 'Checking the certificate…')}</p>
    {:else if info}
      <div class="cert">
        <b>{info.name}</b>
        {#if info.email}<span>{info.email}</span>{/if}
        <span>{L('Berlaku', 'Valid')} {info.notBefore} – {info.notAfter}</span>
        <span class:warn={info.selfSigned}>{info.selfSigned ? L('Sertifikat pribadi (self-signed): pembaca PDF menandainya "belum dipercaya" sampai ditambahkan sebagai tepercaya.', 'Personal (self-signed) certificate: PDF readers show it as "not trusted" until it is added as trusted.') : `${L('Diterbitkan oleh', 'Issued by')} ${info.issuer}`}</span>
      </div>
    {/if}
    <p class="hint">{L('Password hanya dipakai saat menandatangani dan tidak disimpan.', 'The password is only used while signing and is never saved.')}</p>
  {/if}
  {#if !creating}
    <button class="link" onclick={() => (creating = true)}>{L('Belum punya? Buat sertifikat pribadi', "Don't have one? Create a personal certificate")}</button>
  {:else}
    <div class="create">
      <b>{L('Sertifikat pribadi baru', 'New personal certificate')}</b>
      <input class="text-input" bind:value={nName} placeholder={L('Nama lengkap', 'Full name')} />
      <input class="text-input" bind:value={nEmail} placeholder={L('Email (opsional)', 'Email (optional)')} />
      <input class="text-input" bind:value={nOrg} placeholder={L('Organisasi (opsional)', 'Organisation (optional)')} />
      <input class="text-input" type="password" bind:value={nPw} placeholder={L('Password (min. 4 karakter)', 'Password (min. 4 characters)')} autocomplete="new-password" />
      <input class="text-input" type="password" bind:value={nPw2} placeholder={L('Ulangi password', 'Repeat the password')} autocomplete="new-password" />
      {#if nPw2 && nPw !== nPw2}<p class="hint bad">{L('Password tidak sama.', "Passwords don't match.")}</p>{/if}
      <p class="hint">{L('Berlaku 5 tahun. Simpan file .p12 dan password-nya baik-baik.', 'Valid for 5 years. Keep the .p12 file and its password safe.')}</p>
      <div class="inline end">
        <button class="btn" onclick={() => (creating = false)}>{L('Batal', 'Cancel')}</button>
        <button class="btn-accent" disabled={busy || !nName.trim() || nPw.length < 4 || nPw !== nPw2} onclick={create}>{busy ? L('Membuat…', 'Creating…') : L('Buat & simpan', 'Create & save')}</button>
      </div>
    </div>
  {/if}
</div>

<div class="sec">
  <span class="label">{L('Keterangan (opsional)', 'Details (optional)')}</span>
  <input class="text-input" bind:value={o.digisign.reason} placeholder={L('Alasan, mis. Menyetujui dokumen', 'Reason, e.g. I approve this document')} />
  <input class="text-input" bind:value={o.digisign.location} placeholder={L('Lokasi, mis. Jakarta', 'Location, e.g. Jakarta')} />
  <input class="text-input" bind:value={o.digisign.contact} placeholder={L('Kontak (email/telepon)', 'Contact (email/phone)')} />
</div>

<div class="sec">
  <Switch bind:checked={o.digisign.visible} label={L('Tampilkan kotak tanda tangan', 'Show a signature box')} hint={L('Kotak kecil berisi nama & tanggal di halaman', 'A small box with name & date on the page')} />
  {#if o.digisign.visible}
    <div class="grid2">
      <Select
        label={L('Halaman', 'Page')}
        bind:value={o.digisign.page}
        options={[
          { value: 'first', label: L('Halaman pertama', 'First page') },
          { value: 'last', label: L('Halaman terakhir', 'Last page') },
        ]}
      />
      <Select
        label={L('Posisi', 'Position')}
        bind:value={o.digisign.position}
        options={[
          { value: 'br', label: L('Kanan bawah', 'Bottom right') },
          { value: 'bl', label: L('Kiri bawah', 'Bottom left') },
          { value: 'tr', label: L('Kanan atas', 'Top right') },
          { value: 'tl', label: L('Kiri atas', 'Top left') },
        ]}
      />
    </div>
  {/if}
  <p class="hint">{L('Tanda tangan digital lama di PDF sumber tidak ikut terbawa. Untuk tanda tangan gambar/tulisan tangan, pakai alat "Tanda tangan PDF" dulu lalu tandatangani secara digital.', 'Earlier digital signatures of the source are not kept. For a drawn signature, use "Sign PDF" first, then sign digitally.')}</p>
</div>

<style>
  .sec {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .inline {
    display: flex;
    gap: 8px;
    align-items: center;
    min-width: 0;
  }
  .inline.end {
    justify-content: flex-end;
  }
  .grow {
    flex: 1;
    min-width: 0;
    justify-content: flex-start;
  }
  .grid2 {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px;
  }
  .note {
    display: flex;
    gap: 10px;
    padding: 12px;
    border-radius: 10px;
    background: var(--info-soft);
    color: var(--info-text);
    font-size: 12px;
    line-height: 1.5;
  }
  .cert {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 10px 12px;
    border-radius: 10px;
    background: var(--ok-soft);
    font-size: 12px;
    color: var(--text-2);
  }
  .cert b {
    color: var(--text);
    font-size: 13px;
  }
  .cert .warn {
    color: var(--warn);
  }
  .create {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 12px;
    border-radius: 12px;
    border: 1px solid var(--border);
    background: var(--surface-2);
  }
  .link {
    align-self: flex-start;
    padding: 0;
    border: 0;
    background: none;
    font-size: 12px;
    font-weight: 700;
    color: var(--accent-text-2);
  }
  .bad {
    color: var(--err);
  }
</style>
