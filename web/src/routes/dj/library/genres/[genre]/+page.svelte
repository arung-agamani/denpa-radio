<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { trackLibrary, tracksByGenre } from '$lib/stores';
  import InlineTrackDetail from '$lib/components/InlineTrackDetail.svelte';
  import CoverArt from '$lib/components/CoverArt.svelte';

  let loaded = $state(false);

  let genreName = $derived(decodeURIComponent($page.params.genre || ''));

  let genreEntries = $derived($tracksByGenre);
  let currentGenre = $derived(genreEntries.find(e => e.key === genreName));
  let sortedTracks = $derived(
    currentGenre
      ? [...currentGenre.tracks].sort((a, b) => {
          // Sort by artist, then album, then track number.
          const artistCmp = (a.artist || '').localeCompare(b.artist || '');
          if (artistCmp !== 0) return artistCmp;
          const albumCmp = (a.album || '').localeCompare(b.album || '');
          if (albumCmp !== 0) return albumCmp;
          return (a.trackNum || 999) - (b.trackNum || 999);
        })
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
{:else if !currentGenre}
  <div class="flex flex-col items-center justify-center py-16">
    <p class="text-gray-400 dark:text-gray-500">Genre not found.</p>
    <button type="button" class="mt-4 text-sm text-primary-600 dark:text-primary-400 hover:underline" onclick={() => goto('/dj/library/genres')}>Back to genres</button>
  </div>
{:else}
  <!-- Header -->
  <div class="flex items-start gap-5 mb-6">
    <CoverArt src={currentGenre.coverUrl || ''} alt={genreName} size="lg" />
    <div class="flex-1 min-w-0 pt-1">
      <h2 class="text-xl font-bold text-gray-900 dark:text-white truncate">{genreName}</h2>
      <p class="text-xs text-gray-400 dark:text-gray-500 mt-1">{currentGenre.count} track{currentGenre.count !== 1 ? 's' : ''}</p>
    </div>
  </div>

  <!-- Track list -->
  <div class="space-y-1">
    {#each sortedTracks as track (track.id)}
      <InlineTrackDetail {track} />
    {/each}
  </div>
{/if}
