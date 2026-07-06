<script lang="ts">
  import { onMount } from 'svelte';
  import { trackLibrary, tracksByGenre } from '$lib/stores';
  import CoverArt from '$lib/components/CoverArt.svelte';
  import { goto } from '$app/navigation';

  let loaded = $state(false);

  onMount(async () => {
    if ($trackLibrary.length === 0) {
      await trackLibrary.refresh();
    }
    loaded = true;
  });

  let entries = $derived($tracksByGenre);
  let searchQuery = $state('');

  let filtered = $derived(
    searchQuery.trim()
      ? entries.filter(e => e.key.toLowerCase().includes(searchQuery.toLowerCase()))
      : entries
  );
</script>

{#if !loaded}
  <div class="flex items-center justify-center py-16">
    <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500"></div>
  </div>
{:else}
  <div class="mb-4">
    <input
      type="text"
      placeholder="Search genres..."
      bind:value={searchQuery}
      class="w-full sm:w-80 px-3 py-2 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
    />
  </div>

  {#if filtered.length === 0}
    <div class="flex flex-col items-center justify-center py-16 text-gray-400 dark:text-gray-500">
      <span class="text-4xl mb-3">🏷️</span>
      <p class="text-sm font-medium">No genres found</p>
    </div>
  {:else}
    <div class="flex flex-wrap gap-3">
      {#each filtered as entry (entry.key)}
        <button
          type="button"
          class="inline-flex items-center gap-2 px-4 py-3 bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 hover:shadow-md hover:border-primary-300 dark:hover:border-primary-700 transition-all duration-200 text-left focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500"
          onclick={() => goto(`/dj/library/genres/${encodeURIComponent(entry.key)}`)}
        >
          <CoverArt src={entry.coverUrl || ''} alt={entry.key} size="sm" />
          <div>
            <p class="text-sm font-semibold text-gray-900 dark:text-white">{entry.key}</p>
            <p class="text-xs text-gray-500 dark:text-gray-400">{entry.count} track{entry.count !== 1 ? 's' : ''}</p>
          </div>
        </button>
      {/each}
    </div>
  {/if}
{/if}
