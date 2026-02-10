<script>
  import { isPlaying } from '../lib/stores.js';

  let audioEl;
  let volume = 0.8;
  let loading = false;
  let error = null;

  const streamUrl = '/stream';

  function play() {
    if (!audioEl) return;
    error = null;
    loading = true;

    // Force a fresh connection by resetting the src each time.
    audioEl.src = streamUrl;
    audioEl.load();
    audioEl.volume = volume;

    const playPromise = audioEl.play();
    if (playPromise !== undefined) {
      playPromise
        .then(() => {
          $isPlaying = true;
          loading = false;
        })
        .catch((err) => {
          // Ignore AbortError which happens when the user stops quickly.
          if (err.name !== 'AbortError') {
            error = 'Could not start playback. Please try again.';
            console.error('Playback error:', err);
          }
          $isPlaying = false;
          loading = false;
        });
    }
  }

  function stop() {
    if (!audioEl) return;
    audioEl.pause();
    audioEl.removeAttribute('src');
    audioEl.load();
    $isPlaying = false;
    loading = false;
    error = null;
  }

  function toggle() {
    if ($isPlaying) {
      stop();
    } else {
      play();
    }
  }

  function handleVolumeChange(e) {
    volume = parseFloat(e.target.value);
    if (audioEl) {
      audioEl.volume = volume;
    }
  }

  function handleError() {
    if ($isPlaying || loading) {
      error = 'Stream connection lost. Click play to reconnect.';
      $isPlaying = false;
      loading = false;
    }
  }

  function handleWaiting() {
    if ($isPlaying) {
      loading = true;
    }
  }

  function handlePlaying() {
    loading = false;
    error = null;
  }

  $: volumePercent = Math.round(volume * 100);

  $: volumeIcon = volume === 0
    ? '🔇'
    : volume < 0.33
      ? '🔈'
      : volume < 0.66
        ? '🔉'
        : '🔊';
</script>

<div class="bg-white dark:bg-gray-800 rounded-2xl shadow-lg border border-gray-200 dark:border-gray-700 p-5">
  <!-- Hidden audio element -->
  <audio
    bind:this={audioEl}
    preload="none"
    crossorigin="anonymous"
    on:error={handleError}
    on:waiting={handleWaiting}
    on:playing={handlePlaying}
  />

  <div class="flex flex-col gap-4">
    <!-- Play/stop and status row -->
    <div class="flex items-center gap-4">
      <button
        type="button"
        class="flex-shrink-0 w-14 h-14 rounded-full flex items-center justify-center text-white transition-all duration-200 focus:outline-none focus:ring-4 focus:ring-primary-300 dark:focus:ring-primary-800 {$isPlaying
          ? 'bg-red-500 hover:bg-red-600 shadow-red-200 dark:shadow-red-900/30'
          : 'bg-primary-500 hover:bg-primary-600 shadow-primary-200 dark:shadow-primary-900/30'} shadow-lg hover:scale-105 active:scale-95"
        on:click={toggle}
        disabled={loading}
        aria-label={$isPlaying ? 'Stop' : 'Play'}
      >
        {#if loading}
          <svg class="animate-spin w-6 h-6" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
          </svg>
        {:else if $isPlaying}
          <svg class="w-6 h-6" fill="currentColor" viewBox="0 0 24 24">
            <rect x="6" y="5" width="4" height="14" rx="1" />
            <rect x="14" y="5" width="4" height="14" rx="1" />
          </svg>
        {:else}
          <svg class="w-6 h-6 ml-0.5" fill="currentColor" viewBox="0 0 24 24">
            <path d="M8 5.14v14l11-7-11-7z" />
          </svg>
        {/if}
      </button>

      <div class="flex-1 min-w-0">
        <p class="text-sm font-medium text-gray-500 dark:text-gray-400">
          {#if loading}
            Connecting to stream…
          {:else if $isPlaying}
            Now streaming live
          {:else}
            Ready to play
          {/if}
        </p>

        {#if error}
          <p class="text-xs text-red-500 dark:text-red-400 mt-1">{error}</p>
        {/if}
      </div>

      <!-- Live indicator -->
      {#if $isPlaying}
        <div class="flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-red-100 dark:bg-red-900/50 text-red-600 dark:text-red-400 text-xs font-bold uppercase tracking-wide">
          <span class="relative flex h-2 w-2">
            <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-red-500 opacity-75"></span>
            <span class="relative inline-flex rounded-full h-2 w-2 bg-red-500"></span>
          </span>
          Live
        </div>
      {/if}
    </div>

    <!-- Volume slider -->
    <div class="flex items-center gap-3">
      <span class="text-lg flex-shrink-0 w-7 text-center" title="Volume {volumePercent}%">{volumeIcon}</span>

      <input
        type="range"
        min="0"
        max="1"
        step="0.01"
        value={volume}
        on:input={handleVolumeChange}
        class="flex-1 h-2 rounded-full appearance-none cursor-pointer bg-gray-200 dark:bg-gray-700 accent-primary-500"
        aria-label="Volume"
      />

      <span class="text-xs text-gray-500 dark:text-gray-400 w-10 text-right tabular-nums">{volumePercent}%</span>
    </div>
  </div>
</div>
