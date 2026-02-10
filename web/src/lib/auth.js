import { writable, derived } from 'svelte/store';
import { verifyToken, logout as apiLogout, isLoggedIn, getUsername } from './api.js';

// Reactive store for authentication state.
function createAuthStore() {
  const { subscribe, set, update } = writable({
    authenticated: isLoggedIn(),
    username: getUsername(),
    checking: false,
  });

  return {
    subscribe,

    // Call after a successful login to update the store.
    setLoggedIn(username) {
      set({ authenticated: true, username, checking: false });
    },

    // Call to log out and clear stored credentials.
    logout() {
      apiLogout();
      set({ authenticated: false, username: '', checking: false });
    },

    // Verify the current token against the server. Updates the store
    // to reflect whether the token is still valid.
    async verify() {
      if (!isLoggedIn()) {
        set({ authenticated: false, username: '', checking: false });
        return false;
      }

      update((s) => ({ ...s, checking: true }));

      try {
        await verifyToken();
        update((s) => ({ ...s, authenticated: true, checking: false }));
        return true;
      } catch {
        apiLogout();
        set({ authenticated: false, username: '', checking: false });
        return false;
      }
    },
  };
}

export const auth = createAuthStore();

// Derived convenience store: true when the user is authenticated.
export const isAuthenticated = derived(auth, ($auth) => $auth.authenticated);

// Listen for 401 responses from the API layer and auto-logout.
if (typeof window !== 'undefined') {
  window.addEventListener('auth:unauthorized', () => {
    auth.logout();
  });
}
