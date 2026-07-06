<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { trackLibrary, toasts, playlists, master } from '$lib/stores';
  import { scanTracks, reconcile, enrichLibrary } from '$lib/api';

  let { children } = $props();

  const tabs = [
    { id: 'albums', label: 'Albums', icon: '💿', href: '/dj/library/albums' },
    { id: 'artists', label: 'Artists', icon: '🎤', href: '/dj/library/artists' },
    { id: 'genres', label: 'Genres', icon: '🏷️', href: '/dj/library/genres' },
  ];

  let currentPath = $derived($page.url.pathname);
  let activeTab = $derived(
    tabs.find(t => currentPath === t.href || currentPath.startsWith(t.href + '/'))?.id || 'albums'
  );
  let totalTracks = $derived($trackLibrary.length);

  let scanning = $state(false);
  let reconciling = $state(false);
  let enriching = $state(false);

  async function handleScan() {
    scanning = true;
    try {
      const data = await scanTracks();
      toasts.success(`Scan complete: ${data.newly_added} new track(s) added. Library total: ${data.library_total}.`);
      await trackLibrary.refresh();
    } catch (err) {
      toasts.error('Scan failed: ' + (err instanceof Error ? err.message : String(err)));
    } finally {
      scanning = false;
    }
  }

  async function handleReconcile() {
    reconciling = true;
    try {
      const data = await reconcile();
      toasts.success(`Reconciled: ${data.removed_count} removed, ${data.orphaned_count} new files found.`);
      await playlists.refresh();
      await master.refresh();
      await trackLibrary.refresh();
    } catch (err) {
      toasts.error('Reconcile failed: ' + (err instanceof Error ? err.message : String(err)));
    } finally {
      reconciling = false;
    }
  }

  async function handleEnrichAll() {
    if (!confirm('Enrich all tracks without cover art? This may take a while.')) {
      return;
    }
    enriching = true;
    try {
      const data = await enrichLibrary();
      toasts.success(`Enrichment complete: ${data.art_found} of ${data.tracks_attempted} tracks updated.`);
      await trackLibrary.refresh();
    } catch (err) {
      toasts.error('Enrichment failed: ' + (err instanceof Error ? err.message : String(err)));
    } finally {
      enriching = false;
    }
  }
</script>

<div class="space-y-6">
  <!-- Header -->
  <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
    <h1 class="text-2xl font-bold text-gray-900 dark:text-white">📚 Library</h1>
    <div class="flex items-center gap-2">
      <span class="text-xs text-gray-400 dark:text-gray-500 bg-gray-100 dark:bg-gray-800 px-2.5 py-1 rounded-full font-medium">
        {totalTracks} track{totalTracks !== 1 ? 's' : ''}
      </span>
    </div>
  </div>

  <div class="flex flex-wrap gap-2">
    <a
      href="/dj/tracks"
      class="px-3 py-1.5 text-xs font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors flex items-center gap-1.5"
    >
      📚 Track Library (legacy)
    </a>
    <button
      type="button"
      class="px-3 py-1.5 text-xs font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors flex items-center gap-1.5 disabled:opacity-50"
      onclick={handleScan}
      disabled={scanning}
    >
      {#if scanning}<svg class="animate-spin w-3.5 h-3.5" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" /><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" /></svg>{/if}
      🔍 Scan
    </button>
    <button
      type="button"
      class="px-3 py-1.5 text-xs font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors flex items-center gap-1.5 disabled:opacity-50"
      onclick={handleReconcile}
      disabled={reconciling}
    >
      {#if reconciling}<svg class="animate-spin w-3.5 h-3.5" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" /><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" /></svg>{/if}
      🔄 Reconcile
    </button>
    <button
      type="button"
      class="px-3 py-1.5 text-xs font-medium rounded-lg border border-primary-300 dark:border-primary-800 text-primary-700 dark:text-primary-300 hover:bg-primary-50 dark:hover:bg-primary-900/20 transition-colors flex items-center gap-1.5 disabled:opacity-50"
      onclick={handleEnrichAll}
      disabled={enriching}
    >
      {#if enriching}<svg class="animate-spin w-3.5 h-3.5" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" /><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" /></svg>{/if}
      ✨ Enrich All
    </button>
  </div>

  <div class="flex gap-1 border-b border-gray-200 dark:border-gray-700">
    {#each tabs as tab}
      <button
        type="button"
        class="px-4 py-2.5 text-sm font-medium transition-colors border-b-2 -mb-px {activeTab === tab.id
          ? 'border-primary-500 text-primary-600 dark:text-primary-400'
          : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'}"
        onclick={() => goto(tab.href)}
      >
        <span class="mr-1.5">{tab.icon}</span>
        {tab.label}
      </button>
    {/each}
  </div>

  <!-- Content -->
  <div>
    {@render children()}
  </div>
</div>
