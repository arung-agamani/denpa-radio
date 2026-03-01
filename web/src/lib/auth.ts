import { writable, derived } from 'svelte/store';
import { verifyToken, logout as apiLogout, isLoggedIn, getUsername } from './api';

export interface AuthState {
    authenticated: boolean;
    username: string;
    checking: boolean;
}

// Reactive store for authentication state.
function createAuthStore() {
    const { subscribe, set, update } = writable<AuthState>({
        authenticated: isLoggedIn(),
        username: getUsername(),
        checking: false,
    });

    return {
        subscribe,

        // Call after a successful login to update the store.
        setLoggedIn(username: string): void {
            set({ authenticated: true, username, checking: false });
        },

        // Call to log out and clear stored credentials.
        logout(): void {
            apiLogout();
            set({ authenticated: false, username: '', checking: false });
        },

        // Verify the current token against the server. Updates the store
        // to reflect whether the token is still valid.
        async verify(): Promise<boolean> {
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

export const isAuthenticated = derived(auth, ($auth) => $auth.authenticated);
