<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { trackLibrary, tracksByAlbum } from '$lib/stores';
  import InlineTrackDetail from '$lib/components/InlineTrackDetail.svelte';
  import CoverArt from '$lib/components/CoverArt.svelte';

  let loaded = $state(false);

  let albumName = $derived(decodeURIComponent($page.params.album || ''));

  let entries = $derived($tracksByAlbum);
  let currentAlbum = $derived(entries.find(e => e.key === albumName));
  let sortedTracks = $derived(
    currentAlbum
      ? [...currentAlbum.tracks].sort((a, b) => (a.trackNum || 999) - (b.trackNum || 999))
      : []
  );

  onMount(async () => {
    if ($trackLibrary.length === 0) {
      await trackLibrary.refresh();
    }
    loaded = true;
  });
</script>

{#if !loaded}
  <div class="flex items-center justify-center py-16">
    <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500"></div>
  </div>
{:else if !currentAlbum}
  <div class="flex flex-col items-center justify-center py-16">
    <p class="text-gray-400 dark:text-gray-500">Album not found.</p>
    <button type="button" class="mt-4 text-sm text-primary-600 dark:text-primary-400 hover:underline" onclick={() => goto('/dj/library/albums')}>Back to albums</button>
  </div>
{:else}
  <!-- Header -->
  <div class="flex items-start gap-5 mb-6">
    <CoverArt src={currentAlbum.coverUrl || ''} alt={albumName} size="lg" />
    <div class="flex-1 min-w-0 pt-1">
      <h2 class="text-xl font-bold text-gray-900 dark:text-white truncate">{albumName}</h2>
      {#if currentAlbum.artist}
        <p class="text-sm text-gray-500 dark:text-gray-400">{currentAlbum.artist}</p>
      {/if}
      <p class="text-xs text-gray-400 dark:text-gray-500 mt-1">{sortedTracks.length} track{sortedTracks.length !== 1 ? 's' : ''}</p>
    </div>
  </div>

  <!-- Track list -->
  <div class="space-y-1">
    {#each sortedTracks as track (track.id)}
      <InlineTrackDetail {track} />
    {/each}
  </div>
{/if}
