<script lang="ts">
  import CoverArt from './CoverArt.svelte';

  let { name = 'Unknown', artist = '', trackCount = 0, coverUrl = '', onclick }: {
    name?: string;
    artist?: string;
    trackCount?: number;
    coverUrl?: string;
    onclick?: () => void;
  } = $props();
</script>

<button
  type="button"
  class="group bg-white dark:bg-gray-800 rounded-2xl border border-gray-200 dark:border-gray-700 overflow-hidden hover:shadow-lg hover:border-primary-300 dark:hover:border-primary-700 transition-all duration-200 text-left focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500"
  onclick={onclick}
>
  <!-- Cover art -->
  <CoverArt src={coverUrl} alt={name} size="full" />

  <!-- Info -->
  <div class="p-3 space-y-1">
    <p class="text-sm font-semibold text-gray-900 dark:text-white truncate leading-tight group-hover:text-primary-600 dark:group-hover:text-primary-400 transition-colors" title={name}>
      {name}
    </p>
    {#if artist}
      <p class="text-xs text-gray-500 dark:text-gray-400 truncate">{artist}</p>
    {/if}
    <p class="text-xs text-gray-400 dark:text-gray-500">{trackCount} track{trackCount !== 1 ? 's' : ''}</p>
  </div>
</button>
