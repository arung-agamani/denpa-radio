<script lang="ts">
  import { onMount } from 'svelte';
  import { path, segment } from './lib/router';
  import { auth, isAuthenticated } from './lib/auth';
  import { status, toasts } from './lib/stores';
  import Public from './routes/Public.svelte';
  import DJ from './routes/DJ.svelte';
  import Login from './routes/Login.svelte';
  import Navbar from './components/Navbar.svelte';
  import Toast from './components/Toast.svelte';

  let ready = false;

  onMount(() => {
    // Start polling radio status.
    status.startPolling(5000);

    // Verify existing token if present.
    if ($isAuthenticated) {
      auth.verify();
    }

    ready = true;

    return () => {
      status.stopPolling();
    };
  });

  $: isDJRoute = $segment === 'dj';
  $: isDJLogin = $path === '/dj/login' || $path === '/dj/login/';
  $: needsAuth = isDJRoute && !isDJLogin && !$isAuthenticated;
</script>

<div class="min-h-screen flex flex-col bg-gray-50 dark:bg-gray-900">
  <Navbar />

  <main class="flex-1">
    {#if !ready}
      <div class="flex items-center justify-center h-64">
        <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-500"></div>
      </div>
    {:else if isDJLogin || needsAuth}
      <Login />
    {:else if isDJRoute}
      <DJ />
    {:else}
      <Public />
    {/if}
  </main>

  <footer class="py-4 text-center text-sm text-gray-500 dark:text-gray-400 border-t border-gray-200 dark:border-gray-700">
    <p>Denpa Radio &mdash; Powered by love and denpa waves 📻</p>
  </footer>
</div>

<!-- Toast notifications -->
<div class="fixed bottom-4 right-4 z-50 flex flex-col gap-2 max-w-sm w-full pointer-events-none">
  {#each $toasts as toast (toast.id)}
    <Toast {toast} on:dismiss={() => toasts.remove(toast.id)} />
  {/each}
</div>
