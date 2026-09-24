<script lang="ts">
  // Same artwork as build/appicon.svg (the .exe / taskbar icon).
  let { size = 36 }: { size?: number } = $props()
</script>

<svg width={size} height={size} viewBox="0 0 1024 1024" role="img" aria-label="KuyMediaBox">
  <rect width="1024" height="1024" rx="232" fill="#FF7A45" />
  <g stroke-linejoin="round" stroke-width="16">
    <path d="M512 208 L776 356 L512 502 L248 356 Z" fill="#4A2A1E" stroke="#4A2A1E" />
    <path d="M248 356 L512 502 L512 816 L248 664 Z" fill="#1C110A" stroke="#1C110A" />
    <path d="M512 502 L776 356 L776 664 L512 816 Z" fill="#2E1B14" stroke="#2E1B14" />
    <path d="M332 496 L457 585 L332 674 Z" fill="#FF7A45" stroke="#FF7A45" stroke-width="18" />
  </g>
</svg>

<style>
  svg {
    display: block;
    flex-shrink: 0;
  }
</style>
