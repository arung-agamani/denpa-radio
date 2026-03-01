<script lang="ts">
  import { onMount } from 'svelte';
  import { isPlaying, currentTrackInfo, stationName } from '../lib/stores';

  let audioEl: HTMLAudioElement | undefined;
  let volume = 0.8;
  let loading = false;
  let error: string | null = null;

  const streamUrl = '/stream';

  // ------------------------------------------------------------------
  // Stall / reconnect logic
  // If the stream enters a buffering state for longer than STALL_TIMEOUT_MS
  // we silently reload the source so minor network hiccups self-heal.
  // ------------------------------------------------------------------
  const STALL_TIMEOUT_MS = 5000;
  let stallTimer: ReturnType<typeof setTimeout> | null = null;

  function startStallTimer() {
    clearStallTimer();
    stallTimer = setTimeout(() => {
      if ($isPlaying) {
        console.warn('[Player] Stream stalled for 5 s – reconnecting…');
        reconnect();
      }
    }, STALL_TIMEOUT_MS);
  }

  function clearStallTimer() {
    if (stallTimer !== null) {
      clearTimeout(stallTimer);
      stallTimer = null;
    }
  }

  function reconnect() {
    if (!audioEl) return;
    clearStallTimer();
    audioEl.src = streamUrl;
    audioEl.load();
    audioEl.volume = volume;
    audioEl.play().catch((err: Error) => {
      if (err.name !== 'AbortError') {
        error = 'Stream connection lost. Click play to reconnect.';
        $isPlaying = false;
        loading = false;
      }
    });
  }

  // ------------------------------------------------------------------
  // Media Session API helpers
  // Registers OS-level playback actions (keyboard media keys, lock screen
  // controls on Android / macOS / Windows) and keeps the track metadata
  // in sync so the OS displays the current track title and artist.
  //
  // sessionAttached tracks whether this tab currently owns the OS media
  // controls. The user can voluntarily detach (release) them without
  // stopping audio, so that other tabs / apps can take over OS controls.
  // ------------------------------------------------------------------
  let sessionAttached = false;

  function registerMediaSessionHandlers() {
    if (!('mediaSession' in navigator)) return;

    navigator.mediaSession.setActionHandler('play', () => {
      if (!$isPlaying) play();
    });
    navigator.mediaSession.setActionHandler('pause', () => {
      if ($isPlaying) stop();
    });
    navigator.mediaSession.setActionHandler('stop', () => {
      stop();
    });
  }

  function unregisterMediaSessionHandlers() {
    if (!('mediaSession' in navigator)) return;

    navigator.mediaSession.setActionHandler('play', null);
    navigator.mediaSession.setActionHandler('pause', null);
    navigator.mediaSession.setActionHandler('stop', null);
  }

  function updateMediaSession() {
    if (!('mediaSession' in navigator)) return;

    const track = $currentTrackInfo;
    const station = $stationName || 'Denpa Radio';

    navigator.mediaSession.metadata = new MediaMetadata({
      title: track?.title || station,
      artist: track?.artist || station,
      album: track?.album || station,
    });

    navigator.mediaSession.playbackState = 'playing';
    sessionAttached = true;
  }

  function pauseMediaSession() {
    if (!('mediaSession' in navigator)) return;
    // Keep the metadata intact and mark as paused so the OS retains the
    // media controls widget (e.g. Windows system tray, macOS menu bar).
    // Setting 'none' would dismiss the widget entirely, making it impossible
    // to resume via keyboard / OS controls.
    navigator.mediaSession.playbackState = 'paused';
  }

  // Detach: fully release the OS media session widget so other tabs / apps
  // can take over OS controls. Audio continues playing uninterrupted.
  function detachMediaSession() {
    if (!('mediaSession' in navigator)) return;

    unregisterMediaSessionHandlers();
    navigator.mediaSession.metadata = null;
    navigator.mediaSession.playbackState = 'none';
    sessionAttached = false;
  }

  // Re-attach: reclaim OS media controls for this tab while audio is playing.
  function attachMediaSession() {
    if (!('mediaSession' in navigator)) return;

    registerMediaSessionHandlers();
    updateMediaSession();
  }

  function toggleMediaSession() {
    if (sessionAttached) {
      detachMediaSession();
    } else {
      attachMediaSession();
    }
  }

  // Re-push track metadata to the OS whenever the current track changes,
  // but only if this tab still owns the session.
  $: if ($isPlaying && sessionAttached && $currentTrackInfo !== null) {
    updateMediaSession();
  }

  onMount(() => {
    registerMediaSessionHandlers();
  });

  // ------------------------------------------------------------------
  // Playback control
  // ------------------------------------------------------------------
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
          attachMediaSession();
        })
        .catch((err: Error) => {
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
    clearStallTimer();
    audioEl.pause();
    audioEl.removeAttribute('src');
    audioEl.load();
    $isPlaying = false;
    loading = false;
    error = null;
    // If session is attached, keep widget visible in paused state so the
    // user can resume via OS controls. If already detached, leave it alone.
    if (sessionAttached) pauseMediaSession();
  }

  function toggle() {
    if ($isPlaying) {
      stop();
    } else {
      play();
    }
  }

  function handleVolumeChange(e: Event) {
    volume = parseFloat((e.target as HTMLInputElement).value);
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
    clearStallTimer();
    if (sessionAttached) pauseMediaSession();
  }

  function handleWaiting() {
    if ($isPlaying) {
      loading = true;
      startStallTimer();
    }
  }

  function handlePlaying() {
    loading = false;
    error = null;
    clearStallTimer();
  }

  function handleStalled() {
    if ($isPlaying) {
      loading = true;
      startStallTimer();
    }
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
    on:stalled={handleStalled}
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

      <!-- Live indicator + OS controls detach toggle -->
      {#if $isPlaying}
        <div class="flex items-center gap-2">
          <div class="flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-red-100 dark:bg-red-900/50 text-red-600 dark:text-red-400 text-xs font-bold uppercase tracking-wide">
            <span class="relative flex h-2 w-2">
              <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-red-500 opacity-75"></span>
              <span class="relative inline-flex rounded-full h-2 w-2 bg-red-500"></span>
            </span>
            Live
          </div>

          <!-- Detach / re-attach OS media session controls.
               Detaching releases keyboard media keys and OS lock-screen
               controls so other apps / tabs can claim them, while the
               audio stream keeps playing uninterrupted. -->
          <button
            type="button"
            on:click={toggleMediaSession}
            aria-label={sessionAttached ? 'Detach OS media controls (audio keeps playing)' : 'Attach OS media controls'}
            title={sessionAttached ? 'Detach OS media controls' : 'Attach OS media controls'}
            class="w-7 h-7 rounded-full flex items-center justify-center transition-all duration-150 focus:outline-none focus:ring-2 focus:ring-primary-400 {sessionAttached
              ? 'text-primary-500 hover:text-primary-700 hover:bg-primary-50 dark:hover:bg-primary-900/30'
              : 'text-gray-400 hover:text-gray-600 hover:bg-gray-100 dark:hover:bg-gray-700'}"
          >
            {#if sessionAttached}
              <!-- Link icon: OS controls active -->
              <svg class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" d="M13.828 10.172a4 4 0 0 1 0 5.656l-3 3a4 4 0 0 1-5.656-5.656l1.5-1.5" />
                <path stroke-linecap="round" stroke-linejoin="round" d="M10.172 13.828a4 4 0 0 1 0-5.656l3-3a4 4 0 0 1 5.656 5.656l-1.5 1.5" />
              </svg>
            {:else}
              <!-- Unlink icon: OS controls detached -->
              <svg class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" d="M13.828 10.172a4 4 0 0 1 0 5.656l-3 3a4 4 0 0 1-5.656-5.656l1.5-1.5" />
                <path stroke-linecap="round" stroke-linejoin="round" d="M10.172 13.828a4 4 0 0 1 0-5.656l3-3a4 4 0 0 1 5.656 5.656l-1.5 1.5" />
                <line x1="4" y1="4" x2="20" y2="20" stroke-linecap="round" />
              </svg>
            {/if}
          </button>
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
