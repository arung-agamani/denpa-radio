<script>
  import { createEventDispatcher } from 'svelte';

  /** @type {Array<{id: number, title: string, artist?: string, album?: string, genre?: string, format?: string, duration?: number, filePath: string, checksum: string}>} */
  export let tracks = [];

  /** Whether to show management actions (remove, move) */
  export let editable = false;

  /** Whether to show the index/position column */
  export let showIndex = true;

  /** Whether to show the file format badge */
  export let showFormat = true;

  /** Optional: highlight the track with this checksum */
  export let highlightChecksum = '';

  /** Optional: compact mode with smaller rows */
  export let compact = false;

  /** Optional: label for empty state */
  export let emptyMessage = 'No tracks in this playlist.';

  const dispatch = createEventDispatcher();

  function handleRemove(track, index) {
    dispatch('remove', { track, index });
  }

  function handleMoveUp(track, index) {
    if (index > 0) {
      dispatch('move', { from: index, to: index - 1, track });
    }
  }

  function handleMoveDown(track, index) {
    if (index < tracks.length - 1) {
      dispatch('move', { from: index, to: index + 1, track });
    }
  }

  function handleSelect(track, index) {
    dispatch('select', { track, index });
  }

  function formatDuration(seconds) {
    if (!seconds || seconds <= 0) return '';
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return `${m}:${s.toString().padStart(2, '0')}`;
  }

  function shortenPath(filePath) {
    if (!filePath) return '';
    const parts = filePath.replace(/\\/g, '/').split('/');
    if (parts.length <= 2) return filePath;
    return '…/' + parts.slice(-2).join('/');
  }
</script>

{#if tracks.length === 0}
  <div class="flex flex-col items-center justify-center py-12 text-gray-400 dark:text-gray-500">
    <svg class="w-12 h-12 mb-3 opacity-50" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" d="m9 9 10.5-3m0 6.553v3.75a2.25 2.25 0 0 1-1.632 2.163l-1.32.377a1.803 1.803 0 1 1-.99-3.467l2.31-.66a2.25 2.25 0 0 0 1.632-2.163Zm0 0V2.25L9 5.25v10.303m0 0v3.75a2.25 2.25 0 0 1-1.632 2.163l-1.32.377a1.803 1.803 0 0 1-.99-3.467l2.31-.66A2.25 2.25 0 0 0 9 15.553Z" />
    </svg>
    <p class="text-sm font-medium">{emptyMessage}</p>
  </div>
{:else}
  <div class="overflow-x-auto">
    <table class="w-full text-left">
      <thead>
        <tr class="border-b border-gray-200 dark:border-gray-700 text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">
          {#if editable}
            <th class="px-3 {compact ? 'py-2' : 'py-3'} w-28 text-center">Actions</th>
          {/if}
          {#if showIndex}
            <th class="px-3 {compact ? 'py-2' : 'py-3'} w-12 text-center">#</th>
          {/if}
          <th class="px-3 {compact ? 'py-2' : 'py-3'}">Title</th>
          <th class="px-3 {compact ? 'py-2' : 'py-3'} hidden sm:table-cell">Artist</th>
          <th class="px-3 {compact ? 'py-2' : 'py-3'} hidden md:table-cell">Album</th>
          {#if showFormat}
            <th class="px-3 {compact ? 'py-2' : 'py-3'} hidden lg:table-cell w-20 text-center">Format</th>
          {/if}
          <th class="px-3 {compact ? 'py-2' : 'py-3'} hidden lg:table-cell w-20 text-right">Duration</th>
        </tr>
      </thead>
      <tbody>
        {#each tracks as track, index (track.id || track.checksum || index)}
          {@const isHighlighted = highlightChecksum && track.checksum === highlightChecksum}
          <tr
            class="border-b border-gray-100 dark:border-gray-800 transition-colors
              {isHighlighted
                ? 'bg-primary-50 dark:bg-primary-900/20 border-l-2 border-l-primary-500'
                : 'hover:bg-gray-50 dark:hover:bg-gray-800/50'}"
            on:click={() => handleSelect(track, index)}
            role="button"
            tabindex="0"
            on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); handleSelect(track, index); } }}
          >
            {#if editable}
              <td class="px-3 {compact ? 'py-1.5' : 'py-3'} text-center">
                <div class="flex items-center justify-center gap-1">
                  <!-- Move up -->
                  <button
                    type="button"
                    class="p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 transition-colors disabled:opacity-30 disabled:cursor-not-allowed"
                    title="Move up"
                    disabled={index === 0}
                    on:click|stopPropagation={() => handleMoveUp(track, index)}
                  >
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 15.75l7.5-7.5 7.5 7.5" />
                    </svg>
                  </button>

                  <!-- Move down -->
                  <button
                    type="button"
                    class="p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 transition-colors disabled:opacity-30 disabled:cursor-not-allowed"
                    title="Move down"
                    disabled={index === tracks.length - 1}
                    on:click|stopPropagation={() => handleMoveDown(track, index)}
                  >
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
                    </svg>
                  </button>

                  <!-- Remove -->
                  <button
                    type="button"
                    class="p-1 rounded hover:bg-red-100 dark:hover:bg-red-900/40 text-gray-400 hover:text-red-600 dark:hover:text-red-400 transition-colors"
                    title="Remove from playlist"
                    on:click|stopPropagation={() => handleRemove(track, index)}
                  >
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  </button>
                </div>
              </td>
            {/if}
            {#if showIndex}
              <td class="px-3 {compact ? 'py-1.5' : 'py-3'} text-center">
                <span class="text-xs font-mono text-gray-400 dark:text-gray-500 {isHighlighted ? 'text-primary-500 dark:text-primary-400 font-semibold' : ''}">
                  {#if isHighlighted}
                    <span class="inline-block animate-pulse">▶</span>
                  {:else}
                    {index + 1}
                  {/if}
                </span>
              </td>
            {/if}

            <td class="px-3 {compact ? 'py-1.5' : 'py-3'}">
              <div class="min-w-0">
                <p class="text-sm font-medium text-gray-900 dark:text-white truncate {isHighlighted ? 'text-primary-700 dark:text-primary-300' : ''}"
                   title={track.title}>
                  {track.title || 'Untitled'}
                </p>
                <!-- Show artist on mobile beneath title since the Artist column is hidden -->
                {#if track.artist}
                  <p class="text-xs text-gray-500 dark:text-gray-400 truncate sm:hidden mt-0.5">
                    {track.artist}
                  </p>
                {/if}
                <!-- Show file path hint on hover/compact -->
                {#if compact}
                  <p class="text-xs text-gray-400 dark:text-gray-600 truncate mt-0.5" title={track.filePath}>
                    {shortenPath(track.filePath)}
                  </p>
                {/if}
              </div>
            </td>

            <td class="px-3 {compact ? 'py-1.5' : 'py-3'} hidden sm:table-cell">
              <span class="text-sm text-gray-600 dark:text-gray-300 truncate block max-w-[200px]" title={track.artist || ''}>
                {track.artist || '—'}
              </span>
            </td>

            <td class="px-3 {compact ? 'py-1.5' : 'py-3'} hidden md:table-cell">
              <span class="text-sm text-gray-500 dark:text-gray-400 truncate block max-w-[200px]" title={track.album || ''}>
                {track.album || '—'}
              </span>
            </td>

            {#if showFormat}
              <td class="px-3 {compact ? 'py-1.5' : 'py-3'} hidden lg:table-cell text-center">
                {#if track.format}
                  <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium uppercase
                    {track.format === 'flac'
                      ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
                      : track.format === 'mp3'
                        ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300'
                        : 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'}">
                    {track.format}
                  </span>
                {/if}
              </td>
            {/if}

            <td class="px-3 {compact ? 'py-1.5' : 'py-3'} hidden lg:table-cell text-right">
              <span class="text-xs text-gray-500 dark:text-gray-400 font-mono tabular-nums">
                {formatDuration(track.duration)}
              </span>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>

  <!-- Summary footer -->
  <div class="px-3 py-2 flex items-center justify-between text-xs text-gray-400 dark:text-gray-500 border-t border-gray-100 dark:border-gray-800">
    <span>{tracks.length} track{tracks.length !== 1 ? 's' : ''}</span>
    {#if tracks.some(t => t.duration && t.duration > 0)}
      {@const totalSeconds = tracks.reduce((sum, t) => sum + (t.duration || 0), 0)}
      {@const hours = Math.floor(totalSeconds / 3600)}
      {@const minutes = Math.floor((totalSeconds % 3600) / 60)}
      <span>
        {#if hours > 0}
          {hours}h {minutes}m
        {:else}
          {minutes} min
        {/if}
        total
      </span>
    {/if}
  </div>
{/if}
