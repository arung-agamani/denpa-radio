<script lang="ts">
  import type { Track } from '$lib/api';
  import { updateTrack, enrichTrack, deleteTrack } from '$lib/api';
  import { trackLibrary, toasts, currentTrackInfo } from '$lib/stores';
  import CoverArt from './CoverArt.svelte';

  let { track, expanded = false }: { track: Track; expanded?: boolean } = $props();

  let isExpanded = $state(expanded);
  let enriching = $state(false);
  let editing = $state(false);

  // Editable copies
  let editTitle = $state(track.title);
  let editArtist = $state(track.artist);
  let editAlbum = $state(track.album);
  let editGenre = $state(track.genre);
  let editYear: number | null = $state(track.year ?? null);
  let editTrackNum: number | null = $state(track.trackNum ?? null);

  let coverEl: string = $derived(track.coverUrl || '');

  async function handleEnrich() {
    enriching = true;
    try {
      const result = await enrichTrack(track.id);
      if (result.artFetched) {
        toasts.success(`Cover art found for "${track.album || track.title}"`);
        await trackLibrary.refresh();
      } else if (result.year || (result.genres && result.genres.length > 0)) {
        toasts.success(`Metadata updated for "${track.album || track.title}"`);
        await trackLibrary.refresh();
      } else {
        toasts.info('No additional metadata found from external sources.');
      }
    } catch (err) {
      toasts.error('Enrichment failed: ' + (err instanceof Error ? err.message : String(err)));
    } finally {
      enriching = false;
    }
  }

  async function handleDelete() {
    if (!confirm(`Remove "${track.title || 'this track'}" from the library and all playlists?`)) {
      return;
    }
    try {
      await deleteTrack(track.id);
      toasts.success('Track removed from library and all playlists.');
      await trackLibrary.refresh();
    } catch (err) {
      toasts.error('Failed to delete track: ' + (err instanceof Error ? err.message : String(err)));
    }
  }

  function startEdit() {
    editTitle = track.title;
    editArtist = track.artist;
    editAlbum = track.album;
    editGenre = track.genre;
    editYear = track.year ?? null;
    editTrackNum = track.trackNum ?? null;
    editing = true;
  }

  function cancelEdit() {
    editing = false;
  }

  async function saveEdit() {
    try {
      const updates: Partial<Track> = {};
      if (editTitle !== track.title) updates.title = editTitle;
      if (editArtist !== track.artist) updates.artist = editArtist;
      if (editAlbum !== track.album) updates.album = editAlbum;
      if (editGenre !== track.genre) updates.genre = editGenre;
      if (editYear !== track.year) updates.year = editYear ?? undefined;
      if (editTrackNum !== track.trackNum) updates.trackNum = editTrackNum ?? undefined;

      if (Object.keys(updates).length === 0) {
        editing = false;
        return;
      }

      await updateTrack(track.id, updates);
      toasts.success('Track metadata updated!');
      editing = false;
      await trackLibrary.refresh();
    } catch (err) {
      toasts.error('Failed to update track: ' + (err instanceof Error ? err.message : String(err)));
    }
  }

  function formatDuration(seconds: number): string {
    if (!seconds || seconds <= 0) return '';
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return `${m}:${s.toString().padStart(2, '0')}`;
  }
</script>

<div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden transition-all duration-200">
  <!-- Collapsed row -->
  <button
    type="button"
    class="w-full flex items-center gap-3 px-4 py-3 text-left hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500"
    onclick={() => (isExpanded = !isExpanded)}
  >
    <!-- Playing indicator -->
    <div class="flex-shrink-0 w-8 text-center text-xs font-mono text-gray-400 dark:text-gray-500">
      {#if $currentTrackInfo?.checksum && $currentTrackInfo.checksum === track.checksum}
        <span class="text-primary-500">▶</span>
      {:else}
        {track.trackNum || '?'}
      {/if}
    </div>

    <div class="flex-1 min-w-0">
      <p class="text-sm font-medium text-gray-900 dark:text-white truncate">{track.title || 'Untitled'}</p>
      {#if track.artist}
        <p class="text-xs text-gray-500 dark:text-gray-400 truncate">{track.artist}</p>
      {/if}
    </div>

    {#if track.format}
      <span class="flex-shrink-0 px-2 py-0.5 rounded text-xs font-medium uppercase {track.format === 'flac' ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300' : track.format === 'mp3' ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300' : 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'}">{track.format}</span>
    {/if}

    <span class="flex-shrink-0 text-xs text-gray-400 dark:text-gray-500 font-mono tabular-nums">{formatDuration(track.duration)}</span>

    <svg class="flex-shrink-0 w-4 h-4 text-gray-400 transition-transform duration-200 {isExpanded ? 'rotate-90' : ''}" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 4.5l7.5 7.5-7.5 7.5" />
    </svg>
  </button>

  <!-- Expanded detail panel -->
  {#if isExpanded}
    <div class="border-t border-gray-100 dark:border-gray-700">
      <div class="p-4 sm:p-5">
        <div class="flex flex-col sm:flex-row gap-5">
          <!-- Cover art -->
          <div class="flex-shrink-0 w-full sm:w-36">
            <CoverArt src={coverEl} alt={track.album || track.title} size="full" class="w-full sm:w-36" />
          </div>

          <!-- Metadata -->
          <div class="flex-1 min-w-0 space-y-4">
            {#if editing}
              <!-- Editable fields -->
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Title</label>
                  <input type="text" bind:value={editTitle} class="w-full px-3 py-1.5 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-primary-500" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Artist</label>
                  <input type="text" bind:value={editArtist} class="w-full px-3 py-1.5 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-primary-500" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Album</label>
                  <input type="text" bind:value={editAlbum} class="w-full px-3 py-1.5 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-primary-500" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Genre</label>
                  <input type="text" bind:value={editGenre} class="w-full px-3 py-1.5 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-primary-500" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Year</label>
                  <input type="number" bind:value={editYear} class="w-full px-3 py-1.5 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-primary-500" />
                </div>
                <div>
                  <label class="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Track #</label>
                  <input type="number" bind:value={editTrackNum} class="w-full px-3 py-1.5 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-primary-500" />
                </div>
              </div>
              <div class="flex items-center gap-2">
                <button type="button" class="px-4 py-1.5 text-sm font-semibold text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors" onclick={saveEdit}>Save</button>
                <button type="button" class="px-4 py-1.5 text-sm font-medium text-gray-600 dark:text-gray-300 hover:text-gray-800 dark:hover:text-white transition-colors" onclick={cancelEdit}>Cancel</button>
              </div>
            {:else}
              <!-- Read-only metadata -->
              <div class="grid grid-cols-2 sm:grid-cols-3 gap-x-4 gap-y-2 text-sm">
                <div>
                  <span class="text-xs font-medium text-gray-400 dark:text-gray-500 block">Title</span>
                  <span class="text-gray-900 dark:text-white">{track.title || '—'}</span>
                </div>
                <div>
                  <span class="text-xs font-medium text-gray-400 dark:text-gray-500 block">Artist</span>
                  <span class="text-gray-900 dark:text-white">{track.artist || '—'}</span>
                </div>
                <div>
                  <span class="text-xs font-medium text-gray-400 dark:text-gray-500 block">Album</span>
                  <span class="text-gray-900 dark:text-white">{track.album || '—'}</span>
                </div>
                <div>
                  <span class="text-xs font-medium text-gray-400 dark:text-gray-500 block">Genre</span>
                  <span class="text-gray-900 dark:text-white">{track.genre || '—'}</span>
                </div>
                <div>
                  <span class="text-xs font-medium text-gray-400 dark:text-gray-500 block">Year</span>
                  <span class="text-gray-900 dark:text-white">{track.year || '—'}</span>
                </div>
                <div>
                  <span class="text-xs font-medium text-gray-400 dark:text-gray-500 block">Duration</span>
                  <span class="text-gray-900 dark:text-white font-mono">{formatDuration(track.duration)}</span>
                </div>
              </div>

              <!-- Actions -->
              <div class="flex flex-wrap items-center gap-2 pt-1">
                <button
                  type="button"
                  class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-lg bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300 hover:bg-primary-200 dark:hover:bg-primary-800/60 transition-colors disabled:opacity-50"
                  onclick={startEdit}
                >
                  <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0 1 15.75 21H5.25A2.25 2.25 0 0 1 3 18.75V8.25A2.25 2.25 0 0 1 5.25 6H10" /></svg>
                  Edit
                </button>
                <button
                  type="button"
                  class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-lg border border-gray-300 dark:border-gray-600 text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors disabled:opacity-50"
                  onclick={handleEnrich}
                  disabled={enriching}
                >
                  {#if enriching}
                    <svg class="animate-spin w-3.5 h-3.5" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" /><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" /></svg>
                    Enriching…
                  {:else}
                    <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M9.813 15.904 9 18.75l-.813-2.846a4.5 4.5 0 0 0-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 0 0 3.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 0 0 3.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 0 0-3.09 3.09ZM18.259 8.715 18 9.75l-.259-1.035a3.375 3.375 0 0 0-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 0 0 2.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 0 0 2.455 2.456L21.75 6l-1.036.259a3.375 3.375 0 0 0-2.455 2.456Z" /></svg>
                    Enrich
                  {/if}
                </button>

                <button
                  type="button"
                  class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-lg border border-red-200 dark:border-red-900 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/30 transition-colors"
                  onclick={handleDelete}
                >
                  <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" /></svg>
                  Delete
                </button>

                {#if track.coverUrl}
                  <span class="text-xs text-green-600 dark:text-green-400 flex items-center gap-1">
                    <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5" /></svg>
                    Cover art available
                  </span>
                {/if}
              </div>
            {/if}
          </div>
        </div>
      </div>
    </div>
  {/if}
</div>
