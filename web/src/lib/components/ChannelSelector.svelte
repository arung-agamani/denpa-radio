<script lang="ts">
  import { channels, selectedChannel } from '$lib/stores';

  let channelList = $derived($channels);
  let currentSlug = $derived($selectedChannel);

  function handleChange(e: Event) {
    const target = e.target as HTMLSelectElement;
    selectedChannel.set(target.value);
  }
</script>

{#if channelList.length > 1}
  <div class="bg-white dark:bg-gray-800 rounded-2xl shadow-lg border border-gray-200 dark:border-gray-700 p-4">
    <label for="channel-select" class="block text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400 mb-2">
      Channel
    </label>
    <select
      id="channel-select"
      class="w-full rounded-lg border-gray-300 dark:border-gray-600 dark:bg-gray-700 text-gray-900 dark:text-white text-sm font-medium focus:ring-2 focus:ring-primary-500 focus:border-primary-500 transition-colors"
      value={currentSlug}
      onchange={handleChange}
    >
      {#each channelList as channel (channel.slug)}
        <option value={channel.slug} disabled={!channel.enabled}>
          {channel.name}{#if !channel.enabled} (offline){/if}
        </option>
      {/each}
    </select>
  </div>
{/if}