<script lang="ts">
  import { onMount } from 'svelte';
  import {
    listChannels,
    createChannel,
    updateChannel,
    deleteChannel,
    skipNextChannel,
    skipPrevChannel,
    setChannelTimeSlots,
    assignPlaylistToChannelTag,
    removePlaylistFromChannelTag,
  } from '$lib/api';
  import type { ChannelInfo, CreateChannelBody, UpdateChannelBody } from '$lib/api';
  import type { TimeSlot } from '$lib/api';
  import { toasts, playlists, timeSlots } from '$lib/stores';
  import { getTagEmoji, getTagLabel, getTagColor, formatHourRange } from '$lib/tags';
  import TimeSlotEditor from '$lib/components/TimeSlotEditor.svelte';

  // Channel list state
  let channels: ChannelInfo[] = $state([]);
  let loading = $state(false);

  // Create form state
  let newName = $state('');
  let newSlug = $state('');
  let newDescription = $state('');
  let newBitrate = $state('');
  let newSortOrder = $state('');
  let creating = $state(false);

  // Edit state
  let editingSlug: string | null = $state(null);
  let editName = $state('');
  let editDescription = $state('');
  let editBitrate = $state('');
  let editSortOrder = $state('');
  let editEnabled = $state(false);
  let savingEdit = $state(false);

  // Delete confirmation
  let confirmDeleteSlug: string | null = $state(null);

  // Expanded channel (for time slots / master)
  let expandedSlug: string | null = $state(null);

  // Time slot editor state
  let editingSlotsSlug: string | null = $state(null);
  let savingSlots = $state(false);

  // Skip state per channel
  let skippingSlug: string | null = $state(null);

  // Assign playlist form per channel
  let assignTag: Record<string, string> = $state({});
  let assignPlaylistId: Record<string, number | null> = $state({});
  let assigningSlug: string | null = $state(null);

  // Channel master data (tag -> playlists)
  interface ChannelMaster {
    [tag: string]: { playlists: { id: number; name: string; trackCount: number }[]; count: number };
  }
  let channelMasterData: Record<string, ChannelMaster> = $state({});
  let loadingMasterSlug: string | null = $state(null);

  let sortedChannels = $derived([...channels].sort((a, b) => a.sortOrder - b.sortOrder));
  let allPlaylistsList = $derived($playlists || []);

  async function loadChannels() {
    loading = true;
    try {
      const res = await listChannels();
      channels = res.channels || [];
    } catch (err) {
      toasts.error('Failed to load channels: ' + (err instanceof Error ? err.message : String(err)));
    } finally {
      loading = false;
    }
  }

  async function handleCreate() {
    if (!newName.trim()) {
      toasts.warning('Please enter a channel name.');
      return;
    }
    if (!newSlug.trim()) {
      toasts.warning('Please enter a channel slug.');
      return;
    }
    const body: CreateChannelBody = {
      name: newName.trim(),
      slug: newSlug.trim(),
    };
    if (newDescription.trim()) body.description = newDescription.trim();
    if (newBitrate.trim()) body.bitrate = newBitrate.trim();
    if (newSortOrder.trim()) {
      const parsed = parseInt(newSortOrder.trim(), 10);
      if (!Number.isNaN(parsed)) body.sortOrder = parsed;
    }
    creating = true;
    try {
      await createChannel(body);
      toasts.success('Channel created!');
      newName = '';
      newSlug = '';
      newDescription = '';
      newBitrate = '';
      newSortOrder = '';
      await loadChannels();
    } catch (err) {
      toasts.error('Failed to create channel: ' + (err instanceof Error ? err.message : String(err)));
    } finally {
      creating = false;
    }
  }

  function startEdit(ch: ChannelInfo) {
    editingSlug = ch.slug;
    editName = ch.name;
    editDescription = ch.description || '';
    editBitrate = ch.bitrate || '';
    editSortOrder = String(ch.sortOrder);
    editEnabled = ch.enabled;
  }

  function cancelEdit() {
    editingSlug = null;
  }

  async function saveEdit(slug: string) {
    if (!editName.trim()) {
      toasts.warning('Channel name cannot be empty.');
      return;
    }
    const body: UpdateChannelBody = {
      name: editName.trim(),
      enabled: editEnabled,
    };
    if (editDescription.trim()) body.description = editDescription.trim();
    if (editBitrate.trim()) body.bitrate = editBitrate.trim();
    if (editSortOrder.trim()) {
      const parsed = parseInt(editSortOrder.trim(), 10);
      if (!Number.isNaN(parsed)) body.sortOrder = parsed;
    }
    savingEdit = true;
    try {
      await updateChannel(slug, body);
      toasts.success('Channel updated!');
      editingSlug = null;
      await loadChannels();
    } catch (err) {
      toasts.error('Failed to update channel: ' + (err instanceof Error ? err.message : String(err)));
    } finally {
      savingEdit = false;
    }
  }

  async function handleDelete(slug: string) {
    try {
      await deleteChannel(slug);
      toasts.success('Channel deleted.');
      confirmDeleteSlug = null;
      if (expandedSlug === slug) expandedSlug = null;
      if (editingSlug === slug) editingSlug = null;
      await loadChannels();
    } catch (err) {
      toasts.error('Failed to delete channel: ' + (err instanceof Error ? err.message : String(err)));
    }
  }

  async function handleSkipNext(slug: string) {
    skippingSlug = slug;
    try {
      await skipNextChannel(slug);
      toasts.success('Skipped to next track.');
    } catch (err) {
      toasts.error('Skip failed: ' + (err instanceof Error ? err.message : String(err)));
    } finally {
      skippingSlug = null;
    }
  }

  async function handleSkipPrev(slug: string) {
    skippingSlug = slug;
    try {
      await skipPrevChannel(slug);
      toasts.success('Skipped to previous track.');
    } catch (err) {
      toasts.error('Skip failed: ' + (err instanceof Error ? err.message : String(err)));
    } finally {
      skippingSlug = null;
    }
  }

  function toggleExpand(slug: string) {
    expandedSlug = expandedSlug === slug ? null : slug;
  }

  function openSlotEditor(slug: string) {
    editingSlotsSlug = slug;
  }

  async function saveSlots(slug: string, newSlots: TimeSlot[]) {
    savingSlots = true;
    try {
      await setChannelTimeSlots(slug, newSlots);
      toasts.success('Time slots updated for ' + slug + '!');
      editingSlotsSlug = null;
    } catch (err) {
      toasts.error('Failed to save time slots: ' + (err instanceof Error ? err.message : String(err)));
    } finally {
      savingSlots = false;
    }
  }

  function cancelSlotEditor() {
    editingSlotsSlug = null;
  }

  async function handleAssignToTag(slug: string) {
    const tag = assignTag[slug];
    const plId = assignPlaylistId[slug];
    if (!tag || !plId) {
      toasts.warning('Please select a tag and a playlist.');
      return;
    }
    assigningSlug = slug;
    try {
      await assignPlaylistToChannelTag(slug, tag, plId);
      toasts.success('Playlist assigned to ' + getTagLabel(tag, $timeSlots) + '!');
      assignPlaylistId[slug] = null;
      await loadChannelMaster(slug);
    } catch (err) {
      toasts.error('Failed to assign: ' + (err instanceof Error ? err.message : String(err)));
    } finally {
      assigningSlug = null;
    }
  }

  async function handleRemoveFromTag(slug: string, tag: string, playlistId: number) {
    try {
      await removePlaylistFromChannelTag(slug, tag, playlistId);
      toasts.success('Playlist removed from ' + getTagLabel(tag, $timeSlots));
      await loadChannelMaster(slug);
    } catch (err) {
      toasts.error('Failed to remove: ' + (err instanceof Error ? err.message : String(err)));
    }
  }

  async function loadChannelMaster(slug: string) {
    loadingMasterSlug = slug;
    try {
      const detail = await getChannelDetail(slug);
      channelMasterData[slug] = detail;
    } catch {
      // If we can't load master data, show empty
      channelMasterData[slug] = {};
    } finally {
      loadingMasterSlug = null;
    }
  }

  async function getChannelDetail(slug: string): Promise<ChannelMaster> {
    // The backend doesn't have a GET /channels/:slug/master endpoint,
    // so we return an empty master for now. The assign/remove still work.
    return {};
  }

  // Initialize assign defaults when expanding
  $effect(() => {
    if (expandedSlug && $timeSlots.length > 0 && !assignTag[expandedSlug]) {
      assignTag[expandedSlug] = $timeSlots[0].tag;
    }
  });

  onMount(() => {
    loadChannels();
    playlists.refresh();
  });
</script>

<h1 class="text-2xl font-bold text-gray-900 dark:text-white">Channels</h1>
<p class="text-sm text-gray-500 dark:text-gray-400 -mt-4">Manage broadcast channels, each with its own schedule and playlist assignments.</p>

<!-- Create channel form -->
<div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 p-5">
  <h3 class="text-base font-semibold text-gray-900 dark:text-white mb-4">Create New Channel</h3>
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
    <input type="text" bind:value={newName} placeholder="Channel name" class="px-4 py-2.5 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500" />
    <input type="text" bind:value={newSlug} placeholder="Slug (e.g. jazz)" class="px-4 py-2.5 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500" />
    <input type="text" bind:value={newDescription} placeholder="Description (optional)" class="px-4 py-2.5 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500" />
    <input type="text" bind:value={newBitrate} placeholder="Bitrate (e.g. 192k, optional)" class="px-4 py-2.5 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500" />
    <input type="number" bind:value={newSortOrder} placeholder="Sort order (optional)" class="px-4 py-2.5 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500" />
    <button type="button" class="px-6 py-2.5 text-sm font-semibold text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed" onclick={handleCreate} disabled={creating}>
      {creating ? 'Creating...' : 'Create'}
    </button>
  </div>
</div>

<!-- Channel list -->
<div class="bg-white dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
  <div class="px-5 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
    <h3 class="text-base font-semibold text-gray-900 dark:text-white">All Channels</h3>
    <button type="button" class="text-xs text-primary-600 dark:text-primary-400 hover:underline" onclick={loadChannels}>Refresh</button>
  </div>

  {#if loading}
    <div class="flex items-center justify-center py-12">
      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500"></div>
    </div>
  {:else if sortedChannels.length === 0}
    <div class="flex flex-col items-center justify-center py-12 text-gray-400 dark:text-gray-500">
      <span class="text-4xl mb-3">📻</span>
      <p class="text-sm font-medium">No channels yet</p>
      <p class="text-xs mt-1">Create one above to get started!</p>
    </div>
  {:else}
    <div class="divide-y divide-gray-100 dark:divide-gray-800">
      {#each sortedChannels as ch (ch.slug)}
        <div class="group">
          <!-- Channel row -->
          <div class="px-5 py-4 flex items-center gap-4 hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors">
            <!-- Sort order badge -->
            <div class="flex-shrink-0 w-10 h-10 rounded-lg flex items-center justify-center text-sm font-bold {ch.enabled ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/40 dark:text-primary-300' : 'bg-gray-100 text-gray-400 dark:bg-gray-700 dark:text-gray-500'}">
              {ch.sortOrder}
            </div>

            <!-- Channel info -->
            <div class="flex-1 min-w-0">
              {#if editingSlug === ch.slug}
                <!-- Inline edit form -->
                <div class="space-y-2">
                  <div class="flex flex-wrap gap-2">
                    <input type="text" bind:value={editName} placeholder="Name" class="flex-1 min-w-[120px] px-3 py-1.5 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-primary-500" />
                    <input type="text" bind:value={editDescription} placeholder="Description" class="flex-1 min-w-[120px] px-3 py-1.5 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-primary-500" />
                  </div>
                  <div class="flex flex-wrap gap-2">
                    <input type="text" bind:value={editBitrate} placeholder="Bitrate" class="w-28 px-3 py-1.5 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-primary-500" />
                    <input type="number" bind:value={editSortOrder} placeholder="Sort" class="w-20 px-3 py-1.5 text-sm rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-primary-500" />
                    <label class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300 px-2 py-1.5">
                      <input type="checkbox" bind:checked={editEnabled} class="rounded border-gray-300 dark:border-gray-600 text-primary-600 focus:ring-primary-500" />
                      Enabled
                    </label>
                  </div>
                  <div class="flex gap-2">
                    <button type="button" class="px-4 py-1.5 text-xs font-semibold text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors disabled:opacity-50" onclick={() => saveEdit(ch.slug)} disabled={savingEdit}>
                      {savingEdit ? 'Saving...' : 'Save'}
                    </button>
                    <button type="button" class="px-4 py-1.5 text-xs font-medium border border-gray-300 dark:border-gray-600 text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors" onclick={cancelEdit}>Cancel</button>
                  </div>
                </div>
              {:else}
                <p class="text-sm font-semibold text-gray-900 dark:text-white truncate">
                  {ch.name}
                  {#if !ch.enabled}
                    <span class="ml-2 text-xs font-normal text-gray-400 dark:text-gray-500">(disabled)</span>
                  {/if}
                </p>
                <p class="text-xs text-gray-500 dark:text-gray-400 truncate">
                  /{ch.slug}
                  {#if ch.description} · {ch.description}{/if}
                  {#if ch.bitrate} · {ch.bitrate}{/if}
                </p>
              {/if}
            </div>

            <!-- Action buttons (hidden during edit) -->
            {#if editingSlug !== ch.slug}
              <div class="flex items-center gap-1">
                <!-- Skip controls -->
                <button type="button" class="p-2 rounded-lg text-gray-400 hover:text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/30 dark:hover:text-blue-400 transition-colors" title="Skip to previous track" onclick={() => handleSkipPrev(ch.slug)} disabled={skippingSlug === ch.slug}>
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M18.75 19.5 12 12m0 0-6.75 7.5M12 12V3" /></svg>
                </button>
                <button type="button" class="p-2 rounded-lg text-gray-400 hover:text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/30 dark:hover:text-blue-400 transition-colors" title="Skip to next track" onclick={() => handleSkipNext(ch.slug)} disabled={skippingSlug === ch.slug}>
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M5.25 5.653c0-.856.917-1.398 1.667-.986l11.54 6.348a1.125 1.125 0 0 1 0 1.971l-11.54 6.347a1.125 1.125 0 0 1-1.667-.985V5.653Z" /></svg>
                </button>

                <!-- Expand toggle -->
                <button type="button" class="p-2 rounded-lg text-gray-400 hover:text-primary-600 hover:bg-primary-50 dark:hover:bg-primary-900/30 dark:hover:text-primary-400 transition-colors" title="Toggle schedule view" onclick={() => toggleExpand(ch.slug)}>
                  <svg class="w-4 h-4 {expandedSlug === ch.slug ? 'rotate-180' : ''} transition-transform" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M19.5 8.25 12 15.75 4.5 8.25" /></svg>
                </button>

                <!-- Edit -->
                <button type="button" class="p-2 rounded-lg text-gray-400 hover:text-primary-600 hover:bg-primary-50 dark:hover:bg-primary-900/30 dark:hover:text-primary-400 transition-colors" title="Edit channel" onclick={() => startEdit(ch)}>
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0 1 15.75 21H5.25A2.25 2.25 0 0 1 3 18.75V8.25A2.25 2.25 0 0 1 5.25 6H10" /></svg>
                </button>

                <!-- Delete (disabled for main) -->
                {#if ch.slug !== 'main'}
                  <button type="button" class="p-2 rounded-lg text-gray-400 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/30 dark:hover:text-red-400 transition-colors" title="Delete channel" onclick={() => (confirmDeleteSlug = ch.slug)}>
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" /></svg>
                  </button>
                {:else}
                  <button type="button" class="p-2 rounded-lg text-gray-300 dark:text-gray-600 cursor-not-allowed" title="Default channel cannot be deleted" disabled>
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" /></svg>
                  </button>
                {/if}
              </div>
            {/if}
          </div>

          <!-- Expanded schedule view -->
          {#if expandedSlug === ch.slug}
            <div class="px-5 py-4 bg-gray-50 dark:bg-gray-900/50 border-t border-gray-100 dark:border-gray-800 space-y-4">
              <!-- Time slot editor -->
              <div>
                <div class="flex items-center justify-between mb-2">
                  <h4 class="text-sm font-semibold text-gray-700 dark:text-gray-300">Time Slot Schedule</h4>
                  {#if editingSlotsSlug !== ch.slug}
                    <button type="button" class="px-3 py-1 text-xs font-semibold text-primary-600 dark:text-primary-400 border border-primary-300 dark:border-primary-700 rounded-lg hover:bg-primary-50 dark:hover:bg-primary-900/30 transition-colors" onclick={() => openSlotEditor(ch.slug)}>Configure</button>
                  {/if}
                </div>
                {#if editingSlotsSlug === ch.slug}
                  <TimeSlotEditor slots={$timeSlots} saving={savingSlots} onsave={(e) => saveSlots(ch.slug, e)} oncancel={cancelSlotEditor} />
                {:else}
                  <div class="flex flex-wrap gap-2">
                    {#each $timeSlots as slot}
                      <span class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium border {getTagColor(slot.tag, $timeSlots)}">
                        {getTagEmoji(slot.tag)} {slot.label}
                        <span class="opacity-60">({formatHourRange(slot.startHour, slot.endHour)})</span>
                      </span>
                    {/each}
                  </div>
                {/if}
              </div>

              <!-- Assign playlist to tag -->
              <div>
                <h4 class="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">Assign Playlist to Time Slot</h4>
                <div class="flex flex-col sm:flex-row gap-2">
                  <select bind:value={assignTag[ch.slug]} class="px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500">
                    {#each $timeSlots as slot}
                      <option value={slot.tag}>{getTagEmoji(slot.tag)} {slot.label} ({formatHourRange(slot.startHour, slot.endHour)})</option>
                    {/each}
                  </select>
                  <select bind:value={assignPlaylistId[ch.slug]} class="flex-1 px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500">
                    <option value={null}>Select a playlist...</option>
                    {#each allPlaylistsList as pl}
                      <option value={pl.id}>{pl.name} ({pl.trackCount} tracks)</option>
                    {/each}
                  </select>
                  <button type="button" class="px-4 py-2 text-sm font-semibold text-white bg-primary-600 hover:bg-primary-700 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed" onclick={() => handleAssignToTag(ch.slug)} disabled={assigningSlug === ch.slug}>
                    {assigningSlug === ch.slug ? 'Assigning...' : 'Assign'}
                  </button>
                </div>
              </div>

              <!-- Current assignments -->
              {#if channelMasterData[ch.slug] && Object.keys(channelMasterData[ch.slug]).length > 0}
                <div class="space-y-2">
                  {#each Object.entries(channelMasterData[ch.slug]) as [tag, data]}
                    {#if data.playlists && data.playlists.length > 0}
                      <div class="rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
                        <div class="px-3 py-2 bg-gray-100 dark:bg-gray-800 text-xs font-semibold text-gray-600 dark:text-gray-400 flex items-center gap-2">
                          <span>{getTagEmoji(tag)}</span>
                          <span>{getTagLabel(tag, $timeSlots)}</span>
                          <span class="opacity-60">({data.count} playlist{data.count !== 1 ? 's' : ''})</span>
                        </div>
                        <div class="divide-y divide-gray-100 dark:divide-gray-800">
                          {#each data.playlists as pl}
                            <div class="px-3 py-2 flex items-center justify-between hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors">
                              <span class="text-sm text-gray-700 dark:text-gray-300">{pl.name}</span>
                              <button type="button" class="p-1 rounded text-gray-400 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/30 dark:hover:text-red-400 transition-colors" title="Remove from this time slot" onclick={() => handleRemoveFromTag(ch.slug, tag, pl.id)}>
                                <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" /></svg>
                              </button>
                            </div>
                          {/each}
                        </div>
                      </div>
                    {/if}
                  {/each}
                </div>
              {/if}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<!-- Delete confirmation modal -->
{#if confirmDeleteSlug}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onclick={() => (confirmDeleteSlug = null)} onkeydown={(e) => { if (e.key === 'Escape') confirmDeleteSlug = null; }} role="button" tabindex="-1" aria-label="Close">
    <div class="bg-white dark:bg-gray-800 rounded-2xl shadow-2xl border border-gray-200 dark:border-gray-700 p-6 max-w-sm w-full mx-4" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()} role="dialog" aria-modal="true" tabindex="-1">
      <div class="text-center">
        <div class="w-12 h-12 mx-auto rounded-full bg-red-100 dark:bg-red-900/30 flex items-center justify-center mb-4">
          <svg class="w-6 h-6 text-red-500" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" /></svg>
        </div>
        <h3 class="text-lg font-bold text-gray-900 dark:text-white mb-2">Delete Channel?</h3>
        <p class="text-sm text-gray-500 dark:text-gray-400 mb-6">This action cannot be undone. The channel "{confirmDeleteSlug}" and all its schedule data will be removed.</p>
        <div class="flex gap-3 justify-center">
          <button type="button" class="px-5 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors" onclick={() => (confirmDeleteSlug = null)}>Cancel</button>
          <button type="button" class="px-5 py-2 text-sm font-semibold rounded-lg bg-red-600 text-white hover:bg-red-700 transition-colors" onclick={() => { if (confirmDeleteSlug !== null) handleDelete(confirmDeleteSlug); }}>Delete</button>
        </div>
      </div>
    </div>
  </div>
{/if}