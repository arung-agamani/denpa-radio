<script>
  import { createEventDispatcher } from 'svelte';
  import anime from 'animejs';

  /** @type {Array<{id: number, title: string, artist?: string, album?: string, genre?: string, format?: string, duration?: number, filePath: string, checksum: string}>} */
  export let tracks = [];

  /** Whether to show management actions (drag handle, remove) */
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

  // ---------------------------------------------------------------------------
  // Drag and drop state
  // ---------------------------------------------------------------------------

  let dragSourceIndex = -1;
  let dropTargetIndex = -1;
  let isDragging = false;
  let listContainer;

  /** @type {Map<number, HTMLElement>} track id -> row element */
  let rowElements = new Map();

  function handleRemove(track, index) {
    dispatch('remove', { track, index });
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

  // ---------------------------------------------------------------------------
  // Drag & drop handlers
  // ---------------------------------------------------------------------------

  function onDragStart(e, index) {
    dragSourceIndex = index;
    isDragging = true;
    dropTargetIndex = index;

    // Set drag data
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', String(index));

    // Style the dragged element
    const row = e.currentTarget;
    requestAnimationFrame(() => {
      row.classList.add('tracklist-dragging');
    });

    // Animate lift effect
    anime({
      targets: row,
      scale: [1, 1.02],
      boxShadow: ['0 0 0 rgba(0,0,0,0)', '0 8px 25px rgba(0,0,0,0.15)'],
      duration: 200,
      easing: 'easeOutCubic',
    });
  }

  function onDragEnd(e) {
    const row = e.currentTarget;
    row.classList.remove('tracklist-dragging');

    // Animate back to normal
    anime({
      targets: row,
      scale: 1,
      boxShadow: '0 0 0 rgba(0,0,0,0)',
      duration: 200,
      easing: 'easeOutCubic',
    });

    // Perform the move if target changed
    if (dragSourceIndex !== -1 && dropTargetIndex !== -1 && dragSourceIndex !== dropTargetIndex) {
      dispatch('move', { from: dragSourceIndex, to: dropTargetIndex, track: tracks[dragSourceIndex] });
    }

    // Reset all gap animations
    clearAllGaps();

    dragSourceIndex = -1;
    dropTargetIndex = -1;
    isDragging = false;
  }

  function onDragOver(e, index) {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';

    if (index === dragSourceIndex) {
      if (dropTargetIndex !== dragSourceIndex) {
        dropTargetIndex = dragSourceIndex;
        animateGaps(dragSourceIndex);
      }
      return;
    }

    if (dropTargetIndex !== index) {
      dropTargetIndex = index;
      animateGaps(index);
    }
  }

  function onDragEnter(e, index) {
    e.preventDefault();
  }

  function onDrop(e, index) {
    e.preventDefault();
    // The move is handled in onDragEnd
    dropTargetIndex = index;
  }

  // ---------------------------------------------------------------------------
  // Gap animation using anime.js
  // ---------------------------------------------------------------------------

  let currentGapAnimation = null;

  function animateGaps(targetIndex) {
    if (!listContainer) return;

    const rows = listContainer.querySelectorAll('.tracklist-row');
    if (!rows.length) return;

    // Cancel previous animations
    if (currentGapAnimation) {
      currentGapAnimation.pause();
    }

    const targets = [];
    const marginValues = [];

    rows.forEach((row, i) => {
      if (i === dragSourceIndex) {
        // The source row is being dragged, collapse its space
        targets.push(row);
        marginValues.push({ marginTop: '0px', marginBottom: '0px' });
        return;
      }

      let mt = '0px';
      let mb = '0px';

      if (targetIndex <= dragSourceIndex) {
        // Dragging upward: gap opens above the target
        if (i === targetIndex) {
          mt = '44px';
        }
      } else {
        // Dragging downward: gap opens below the target
        if (i === targetIndex) {
          mb = '44px';
        }
      }

      targets.push(row);
      marginValues.push({ marginTop: mt, marginBottom: mb });
    });

    // Animate each row individually
    const animations = targets.map((el, i) => {
      return anime({
        targets: el,
        marginTop: marginValues[i].marginTop,
        marginBottom: marginValues[i].marginBottom,
        duration: 250,
        easing: 'easeOutCubic',
        autoplay: false,
      });
    });

    // Play all
    const timeline = anime.timeline({ autoplay: true });
    targets.forEach((el, i) => {
      timeline.add({
        targets: el,
        marginTop: marginValues[i].marginTop,
        marginBottom: marginValues[i].marginBottom,
        duration: 250,
        easing: 'easeOutCubic',
      }, 0);
    });

    currentGapAnimation = timeline;
  }

  function clearAllGaps() {
    if (!listContainer) return;
    if (currentGapAnimation) {
      currentGapAnimation.pause();
      currentGapAnimation = null;
    }
    const rows = listContainer.querySelectorAll('.tracklist-row');
    anime({
      targets: rows,
      marginTop: '0px',
      marginBottom: '0px',
      duration: 200,
      easing: 'easeOutCubic',
    });
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
  <div class="overflow-x-auto" bind:this={listContainer}>
    <!-- Header row -->
    <div class="tracklist-header grid border-b border-gray-200 dark:border-gray-700 text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400"
         style="grid-template-columns: {editable ? '3rem ' : ''}{showIndex ? '3rem ' : ''}1fr minmax(0, 12rem) minmax(0, 12rem){showFormat ? ' 5rem' : ''} 5rem;">
      {#if editable}
        <div class="px-2 {compact ? 'py-2' : 'py-3'} text-center"></div>
      {/if}
      {#if showIndex}
        <div class="px-2 {compact ? 'py-2' : 'py-3'} text-center">#</div>
      {/if}
      <div class="px-3 {compact ? 'py-2' : 'py-3'}">Title</div>
      <div class="px-3 {compact ? 'py-2' : 'py-3'} hidden sm:block">Artist</div>
      <div class="px-3 {compact ? 'py-2' : 'py-3'} hidden md:block">Album</div>
      {#if showFormat}
        <div class="px-2 {compact ? 'py-2' : 'py-3'} hidden lg:block text-center">Format</div>
      {/if}
      <div class="px-3 {compact ? 'py-2' : 'py-3'} hidden lg:block text-right">Duration</div>
    </div>

    <!-- Rows -->
    {#each tracks as track, index (track.id || track.checksum || index)}
      {@const isHighlighted = highlightChecksum && track.checksum === highlightChecksum}
      <div
        class="tracklist-row grid items-center border-b border-gray-100 dark:border-gray-800 transition-colors relative
          {isHighlighted
            ? 'bg-primary-50 dark:bg-primary-900/20 border-l-2 border-l-primary-500'
            : 'hover:bg-gray-50 dark:hover:bg-gray-800/50'}
          {isDragging && index === dragSourceIndex ? 'opacity-40' : ''}"
        style="grid-template-columns: {editable ? '3rem ' : ''}{showIndex ? '3rem ' : ''}1fr minmax(0, 12rem) minmax(0, 12rem){showFormat ? ' 5rem' : ''} 5rem;"
        draggable={editable}
        on:dragstart={(e) => editable && onDragStart(e, index)}
        on:dragend={(e) => editable && onDragEnd(e)}
        on:dragover={(e) => editable && onDragOver(e, index)}
        on:dragenter={(e) => editable && onDragEnter(e, index)}
        on:drop={(e) => editable && onDrop(e, index)}
        on:click={() => handleSelect(track, index)}
        role="button"
        tabindex="0"
        on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); handleSelect(track, index); } }}
      >
        {#if editable}
          <div class="px-2 {compact ? 'py-1.5' : 'py-3'} flex items-center justify-center gap-1">
            <!-- Drag handle -->
            <div
              class="cursor-grab active:cursor-grabbing p-1 rounded text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
              title="Drag to reorder"
              on:mousedown|stopPropagation
            >
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5" />
              </svg>
            </div>

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
        {/if}

        {#if showIndex}
          <div class="px-2 {compact ? 'py-1.5' : 'py-3'} text-center">
            <span class="text-xs font-mono text-gray-400 dark:text-gray-500 {isHighlighted ? 'text-primary-500 dark:text-primary-400 font-semibold' : ''}">
              {#if isHighlighted}
                <span class="inline-block animate-pulse">▶</span>
              {:else}
                {index + 1}
              {/if}
            </span>
          </div>
        {/if}

        <div class="px-3 {compact ? 'py-1.5' : 'py-3'} min-w-0">
          <p class="text-sm font-medium text-gray-900 dark:text-white truncate {isHighlighted ? 'text-primary-700 dark:text-primary-300' : ''}"
             title={track.title}>
            {track.title || 'Untitled'}
          </p>
          {#if track.artist}
            <p class="text-xs text-gray-500 dark:text-gray-400 truncate sm:hidden mt-0.5">
              {track.artist}
            </p>
          {/if}
          {#if compact}
            <p class="text-xs text-gray-400 dark:text-gray-600 truncate mt-0.5" title={track.filePath}>
              {shortenPath(track.filePath)}
            </p>
          {/if}
        </div>

        <div class="px-3 {compact ? 'py-1.5' : 'py-3'} hidden sm:block min-w-0">
          <span class="text-sm text-gray-600 dark:text-gray-300 truncate block" title={track.artist || ''}>
            {track.artist || '—'}
          </span>
        </div>

        <div class="px-3 {compact ? 'py-1.5' : 'py-3'} hidden md:block min-w-0">
          <span class="text-sm text-gray-500 dark:text-gray-400 truncate block" title={track.album || ''}>
            {track.album || '—'}
          </span>
        </div>

        {#if showFormat}
          <div class="px-2 {compact ? 'py-1.5' : 'py-3'} hidden lg:block text-center">
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
          </div>
        {/if}

        <div class="px-3 {compact ? 'py-1.5' : 'py-3'} hidden lg:block text-right">
          <span class="text-xs text-gray-500 dark:text-gray-400 font-mono tabular-nums">
            {formatDuration(track.duration)}
          </span>
        </div>
      </div>
    {/each}
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

<style>
  .tracklist-dragging {
    z-index: 50;
    position: relative;
    border-radius: 0.5rem;
    box-shadow: 0 8px 25px rgba(0, 0, 0, 0.15);
  }

  .tracklist-row {
    will-change: margin-top, margin-bottom, transform;
  }

  /* Make dragged ghost image slightly transparent */
  .tracklist-row:global(.tracklist-dragging) {
    opacity: 0.85;
  }
</style>
