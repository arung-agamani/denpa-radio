<script lang="ts">
  import { onMount } from 'svelte';
  import { trackLibrary, tracksByAlbum } from '$lib/stores';
  import AlbumCard from '$lib/components/AlbumCard.svelte';
  import { goto } from '$app/navigation';

  let loaded = $state(false);

  onMount(async () => {
    if ($trackLibrary.length === 0) {
      await trackLibrary.refresh();
    }
    loaded = true;
  });

  let entries = $derived($tracksByAlbum);
  let searchQuery = $state('');

  let filtered = $derived(
    searchQuery.trim()
      ? entries.filter(
          e =>
            e.key.toLowerCase().includes(searchQuery.toLowerCase()) ||
            (e.artist || '').toLowerCase().includes(searchQuery.toLowerCase())
        )
      : entries
  );
</script>

{#if !loaded}
  <div class="flex items-center justify-center py-16">
    <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500"></div>
  </div>
{:else}
  <!-- Search -->
  <div class="mb-4">
    <input
      type="text"
      placeholder="Search albums..."
      bind:value={searchQuery}
      class="w-full sm:w-80 px-3 py-2 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
    />
  </div>

  {#if filtered.length === 0}
    <div class="flex flex-col items-center justify-center py-16 text-gray-400 dark:text-gray-500">
      <span class="text-4xl mb-3">💿</span>
      <p class="text-sm font-medium">No albums found</p>
      {#if searchQuery}
        <p class="text-xs mt-1">Try a different search term.</p>
      {:else}
        <p class="text-xs mt-1">Add some music to get started!</p>
      {/if}
    </div>
  {:else}
    <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
      {#each filtered as entry (entry.key)}
        <AlbumCard
          name={entry.key}
          artist={entry.artist}
          trackCount={entry.count}
          coverUrl={entry.coverUrl || ''}
          onclick={() => goto(`/dj/library/albums/${encodeURIComponent(entry.key)}`)}
        />
      {/each}
    </div>
  {/if}
{/if}
