<script>
  import { path, navigate } from '../lib/router.js';
  import { auth, isAuthenticated } from '../lib/auth.js';
  import { stationName, activeClients } from '../lib/stores.js';

  let mobileMenuOpen = false;

  function toggleMobile() {
    mobileMenuOpen = !mobileMenuOpen;
  }

  function go(to) {
    navigate(to);
    mobileMenuOpen = false;
  }

  function handleLogout() {
    auth.logout();
    navigate('/');
    mobileMenuOpen = false;
  }

  $: isHome = $path === '/' || $path === '';
  $: isDJ = $path.startsWith('/dj');
</script>

<nav class="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 sticky top-0 z-40">
  <div class="max-w-screen-xl mx-auto px-4 sm:px-6 lg:px-8">
    <div class="flex items-center justify-between h-16">
      <!-- Brand -->
      <button
        type="button"
        class="flex items-center gap-2 text-xl font-bold text-primary-600 dark:text-primary-400 hover:opacity-80 transition-opacity"
        on:click={() => go('/')}
      >
        <span class="text-2xl">📻</span>
        <span class="hidden sm:inline">{$stationName}</span>
        <span class="sm:hidden">Denpa Radio</span>
      </button>

      <!-- Desktop nav -->
      <div class="hidden md:flex items-center gap-1">
        <button
          type="button"
          class="px-4 py-2 rounded-lg text-sm font-medium transition-colors {isHome
            ? 'bg-primary-100 text-primary-700 dark:bg-primary-900 dark:text-primary-300'
            : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'}"
          on:click={() => go('/')}
        >
          🎵 Listen
        </button>

        <button
          type="button"
          class="px-4 py-2 rounded-lg text-sm font-medium transition-colors {isDJ
            ? 'bg-primary-100 text-primary-700 dark:bg-primary-900 dark:text-primary-300'
            : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'}"
          on:click={() => go('/dj')}
        >
          🎛️ DJ Panel
        </button>

        <!-- Listeners badge -->
        <div class="ml-3 flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-gray-100 dark:bg-gray-700 text-xs text-gray-600 dark:text-gray-300">
          <span class="relative flex h-2 w-2">
            <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75"></span>
            <span class="relative inline-flex rounded-full h-2 w-2 bg-green-500"></span>
          </span>
          {$activeClients} listener{$activeClients !== 1 ? 's' : ''}
        </div>

        <!-- Auth status -->
        {#if $isAuthenticated}
          <button
            type="button"
            class="ml-2 px-3 py-1.5 rounded-lg text-xs font-medium text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
            on:click={handleLogout}
          >
            Logout
          </button>
        {/if}
      </div>

      <!-- Mobile menu button -->
      <button
        type="button"
        class="md:hidden inline-flex items-center justify-center p-2 rounded-lg text-gray-500 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-primary-500"
        on:click={toggleMobile}
        aria-expanded={mobileMenuOpen}
        aria-label="Toggle navigation"
      >
        {#if mobileMenuOpen}
          <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        {:else}
          <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5" />
          </svg>
        {/if}
      </button>
    </div>
  </div>

  <!-- Mobile menu -->
  {#if mobileMenuOpen}
    <div class="md:hidden border-t border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800">
      <div class="px-4 py-3 space-y-1">
        <button
          type="button"
          class="w-full text-left px-4 py-2.5 rounded-lg text-sm font-medium transition-colors {isHome
            ? 'bg-primary-100 text-primary-700 dark:bg-primary-900 dark:text-primary-300'
            : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'}"
          on:click={() => go('/')}
        >
          🎵 Listen
        </button>

        <button
          type="button"
          class="w-full text-left px-4 py-2.5 rounded-lg text-sm font-medium transition-colors {isDJ
            ? 'bg-primary-100 text-primary-700 dark:bg-primary-900 dark:text-primary-300'
            : 'text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'}"
          on:click={() => go('/dj')}
        >
          🎛️ DJ Panel
        </button>

        <div class="flex items-center justify-between px-4 py-2 text-xs text-gray-500 dark:text-gray-400">
          <div class="flex items-center gap-1.5">
            <span class="relative flex h-2 w-2">
              <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-green-400 opacity-75"></span>
              <span class="relative inline-flex rounded-full h-2 w-2 bg-green-500"></span>
            </span>
            {$activeClients} listener{$activeClients !== 1 ? 's' : ''}
          </div>

          {#if $isAuthenticated}
            <button
              type="button"
              class="text-xs font-medium text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200"
              on:click={handleLogout}
            >
              Logout
            </button>
          {/if}
        </div>
      </div>
    </div>
  {/if}
</nav>
