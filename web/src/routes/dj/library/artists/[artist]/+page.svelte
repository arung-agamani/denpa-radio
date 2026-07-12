<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { trackLibrary, tracksByArtist, toasts } from '$lib/stores';
  import InlineTrackDetail from '$lib/components/InlineTrackDetail.svelte';
  import CoverArt from '$lib/components/CoverArt.svelte';
  import AlbumCard from '$lib/components/AlbumCard.svelte';
  import type { Track } from '$lib/api';
  import { batchUpdateTracks } from '$lib/api';

  let loaded = $state(false);
  let editing = $state(false);
  let saving = $state(false);
  let editName = $state('');

  let artistName = $derived(decodeURIComponent($page.params.artist || ''));

  let artistEntries = $derived($tracksByArtist);
  let currentArtist = $derived(artistEntries.find(e => e.key === artistName));

  // Group this artist's tracks by album.
  let albumGroups = $derived(
    currentArtist
      ? [...currentArtist.tracks].reduce<Map<string, Track[]>>((acc, t) => {
          const key = t.album || 'Unknown Album';
          if (!acc.has(key)) acc.set(key, []);
          acc.get(key)!.push(t);
          return acc;
        }, new Map())
      : new Map<string, Track[]>()
  );

  let albumList = $derived(
    Array.from(albumGroups.entries())
      .map(([album, tracks]: [string, Track[]]) => ({ album, tracks, count: tracks.length, coverUrl: tracks.find((t: Track) => t.coverUrl)?.coverUrl }))
      .sort((a, b) => a.album.localeCompare(b.album))
  );

  function startEdit() {
    editName = artistName;
    editing = true;
  }

  function cancelEdit() {
    editing = false;
  }

  async function saveEdit() {
    const newName = editName.trim();
    if (!newName || newName === artistName || !currentArtist) {
      editing = false;
      return;
    }
    saving = true;
    try {
      await batchUpdateTracks({
        filter: { artist: artistName },
        updates: { artist: newName },
      });
      toasts.success(`Artist renamed to "${newName}".`);
      await trackLibrary.refresh();
      editing = false;
      goto(`/dj/library/artists/${encodeURIComponent(newName)}`, { replaceState: true });
    } catch (err) {
      toasts.error('Failed to rename artist: ' + (err instanceof Error ? err.message : String(err)));
    } finally {
      saving = false;
    }
  }

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
{:else if !currentArtist}
  <div class="flex flex-col items-center justify-center py-16">
    <p class="text-gray-400 dark:text-gray-500">Artist not found.</p>
    <button type="button" class="mt-4 text-sm text-primary-600 dark:text-primary-400 hover:underline" onclick={() => goto('/dj/library/artists')}>Back to artists</button>
  </div>
{:else}
  <div class="flex items-start gap-5 mb-6">
    <CoverArt src={currentArtist.coverUrl || ''} alt={artistName} size="lg" />
    <div class="flex-1 min-w-0 pt-1">
      <h2 class="text-xl font-bold text-gray-900 dark:text-white truncate">{artistName}</h2>
      <p class="text-xs text-gray-400 dark:text-gray-500 mt-1">{currentArtist.count} track{currentArtist.count !== 1 ? 's' : ''} across {albumList.length} album{albumList.length !== 1 ? 's' : ''}</p>
    </div>
    <button
      type="button"
      class="px-3 py-1.5 text-xs font-semibold rounded-lg bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300 hover:bg-primary-200 dark:hover:bg-primary-800/60 transition-colors"
      onclick={startEdit}
    >
      Edit Artist
    </button>
  </div>

  {#if editing}
    <div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5 mb-6">
      <h3 class="text-base font-semibold text-gray-900 dark:text-white mb-4">Edit Artist</h3>
      <div class="mb-4">
        <label for="artist-name" class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Artist Name</label>
        <input id="artist-name" type="text" bind:value={editName} class="w-full sm:w-80 px-3 py-1.5 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-primary-500" />
      </div>
      <div class="flex items-center gap-2">
        <button type="button" class="px-4 py-1.5 text-sm font-semibold text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors disabled:opacity-50" onclick={saveEdit} disabled={saving}>
          {saving ? 'Saving…' : 'Save'}
        </button>
        <button type="button" class="px-4 py-1.5 text-sm font-medium text-gray-600 dark:text-gray-300 hover:text-gray-800 dark:hover:text-white transition-colors" onclick={cancelEdit} disabled={saving}>
          Cancel
        </button>
      </div>
    </div>
  {/if}

  <!-- Albums by this artist -->
  <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-4 mb-8">
    {#each albumList as entry (entry.album)}
      <AlbumCard
        name={entry.album}
        artist={artistName}
        trackCount={entry.count}
        coverUrl={entry.coverUrl || ''}
        onclick={() => goto(`/dj/library/albums/${encodeURIComponent(entry.album)}`)}
      />
    {/each}
  </div>

  <!-- All tracks by this artist -->
  <h3 class="text-base font-semibold text-gray-900 dark:text-white mb-3">All Tracks</h3>
  <div class="space-y-1">
    {#each currentArtist.tracks as track (track.id)}
      <InlineTrackDetail {track} />
    {/each}
  </div>
{/if}
