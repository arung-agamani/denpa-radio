<script lang="ts">
  import type { TrackItem } from '$lib/api';
  import { onMount } from 'svelte';
  import Player from '$lib/components/Player.svelte';
  import NowPlaying from '$lib/components/NowPlaying.svelte';
  import ChannelSelector from '$lib/components/ChannelSelector.svelte';
  import TrackList from '$lib/components/TrackList.svelte';
  import { status, stationName, currentTrackInfo, channels, selectedChannel } from '$lib/stores';
  import { getQueue } from '$lib/api';

  let tracks: TrackItem[] = $state([]);
  let loading = $state(true);
  let error: string | null = $state(null);
  let showFullQueue = $state(false);

  onMount(async () => {
    await channels.refresh();
    await loadQueue();
  });

  async function loadQueue() {
    loading = true;
    error = null;
    try {
      const data = await getQueue($selectedChannel);
      tracks = data.tracks || [];
    } catch (err) {
      console.error('Failed to load queue:', err);
      error = err instanceof Error ? err.message : 'Failed to load queue';
    } finally {
      loading = false;
    }
  }

  // Reload queue whenever the current track changes.
  let prevChecksum = '';
  let currentChecksum = $derived($currentTrackInfo?.checksum || '');
  $effect(() => {
    if (currentChecksum !== prevChecksum) {
      prevChecksum = currentChecksum;
      if (prevChecksum !== '') {
        loadQueue();
      }
    }
  });

  // Also reload when the active playlist's track count changes (add/remove).
  let prevTotalTracks = -1;
  $effect(() => {
    if ($status.total_tracks !== prevTotalTracks && prevTotalTracks !== -1) {
      prevTotalTracks = $status.total_tracks;
      loadQueue();
    } else if (prevTotalTracks === -1) {
      prevTotalTracks = $status.total_tracks || 0;
    }
  });

  let displayTracks = $derived(showFullQueue ? tracks : tracks.slice(0, 25));
  let hasMore = $derived(tracks.length > 25 && !showFullQueue);

  let selectedChannelName = $derived(
    $channels.find((c) => c.slug === $selectedChannel)?.name || $selectedChannel
  );

  let prevChannel = '';
  $effect(() => {
    const ch = $selectedChannel;
    if (ch !== prevChannel) {
      prevChannel = ch;
      if (prevChannel !== '') {
        loadQueue();
        status.refresh();
      }
    }
  });
</script>

<div class="px-4 sm:px-6 lg:px-8 py-8 space-y-6">
  <!-- Hero section -->
  <div class="text-center mb-2">
    <h1 class="text-3xl sm:text-4xl font-extrabold text-gray-900 dark:text-white">
      {$stationName}
    </h1>
    <p class="mt-2 text-gray-500 dark:text-gray-400 text-sm sm:text-base">
      Tune in and enjoy the music 🎶
    </p>
    {#if $channels.length > 1}
      <div class="mt-1.5">
        <span class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium bg-primary-50 dark:bg-primary-900/30 text-primary-700 dark:text-primary-300">
          <span>📡</span>
          <span>{selectedChannelName}</span>
        </span>
      </div>
    {/if}
  </div>

  <!-- Player -->
  <Player />

  <!-- Channel selector -->
  <ChannelSelector />

  <!-- Now Playing -->
  <NowPlaying />

  <!-- Playlist section -->
  <section>
    <div class="flex items-center justify-between mb-4">
      <div class="flex items-center gap-3">
        <h2 class="text-lg font-bold text-gray-900 dark:text-white">Up Next</h2>
        {#if !loading}
          <span class="text-xs text-gray-400 dark:text-gray-500 bg-gray-100 dark:bg-gray-800 px-2.5 py-1 rounded-full font-medium">
            {tracks.length} track{tracks.length !== 1 ? 's' : ''}
          </span>
        {/if}
      </div>

      <button
        type="button"
        class="text-sm text-primary-600 dark:text-primary-400 hover:text-primary-700 dark:hover:text-primary-300 font-medium transition-colors flex items-center gap-1.5"
        onclick={loadQueue}
        title="Refresh queue"
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182" />
        </svg>
        Refresh
      </button>
    </div>

    <div class="bg-white dark:bg-gray-800 rounded-2xl shadow-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
      {#if loading}
        <div class="flex flex-col items-center justify-center py-16">
          <div class="animate-spin rounded-full h-10 w-10 border-b-2 border-primary-500 mb-4"></div>
          <p class="text-sm text-gray-500 dark:text-gray-400">Loading tracks…</p>
        </div>
      {:else if error}
        <div class="flex flex-col items-center justify-center py-16 px-4">
          <div class="w-12 h-12 rounded-full bg-red-100 dark:bg-red-900/30 flex items-center justify-center mb-3">
            <svg class="w-6 h-6 text-red-500" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />
            </svg>
          </div>
          <p class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Could not load queue</p>
          <p class="text-xs text-gray-500 dark:text-gray-400 mb-3 text-center">{error}</p>
          <button
            type="button"
            class="px-4 py-2 text-sm font-medium text-white bg-primary-500 hover:bg-primary-600 rounded-lg transition-colors"
            onclick={loadQueue}
          >
            Try Again
          </button>
        </div>
      {:else}
        <TrackList
          tracks={displayTracks}
          editable={false}
          showIndex={true}
          showFormat={true}
          highlightChecksum={currentChecksum}
          emptyMessage="No tracks in the queue yet."
        />

        {#if hasMore}
          <div class="px-4 py-3 border-t border-gray-100 dark:border-gray-800 text-center">
            <button
              type="button"
              class="text-sm font-medium text-primary-600 dark:text-primary-400 hover:text-primary-700 dark:hover:text-primary-300 transition-colors"
              onclick={() => (showFullQueue = true)}
            >
              Show all {tracks.length} tracks
              <svg class="inline-block w-4 h-4 ml-1" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
              </svg>
            </button>
          </div>
        {:else if showFullQueue && tracks.length > 25}
          <div class="px-4 py-3 border-t border-gray-100 dark:border-gray-800 text-center">
            <button
              type="button"
              class="text-sm font-medium text-primary-600 dark:text-primary-400 hover:text-primary-700 dark:hover:text-primary-300 transition-colors"
              onclick={() => (showFullQueue = false)}
            >
              Show fewer tracks
              <svg class="inline-block w-4 h-4 ml-1" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 15.75l7.5-7.5 7.5 7.5" />
              </svg>
            </button>
          </div>
        {/if}
      {/if}
    </div>
  </section>
</div>
