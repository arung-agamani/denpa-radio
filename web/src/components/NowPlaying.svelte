<script>
  import { currentTrack, currentTrackInfo, activeTag, activePlaylist, status } from '../lib/stores.js';

  const tagEmoji = {
    morning: '🌅',
    afternoon: '☀️',
    evening: '🌇',
    night: '🌙',
  };

  const tagLabel = {
    morning: 'Morning',
    afternoon: 'Afternoon',
    evening: 'Evening',
    night: 'Night',
  };

  $: track = $currentTrackInfo;
  $: tag = $activeTag;
  $: playlist = $activePlaylist;
  $: trackName = $currentTrack;
  $: isIdle = !trackName || trackName === 'none';
  $: summary = $status.playlist_summary || {};
  $: totalTracks = $status.total_tracks || 0;
</script>

<div class="bg-white dark:bg-gray-800 rounded-2xl shadow-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
  <!-- Header gradient bar -->
  <div class="h-1.5 bg-gradient-to-r from-primary-400 via-purple-500 to-pink-500"></div>

  <div class="p-5">
    <!-- Track info section -->
    <div class="flex items-start gap-4">
      <!-- Album art placeholder / animated visualizer -->
      <div class="flex-shrink-0 w-20 h-20 rounded-xl bg-gradient-to-br from-primary-100 to-purple-100 dark:from-primary-900/50 dark:to-purple-900/50 flex items-center justify-center overflow-hidden">
        {#if isIdle}
          <span class="text-3xl opacity-50">📻</span>
        {:else}
          <div class="flex items-end gap-0.5 h-10">
            <div class="w-1.5 bg-primary-500 rounded-full animate-pulse" style="height: 60%; animation-delay: 0ms;"></div>
            <div class="w-1.5 bg-primary-400 rounded-full animate-pulse" style="height: 100%; animation-delay: 150ms;"></div>
            <div class="w-1.5 bg-purple-500 rounded-full animate-pulse" style="height: 40%; animation-delay: 300ms;"></div>
            <div class="w-1.5 bg-purple-400 rounded-full animate-pulse" style="height: 80%; animation-delay: 450ms;"></div>
            <div class="w-1.5 bg-pink-500 rounded-full animate-pulse" style="height: 55%; animation-delay: 600ms;"></div>
          </div>
        {/if}
      </div>

      <!-- Track details -->
      <div class="flex-1 min-w-0">
        <p class="text-xs font-semibold uppercase tracking-wider text-primary-500 dark:text-primary-400 mb-1">
          {#if isIdle}
            Nothing Playing
          {:else}
            Now Playing
          {/if}
        </p>

        {#if track}
          <h3 class="text-lg font-bold text-gray-900 dark:text-white truncate" title={track.title}>
            {track.title || trackName}
          </h3>

          {#if track.artist}
            <p class="text-sm text-gray-600 dark:text-gray-300 truncate mt-0.5" title={track.artist}>
              {track.artist}
            </p>
          {/if}

          {#if track.album}
            <p class="text-xs text-gray-500 dark:text-gray-400 truncate mt-0.5" title={track.album}>
              {track.album}
              {#if track.year}
                <span class="text-gray-400 dark:text-gray-500">({track.year})</span>
              {/if}
            </p>
          {/if}
        {:else if !isIdle}
          <h3 class="text-lg font-bold text-gray-900 dark:text-white truncate" title={trackName}>
            {trackName}
          </h3>
          <p class="text-sm text-gray-500 dark:text-gray-400 mt-0.5">
            No metadata available
          </p>
        {:else}
          <h3 class="text-lg font-bold text-gray-400 dark:text-gray-500">
            Waiting for stream…
          </h3>
          <p class="text-sm text-gray-400 dark:text-gray-500 mt-0.5">
            Press play to start listening
          </p>
        {/if}
      </div>
    </div>

    <!-- Divider -->
    <hr class="my-4 border-gray-200 dark:border-gray-700" />

    <!-- Active playlist & tag info -->
    <div class="flex flex-wrap items-center gap-3">
      <!-- Time tag badge -->
      {#if tag}
        <div class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-semibold bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300">
          <span>{tagEmoji[tag] || '🕐'}</span>
          <span>{tagLabel[tag] || tag}</span>
        </div>
      {/if}

      <!-- Active playlist name -->
      {#if playlist}
        <div class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-semibold bg-primary-50 dark:bg-primary-900/30 text-primary-700 dark:text-primary-300">
          <span>🎵</span>
          <span class="truncate max-w-[200px]">{playlist}</span>
        </div>
      {/if}

      <!-- Track count -->
      <div class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium bg-gray-50 dark:bg-gray-700/50 text-gray-500 dark:text-gray-400">
        <span>💿</span>
        <span>{totalTracks} track{totalTracks !== 1 ? 's' : ''}</span>
      </div>

      <!-- Additional track metadata badges -->
      {#if track}
        {#if track.genre}
          <div class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-purple-50 dark:bg-purple-900/30 text-purple-600 dark:text-purple-300">
            {track.genre}
          </div>
        {/if}

        {#if track.format}
          <div class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-blue-50 dark:bg-blue-900/30 text-blue-600 dark:text-blue-300 uppercase">
            {track.format}
          </div>
        {/if}

        {#if track.duration && track.duration > 0}
          <div class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-gray-50 dark:bg-gray-700/50 text-gray-500 dark:text-gray-400">
            {Math.floor(track.duration / 60)}:{(track.duration % 60).toString().padStart(2, '0')}
          </div>
        {/if}
      {/if}
    </div>

    <!-- Time slot overview (small icons showing which slots have playlists) -->
    {#if Object.keys(summary).length > 0}
      <div class="mt-4 flex items-center gap-2">
        <span class="text-xs text-gray-400 dark:text-gray-500 mr-1">Schedule:</span>
        {#each ['morning', 'afternoon', 'evening', 'night'] as slot}
          {@const count = summary[slot] || 0}
          <div
            class="flex items-center gap-1 px-2 py-0.5 rounded text-xs transition-colors
              {tag === slot
                ? 'bg-primary-100 dark:bg-primary-900/40 text-primary-700 dark:text-primary-300 font-semibold ring-1 ring-primary-300 dark:ring-primary-700'
                : count > 0
                  ? 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300'
                  : 'bg-gray-50 dark:bg-gray-800 text-gray-400 dark:text-gray-600'}"
            title="{tagLabel[slot]}: {count} playlist{count !== 1 ? 's' : ''}{tag === slot ? ' (active)' : ''}"
          >
            <span class="text-xs">{tagEmoji[slot]}</span>
            <span>{count}</span>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>
