<script lang="ts">
  import { L } from '../lib/i18n.svelte'
  import Select from './Select.svelte'
  import type { Denoise } from '../lib/types'

  let { value = $bindable() }: { value: Denoise } = $props()
</script>

<div class="sec">
  <Select
    label={L('Kurangi noise', 'Reduce noise')}
    bind:value
    options={[
      { value: 'off', label: L('Kurangi noise: tidak', 'Reduce noise: off') },
      { value: 'light', label: L('Kurangi noise: ringan', 'Reduce noise: light') },
      { value: 'medium', label: L('Kurangi noise: sedang', 'Reduce noise: medium') },
      { value: 'strong', label: L('Kurangi noise: kuat', 'Reduce noise: strong') },
    ]}
  />
  {#if value !== 'off'}
    <p class="hint">
      {L('Mendesis, dengung AC/kipas, dan angin diredam — cocok untuk rekaman suara, podcast, dan kuliah.', 'Hiss, air-conditioning/fan hum and wind are reduced — good for voice recordings, podcasts and lectures.')}
      {value === 'strong' ? L(' Level kuat bisa membuat suara sedikit "robotik".', ' Strong can make voices sound slightly "robotic".') : ''}
    </p>
  {/if}
</div>

<style>
  .sec {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .hint {
    margin: 0;
  }
</style>
