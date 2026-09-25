<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import Icon from './Icon.svelte'
  import Switch from './Switch.svelte'
  import Select from './Select.svelte'
  import { api, errText } from '../lib/api'
  import { saveSettings, settings, toast } from '../lib/stores/app.svelte'

  let tpl = $state(settings.value?.nameTemplate ?? '')
  let spTpl = $state(settings.value?.spotifyTemplate ?? '')
  $effect(() => {
    if (!settings.value) return
    if (document.activeElement?.id !== 'tpl-yt') tpl = settings.value.nameTemplate
    if (document.activeElement?.id !== 'tpl-sp') spTpl = settings.value.spotifyTemplate
  })

  function saveTpl() {
    if (settings.value && (tpl !== settings.value.nameTemplate || spTpl !== settings.value.spotifyTemplate)) {
      saveSettings({ nameTemplate: tpl, spotifyTemplate: spTpl })
    }
  }

  async function pickCookies() {
    try {
      const p = await api.pickFile(L('Pilih file cookies.txt', 'Choose a cookies.txt file'), 'cookies.txt', '*.txt')
      if (p) saveSettings({ cookiesFile: p, cookiesBrowser: '' })
    } catch (e) {
      toast(errText(e), 'err')
    }
  }

  function example(t: string, spotify: boolean): string {
    const vals: Record<string, string> = spotify
      ? { artist: 'Raisa', title: 'Kali Kedua', album: 'Handmade', year: '2016', index: '03' }
      : { title: 'Judul Video', uploader: 'Nama Channel', channel: 'Nama Channel', date: '2026-09-25', year: '2026', id: 'dQw4w9WgXcQ', index: '03', playlist: 'Playlist', res: '1080p' }
    return t.replace(/\{([a-z]+)\}/g, (m, k) => vals[k] ?? m)
  }
</script>

{#if settings.value}
  <section class="card" aria-label={L('Pengaturan download', 'Download settings')}>
    <div class="head"><h2>Download</h2></div>
    <div class="gbody">
      <div class="sec">
        <span class="label">{L('Login lewat cookies browser', 'Sign in with browser cookies')}</span>
        <Select
          label={L('Browser', 'Browser')}
          value={settings.value.cookiesFile ? 'file' : settings.value.cookiesBrowser}
          onchange={(v) => (v === 'file' ? pickCookies() : saveSettings({ cookiesBrowser: v, cookiesFile: '' }))}
          options={[
            { value: '', label: L('Tidak memakai cookies', 'No cookies') },
            { value: 'firefox', label: 'Firefox' },
            { value: 'edge', label: 'Microsoft Edge' },
            { value: 'chrome', label: 'Google Chrome' },
            { value: 'brave', label: 'Brave' },
            { value: 'opera', label: 'Opera' },
            { value: 'vivaldi', label: 'Vivaldi' },
            { value: 'chromium', label: 'Chromium' },
            { value: 'file', label: L('File cookies.txt…', 'cookies.txt file…') },
          ]}
        />
        {#if settings.value.cookiesFile}
          <span class="hint ellipsis" title={settings.value.cookiesFile}><Icon name="file" size={12} /> {settings.value.cookiesFile}</span>
        {/if}
        <p class="hint">
          {L('Untuk video dibatasi umur, khusus member, atau post yang butuh login. Login dulu di browser tersebut.', 'For age-restricted, members-only or login-only posts. Sign in with that browser first.')}
          {#if settings.value.cookiesBrowser && settings.value.cookiesBrowser !== 'firefox'}{L(' Chrome/Edge/Brave harus ditutup saat mengunduh; bila gagal, pakai Firefox atau file cookies.txt.', ' Close Chrome/Edge/Brave while downloading; if it fails, use Firefox or a cookies.txt file.')}{/if}
        </p>
      </div>

      <div class="sec">
        <label class="label" for="tpl-yt">{L('Template nama file (YouTube & situs lain)', 'File name template (YouTube & other sites)')}</label>
        <input id="tpl-yt" class="text-input" bind:value={tpl} onblur={saveTpl} onkeydown={(e) => e.key === 'Enter' && (e.currentTarget as HTMLInputElement).blur()} placeholder={L('(bawaan: judul video)', '(default: video title)')} maxlength="120" />
        <p class="hint"><code>{'{title}'}</code> <code>{'{uploader}'}</code> <code>{'{date}'}</code> <code>{'{year}'}</code> <code>{'{id}'}</code> <code>{'{index}'}</code> <code>{'{playlist}'}</code> <code>{'{res}'}</code></p>
        {#if tpl}<p class="hint">{L('Contoh', 'Example')}: {example(tpl, false)}.mp4</p>{/if}
      </div>

      <div class="sec">
        <label class="label" for="tpl-sp">{L('Template nama file Spotify', 'Spotify file name template')}</label>
        <input id="tpl-sp" class="text-input" bind:value={spTpl} onblur={saveTpl} onkeydown={(e) => e.key === 'Enter' && (e.currentTarget as HTMLInputElement).blur()} placeholder={L('(bawaan: Artis - Judul)', '(default: Artist - Title)')} maxlength="120" />
        <p class="hint"><code>{'{artist}'}</code> <code>{'{title}'}</code> <code>{'{album}'}</code> <code>{'{year}'}</code> <code>{'{index}'}</code></p>
        {#if spTpl}<p class="hint">{L('Contoh', 'Example')}: {example(spTpl, true)}.mp3</p>{/if}
      </div>

      <div class="sec">
        <span class="label">{L('Batas kecepatan download', 'Download speed limit')}</span>
        <Select
          label={L('Batas kecepatan download', 'Download speed limit')}
          value={settings.value.downloadLimitKB ?? 0}
          onchange={(v) => saveSettings({ downloadLimitKB: v })}
          options={[
            { value: 0, label: L('Tanpa batas', 'No limit') },
            { value: 256, label: '256 KB/s' },
            { value: 512, label: '512 KB/s' },
            { value: 1024, label: '1 MB/s' },
            { value: 2048, label: '2 MB/s' },
            { value: 5120, label: '5 MB/s' },
            { value: 10240, label: '10 MB/s' },
          ]}
        />
        <p class="hint">{L('Total untuk semua unduhan yang berjalan bersamaan, supaya internet tetap lancar untuk yang lain. Unduhan yang terputus dilanjutkan otomatis.', 'Total for all downloads running at once, so the connection stays usable for everything else. Interrupted downloads resume automatically.')}</p>
      </div>

      <Switch
        checked={settings.value.spotifyLogin}
        onchange={(v) => saveSettings({ spotifyLogin: v })}
        label={L('Login Spotify (playlist private)', 'Spotify sign-in (private playlists)')}
        hint={L('Saat membaca link, browser terbuka untuk login Spotify sekali', 'When reading a link, the browser opens once for the Spotify login')}
      />
      <Switch
        checked={settings.value.clipboardWatch}
        onchange={(v) => saveSettings({ clipboardWatch: v })}
        label={L('Pantau clipboard', 'Watch the clipboard')}
        hint={L('Link yang disalin otomatis masuk ke halaman Download', 'Copied links are added to the Download page automatically')}
      />
    </div>
  </section>
{/if}

<style>
  .head {
    display: flex;
    align-items: center;
    padding: 18px 20px;
    border-bottom: 1px solid var(--border);
  }
  h2 {
    margin: 0;
    font-size: 16px;
    font-weight: 800;
  }
  .gbody {
    padding: 18px 20px 20px;
    display: flex;
    flex-direction: column;
    gap: 18px;
  }
  .sec {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  code {
    font-family: var(--mono);
    font-size: 11px;
    padding: 1px 4px;
    border-radius: 4px;
    background: var(--surface-2);
    color: var(--accent-text-2);
  }
  .ellipsis {
    display: flex;
    align-items: center;
    gap: 4px;
  }
</style>
