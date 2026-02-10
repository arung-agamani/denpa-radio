<script>
  import { createEventDispatcher } from 'svelte';
  import { fade, fly } from 'svelte/transition';

  export let toast;

  const dispatch = createEventDispatcher();

  const icons = {
    success: '✓',
    error: '✕',
    warning: '⚠',
    info: 'ℹ',
  };

  const colors = {
    success: 'bg-green-100 text-green-800 dark:bg-green-800 dark:text-green-100 border-green-300 dark:border-green-600',
    error: 'bg-red-100 text-red-800 dark:bg-red-800 dark:text-red-100 border-red-300 dark:border-red-600',
    warning: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-800 dark:text-yellow-100 border-yellow-300 dark:border-yellow-600',
    info: 'bg-blue-100 text-blue-800 dark:bg-blue-800 dark:text-blue-100 border-blue-300 dark:border-blue-600',
  };

  const iconBg = {
    success: 'bg-green-200 text-green-600 dark:bg-green-700 dark:text-green-200',
    error: 'bg-red-200 text-red-600 dark:bg-red-700 dark:text-red-200',
    warning: 'bg-yellow-200 text-yellow-600 dark:bg-yellow-700 dark:text-yellow-200',
    info: 'bg-blue-200 text-blue-600 dark:bg-blue-700 dark:text-blue-200',
  };

  $: type = toast.type || 'info';
  $: colorClass = colors[type] || colors.info;
  $: iconBgClass = iconBg[type] || iconBg.info;
  $: icon = icons[type] || icons.info;
</script>

<div
  class="pointer-events-auto flex items-center gap-3 px-4 py-3 rounded-lg border shadow-lg {colorClass}"
  role="alert"
  in:fly={{ x: 100, duration: 300 }}
  out:fade={{ duration: 200 }}
>
  <div class="flex-shrink-0 w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold {iconBgClass}">
    {icon}
  </div>

  <div class="flex-1 text-sm font-medium">
    {toast.message}
  </div>

  <button
    type="button"
    class="flex-shrink-0 ml-2 inline-flex items-center justify-center w-6 h-6 rounded-full opacity-60 hover:opacity-100 transition-opacity focus:outline-none"
    on:click={() => dispatch('dismiss')}
    aria-label="Close"
  >
    <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 14 14">
      <path stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="m1 1 6 6m0 0 6 6M7 7l6-6M7 7l-6 6" />
    </svg>
  </button>
</div>
