<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { trackLibrary, tracksByAlbum, toasts } from '$lib/stores';
  import InlineTrackDetail from '$lib/components/InlineTrackDetail.svelte';
  import CoverArt from '$lib/components/CoverArt.svelte';
  import { batchUpdateTracks, batchUpdateCover } from '$lib/api';

  let loaded = $state(false);
  let editing = $state(false);
  let saving = $state(false);
  let coverFile: File | null = $state(null);
  let coverPreview: string | null = $state(null);

  let albumName = $derived(decodeURIComponent($page.params.album || ''));

  let entries = $derived($tracksByAlbum);
  let currentAlbum = $derived(entries.find(e => e.key === albumName));
  let sortedTracks = $derived(
    currentAlbum
      ? [...currentAlbum.tracks].sort((a, b) => (a.trackNum || 999) - (b.trackNum || 999))
      : []
  );

  let editAlbum = $state('');
  let editArtist = $state('');
  let editGenre = $state('');
  let editYear: number | null = $state(null);

  function startEdit() {
    editAlbum = currentAlbum?.key || albumName;
    editArtist = currentAlbum?.artist || '';
    editGenre = currentAlbum?.genre || '';
    editYear = currentAlbum?.year ?? null;
    coverFile = null;
    coverPreview = null;
    editing = true;
  }

  function cancelEdit() {
    editing = false;
    coverFile = null;
    coverPreview = null;
  }

  function onCoverChange(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0] || null;
    coverFile = file;
    if (file) {
      coverPreview = URL.createObjectURL(file);
    } else {
      coverPreview = null;
    }
  }

  async function saveEdit() {
    if (!currentAlbum) return;
    saving = true;

    try {
      const originalArtist = currentAlbum.artist || '';
      const newArtist = editArtist.trim();
      const newAlbum = editAlbum.trim();
      const newGenre = editGenre.trim();
      const newYear = editYear;

      const filter = {
        album: albumName,
        artist: originalArtist || undefined,
      };

      const updates: Record<string, string | number | undefined> = {};
      if (newAlbum !== albumName) updates.album = newAlbum;
      if (newArtist !== originalArtist) updates.artist = newArtist;
      if (newGenre !== (currentAlbum.genre || '')) updates.genre = newGenre;
      if (newYear !== (currentAlbum.year ?? null)) updates.year = newYear ?? undefined;

      if (Object.keys(updates).length > 0) {
        const result = await batchUpdateTracks({ filter, updates });
        toasts.success(`Updated ${result.updated} track(s).`);
      }

      if (coverFile) {
        const coverFilter = {
          album: newAlbum || albumName,
          artist: newArtist || originalArtist || undefined,
        };
        const coverResult = await batchUpdateCover(coverFilter, coverFile);
        toasts.success(`Cover art applied to ${coverResult.updated} track(s).`);
      }

      await trackLibrary.refresh();
      editing = false;

      if (newAlbum !== albumName) {
        goto(`/dj/library/albums/${encodeURIComponent(newAlbum)}`, { replaceState: true });
      }
    } catch (err) {
      toasts.error('Failed to update album: ' + (err instanceof Error ? err.message : String(err)));
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
{:else if !currentAlbum}
  <div class="flex flex-col items-center justify-center py-16">
    <p class="text-gray-400 dark:text-gray-500">Album not found.</p>
    <button type="button" class="mt-4 text-sm text-primary-600 dark:text-primary-400 hover:underline" onclick={() => goto('/dj/library/albums')}>Back to albums</button>
  </div>
{:else}
  <div class="flex items-start gap-5 mb-6">
    <CoverArt src={coverPreview || currentAlbum.coverUrl || ''} alt={albumName} size="lg" />
    <div class="flex-1 min-w-0 pt-1">
      <h2 class="text-xl font-bold text-gray-900 dark:text-white truncate">{albumName}</h2>
      {#if currentAlbum.artist}
        <p class="text-sm text-gray-500 dark:text-gray-400">{currentAlbum.artist}</p>
      {/if}
      <p class="text-xs text-gray-400 dark:text-gray-500 mt-1">{sortedTracks.length} track{sortedTracks.length !== 1 ? 's' : ''}</p>
    </div>
    <button
      type="button"
      class="px-3 py-1.5 text-xs font-semibold rounded-lg bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300 hover:bg-primary-200 dark:hover:bg-primary-800/60 transition-colors"
      onclick={startEdit}
    >
      Edit Album
    </button>
  </div>

  {#if editing}
    <div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5 mb-6">
      <h3 class="text-base font-semibold text-gray-900 dark:text-white mb-4">Edit Album</h3>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-4">
        <div>
          <label for="album-name" class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Album Name</label>
          <input id="album-name" type="text" bind:value={editAlbum} class="w-full px-3 py-1.5 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-primary-500" />
        </div>
        <div>
          <label for="album-artist" class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Artist</label>
          <input id="album-artist" type="text" bind:value={editArtist} class="w-full px-3 py-1.5 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-primary-500" />
        </div>
        <div>
          <label for="album-genre" class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Genre</label>
          <input id="album-genre" type="text" bind:value={editGenre} class="w-full px-3 py-1.5 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-primary-500" />
        </div>
        <div>
          <label for="album-year" class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Year</label>
          <input id="album-year" type="number" bind:value={editYear} class="w-full px-3 py-1.5 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-primary-500" />
        </div>
      </div>
      <div class="mb-4">
        <label for="album-cover" class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Cover Art</label>
        <input id="album-cover" type="file" accept="image/*" onchange={onCoverChange} class="block w-full text-sm text-gray-600 dark:text-gray-300 file:mr-3 file:py-1.5 file:px-3 file:rounded-lg file:border-0 file:text-xs file:font-medium file:bg-primary-100 file:text-primary-700 dark:file:bg-primary-900/40 dark:file:text-primary-300 hover:file:bg-primary-200 dark:hover:file:bg-primary-800/60" />
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
