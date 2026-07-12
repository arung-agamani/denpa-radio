<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { trackLibrary, tracksByGenre, toasts } from '$lib/stores';
  import InlineTrackDetail from '$lib/components/InlineTrackDetail.svelte';
  import CoverArt from '$lib/components/CoverArt.svelte';
  import { batchUpdateTracks } from '$lib/api';

  let loaded = $state(false);
  let editing = $state(false);
  let saving = $state(false);
  let editName = $state('');

  let genreName = $derived(decodeURIComponent($page.params.genre || ''));

  let genreEntries = $derived($tracksByGenre);
  let currentGenre = $derived(genreEntries.find(e => e.key === genreName));
  let sortedTracks = $derived(
    currentGenre
      ? [...currentGenre.tracks].sort((a, b) => {
          const artistCmp = (a.artist || '').localeCompare(b.artist || '');
          if (artistCmp !== 0) return artistCmp;
          const albumCmp = (a.album || '').localeCompare(b.album || '');
          if (albumCmp !== 0) return albumCmp;
          return (a.trackNum || 999) - (b.trackNum || 999);
        })
      : []
  );

  function startEdit() {
    editName = genreName;
    editing = true;
  }

  function cancelEdit() {
    editing = false;
  }

  async function saveEdit() {
    const newName = editName.trim();
    if (!newName || newName === genreName || !currentGenre) {
      editing = false;
      return;
    }
    saving = true;
    try {
      await batchUpdateTracks({
        filter: { genre: genreName },
        updates: { genre: newName },
      });
      toasts.success(`Genre renamed to "${newName}".`);
      await trackLibrary.refresh();
      editing = false;
      goto(`/dj/library/genres/${encodeURIComponent(newName)}`, { replaceState: true });
    } catch (err) {
      toasts.error('Failed to rename genre: ' + (err instanceof Error ? err.message : String(err)));
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
{:else if !currentGenre}
  <div class="flex flex-col items-center justify-center py-16">
    <p class="text-gray-400 dark:text-gray-500">Genre not found.</p>
    <button type="button" class="mt-4 text-sm text-primary-600 dark:text-primary-400 hover:underline" onclick={() => goto('/dj/library/genres')}>Back to genres</button>
  </div>
{:else}
  <div class="flex items-start gap-5 mb-6">
    <CoverArt src={currentGenre.coverUrl || ''} alt={genreName} size="lg" />
    <div class="flex-1 min-w-0 pt-1">
      <h2 class="text-xl font-bold text-gray-900 dark:text-white truncate">{genreName}</h2>
      <p class="text-xs text-gray-400 dark:text-gray-500 mt-1">{currentGenre.count} track{currentGenre.count !== 1 ? 's' : ''}</p>
    </div>
    <button
      type="button"
      class="px-3 py-1.5 text-xs font-semibold rounded-lg bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300 hover:bg-primary-200 dark:hover:bg-primary-800/60 transition-colors"
      onclick={startEdit}
    >
      Edit Genre
    </button>
  </div>

  {#if editing}
    <div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5 mb-6">
      <h3 class="text-base font-semibold text-gray-900 dark:text-white mb-4">Edit Genre</h3>
      <div class="mb-4">
        <label for="genre-name" class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Genre Name</label>
        <input id="genre-name" type="text" bind:value={editName} class="w-full sm:w-80 px-3 py-1.5 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-primary-500" />
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

  <div class="space-y-1">
    {#each sortedTracks as track (track.id)}
      <InlineTrackDetail {track} />
    {/each}
  </div>
{/if}
