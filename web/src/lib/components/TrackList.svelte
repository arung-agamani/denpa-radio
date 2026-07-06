<script lang="ts">
  import anime from 'animejs';
  import type { TrackItem } from '$lib/api';

  let { tracks = [], editable = false, showIndex = true, showFormat = true, highlightChecksum = '', compact = false, emptyMessage = 'No tracks in this playlist.', onremove, onmove, onselect }: {
    tracks?: TrackItem[];
    editable?: boolean;
    showIndex?: boolean;
    showFormat?: boolean;
    highlightChecksum?: string;
    compact?: boolean;
    emptyMessage?: string;
    onremove?: (track: TrackItem, index: number) => void;
    onmove?: (from: number, to: number) => void;
    onselect?: (track: TrackItem, index: number) => void;
  } = $props();

  // Drag and drop state
  let dragSourceIndex = $state(-1);
  let dropTargetIndex = $state(-1);
  let isDragging = $state(false);
  let listContainer: HTMLDivElement | undefined = $state(undefined);

  function handleRemove(track: TrackItem, index: number): void { onremove?.(track, index); }
  function handleSelect(track: TrackItem, index: number): void { onselect?.(track, index); }

  function formatDuration(seconds: number | undefined): string {
    if (!seconds || seconds <= 0) return '';
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return `${m}:${s.toString().padStart(2, '0')}`;
  }

  function shortenPath(filePath: string | undefined): string {
    if (!filePath) return '';
    const parts = filePath.replace(/\\/g, '/').split('/');
    if (parts.length <= 2) return filePath;
    return '…/' + parts.slice(-2).join('/');
  }

  // Drag & drop handlers
  function onDragStart(e: DragEvent, index: number): void {
    dragSourceIndex = index;
    isDragging = true;
    dropTargetIndex = index;
    e.dataTransfer!.effectAllowed = 'move';
    e.dataTransfer!.setData('text/plain', String(index));
    const row = e.currentTarget as HTMLElement;
    requestAnimationFrame(() => { row.classList.add('tracklist-dragging'); });
    anime({ targets: row, scale: [1, 1.02], boxShadow: ['0 0 0 rgba(0,0,0,0)', '0 8px 25px rgba(0,0,0,0.15)'], duration: 200, easing: 'easeOutCubic' });
  }

  function onDragEnd(e: DragEvent): void {
    const row = e.currentTarget as HTMLElement;
    row.classList.remove('tracklist-dragging');
    anime({ targets: row, scale: 1, boxShadow: '0 0 0 rgba(0,0,0,0)', duration: 200, easing: 'easeOutCubic' });
    if (dragSourceIndex !== -1 && dropTargetIndex !== -1 && dragSourceIndex !== dropTargetIndex) {
      onmove?.(dragSourceIndex, dropTargetIndex);
    }
    clearAllGaps();
    dragSourceIndex = -1;
    dropTargetIndex = -1;
    isDragging = false;
  }

  function onDragOver(e: DragEvent, index: number): void {
    e.preventDefault();
    e.dataTransfer!.dropEffect = 'move';
    if (index === dragSourceIndex) {
      if (dropTargetIndex !== dragSourceIndex) { dropTargetIndex = dragSourceIndex; animateGaps(dragSourceIndex); }
      return;
    }
    if (dropTargetIndex !== index) { dropTargetIndex = index; animateGaps(index); }
  }

  function onDragEnter(e: DragEvent, _index: number): void { e.preventDefault(); }
  function onDrop(e: DragEvent, index: number): void { e.preventDefault(); dropTargetIndex = index; }

  // Gap animation using anime.js
  let currentGapAnimation: anime.AnimeTimelineInstance | null = null;

  function animateGaps(targetIndex: number): void {
    if (!listContainer) return;
    const rows = listContainer.querySelectorAll('.tracklist-row');
    if (!rows.length) return;
    if (currentGapAnimation) currentGapAnimation.pause();

    const targets: Element[] = [];
    const marginValues: { marginTop: string; marginBottom: string }[] = [];

    rows.forEach((row, i) => {
      if (i === dragSourceIndex) { targets.push(row); marginValues.push({ marginTop: '0px', marginBottom: '0px' }); return; }
      let mt = '0px'; let mb = '0px';
      if (targetIndex <= dragSourceIndex) { if (i === targetIndex) mt = '44px'; } else { if (i === targetIndex) mb = '44px'; }
      targets.push(row); marginValues.push({ marginTop: mt, marginBottom: mb });
    });

    const timeline = anime.timeline({ autoplay: true });
    targets.forEach((el, i) => { timeline.add({ targets: el, marginTop: marginValues[i].marginTop, marginBottom: marginValues[i].marginBottom, duration: 250, easing: 'easeOutCubic' }, 0); });
    currentGapAnimation = timeline;
  }

  function clearAllGaps(): void {
    if (!listContainer) return;
    if (currentGapAnimation) { currentGapAnimation.pause(); currentGapAnimation = null; }
    const rows = listContainer.querySelectorAll('.tracklist-row');
    anime({ targets: rows, marginTop: '0px', marginBottom: '0px', duration: 200, easing: 'easeOutCubic' });
  }
</script>

{#if tracks.length === 0}
  <div class="flex flex-col items-center justify-center py-12 text-gray-400 dark:text-gray-500">
    <svg class="w-12 h-12 mb-3 opacity-50" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m9 9 10.5-3m0 6.553v3.75a2.25 2.25 0 0 1-1.632 2.163l-1.32.377a1.803 1.803 0 1 1-.99-3.467l2.31-.66a2.25 2.25 0 0 0 1.632-2.163Zm0 0V2.25L9 5.25v10.303m0 0v3.75a2.25 2.25 0 0 1-1.632 2.163l-1.32.377a1.803 1.803 0 0 1-.99-3.467l2.31-.66A2.25 2.25 0 0 0 9 15.553Z" /></svg>
    <p class="text-sm font-medium">{emptyMessage}</p>
  </div>
{:else}
  <div class="overflow-x-auto" bind:this={listContainer}>
    <div class="tracklist-header grid border-b border-gray-200 dark:border-gray-700 text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400" style="grid-template-columns: {editable ? '3rem ' : ''}{showIndex ? '3rem ' : ''}1fr minmax(0, 12rem) minmax(0, 12rem){showFormat ? ' 5rem' : ''} 5rem;">
      {#if editable}<div class="px-2 {compact ? 'py-2' : 'py-3'} text-center"></div>{/if}
      {#if showIndex}<div class="px-2 {compact ? 'py-2' : 'py-3'} text-center">#</div>{/if}
      <div class="px-3 {compact ? 'py-2' : 'py-3'}">Title</div>
      <div class="px-3 {compact ? 'py-2' : 'py-3'} hidden sm:block">Artist</div>
      <div class="px-3 {compact ? 'py-2' : 'py-3'} hidden md:block">Album</div>
      {#if showFormat}<div class="px-2 {compact ? 'py-2' : 'py-3'} hidden lg:block text-center">Format</div>{/if}
      <div class="px-3 {compact ? 'py-2' : 'py-3'} hidden lg:block text-right">Duration</div>
    </div>

    {#each tracks as track, index (track.id || track.checksum || index)}
      {@const isHighlighted = highlightChecksum && track.checksum === highlightChecksum}
      <div class="tracklist-row grid items-center border-b border-gray-100 dark:border-gray-800 transition-colors relative {isHighlighted ? 'bg-primary-50 dark:bg-primary-900/20 border-l-2 border-l-primary-500' : 'hover:bg-gray-50 dark:hover:bg-gray-800/50'} {isDragging && index === dragSourceIndex ? 'opacity-40' : ''}" style="grid-template-columns: {editable ? '3rem ' : ''}{showIndex ? '3rem ' : ''}1fr minmax(0, 12rem) minmax(0, 12rem){showFormat ? ' 5rem' : ''} 5rem;" draggable={editable} ondragstart={(e) => editable && onDragStart(e, index)} ondragend={(e) => editable && onDragEnd(e)} ondragover={(e) => editable && onDragOver(e, index)} ondragenter={(e) => editable && onDragEnter(e, index)} ondrop={(e) => editable && onDrop(e, index)} onclick={() => handleSelect(track, index)} role="button" tabindex="0" onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); handleSelect(track, index); } }}>
        {#if editable}
          <div class="px-2 {compact ? 'py-1.5' : 'py-3'} flex items-center justify-center gap-1">
            <button type="button" class="cursor-grab active:cursor-grabbing p-1 rounded text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors" title="Drag to reorder" aria-label="Drag to reorder" onmousedown={(e) => e.stopPropagation()}>
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5" /></svg>
            </button>
            <button type="button" class="p-1 rounded hover:bg-red-100 dark:hover:bg-red-900/40 text-gray-400 hover:text-red-600 dark:hover:text-red-400 transition-colors" title="Remove from playlist" onclick={(e) => { e.stopPropagation(); handleRemove(track, index); }}>
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
            </button>
          </div>
        {/if}

        {#if showIndex}
          <div class="px-2 {compact ? 'py-1.5' : 'py-3'} text-center">
            <span class="text-xs font-mono text-gray-400 dark:text-gray-500 {isHighlighted ? 'text-primary-500 dark:text-primary-400 font-semibold' : ''}">
              {#if isHighlighted}<span class="inline-block animate-pulse">▶</span>{:else}{index + 1}{/if}
            </span>
          </div>
        {/if}

        <div class="px-3 {compact ? 'py-1.5' : 'py-3'} min-w-0">
          <p class="text-sm font-medium text-gray-900 dark:text-white truncate {isHighlighted ? 'text-primary-700 dark:text-primary-300' : ''}" title={track.title}>{track.title || 'Untitled'}</p>
          {#if track.artist}<p class="text-xs text-gray-500 dark:text-gray-400 truncate sm:hidden mt-0.5">{track.artist}</p>{/if}
          {#if compact}<p class="text-xs text-gray-400 dark:text-gray-600 truncate mt-0.5" title={track.filePath}>{shortenPath(track.filePath)}</p>{/if}
        </div>

        <div class="px-3 {compact ? 'py-1.5' : 'py-3'} hidden sm:block min-w-0">
          <span class="text-sm text-gray-600 dark:text-gray-300 truncate block" title={track.artist || ''}>{track.artist || '—'}</span>
        </div>

        <div class="px-3 {compact ? 'py-1.5' : 'py-3'} hidden md:block min-w-0">
          <span class="text-sm text-gray-500 dark:text-gray-400 truncate block" title={track.album || ''}>{track.album || '—'}</span>
        </div>

        {#if showFormat}
          <div class="px-2 {compact ? 'py-1.5' : 'py-3'} hidden lg:block text-center">
            {#if track.format}
              <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium uppercase {track.format === 'flac' ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300' : track.format === 'mp3' ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300' : 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'}">{track.format}</span>
            {/if}
          </div>
        {/if}

        <div class="px-3 {compact ? 'py-1.5' : 'py-3'} hidden lg:block text-right">
          <span class="text-xs text-gray-500 dark:text-gray-400 font-mono tabular-nums">{formatDuration(track.duration)}</span>
        </div>
      </div>
    {/each}
  </div>

  <div class="px-3 py-2 flex items-center justify-between text-xs text-gray-400 dark:text-gray-500 border-t border-gray-100 dark:border-gray-800">
    <span>{tracks.length} track{tracks.length !== 1 ? 's' : ''}</span>
    {#if tracks.some(t => t.duration && t.duration > 0)}
      {@const totalSeconds = tracks.reduce((sum, t) => sum + (t.duration || 0), 0)}
      {@const hours = Math.floor(totalSeconds / 3600)}
      {@const minutes = Math.floor((totalSeconds % 3600) / 60)}
      <span>{#if hours > 0}{hours}h {minutes}m{:else}{minutes} min{/if} total</span>
    {/if}
  </div>
{/if}

<style>
  :global(.tracklist-dragging) { z-index: 50; position: relative; border-radius: 0.5rem; box-shadow: 0 8px 25px rgba(0, 0, 0, 0.15); }
  .tracklist-row { will-change: margin-top, margin-bottom, transform; }
  .tracklist-row:global(.tracklist-dragging) { opacity: 0.85; }
</style>
