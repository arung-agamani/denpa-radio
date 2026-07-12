import { request, getToken } from "./client";

// ---------------------------------------------------------------------------
// Auth
// ---------------------------------------------------------------------------

export interface LoginResponse {
    token: string;
    username: string;
}

export async function login(username: string, password: string): Promise<LoginResponse> {
    const data = await request<LoginResponse>(
        "POST",
        "/api/auth/login",
        { username, password },
        { noAuth: true },
    );
    if (data.token) {
        localStorage.setItem("dj_token", data.token);
        localStorage.setItem("dj_username", data.username);
    }
    return data;
}

export async function verifyToken(): Promise<unknown> {
    return request("GET", "/api/auth/verify");
}

export function logout(): void {
    localStorage.removeItem("dj_token");
    localStorage.removeItem("dj_username");
}

export function isLoggedIn(): boolean {
    return !!getToken();
}

export function getUsername(): string {
    return localStorage.getItem("dj_username") || "";
}
