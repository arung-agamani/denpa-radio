const BASE = "";

function getToken(): string | null {
    return localStorage.getItem("dj_token");
}

function authHeaders(): Record<string, string> {
    const token = getToken();
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (token) {
        headers["Authorization"] = `Bearer ${token}`;
    }
    return headers;
}

interface RequestOptions {
    noAuth?: boolean;
    raw?: boolean;
    rawBody?: string;
    formData?: FormData;
}

interface ApiResponseEnvelope<T> {
    status: "ok";
    data: T;
}

interface ApiErrorEnvelope {
    status: "error";
    error: {
        code: string;
        message: string;
    } | string;
}

async function request<T = unknown>(
    method: string,
    path: string,
    body: unknown = null,
    options: RequestOptions = {},
): Promise<T> {
    const config: RequestInit & { headers: Record<string, string> } = {
        method,
        headers: options.noAuth
            ? { "Content-Type": "application/json" }
            : authHeaders(),
    };

    if (body !== null) {
        if (
            body instanceof Blob ||
            body instanceof ArrayBuffer ||
            typeof body === "string"
        ) {
            config.body = body as BodyInit;
        } else {
            config.body = JSON.stringify(body);
        }
    }

    if (options.rawBody) {
        config.body = options.rawBody;
        if (!options.noAuth) {
            config.headers = { ...authHeaders() };
        }
        delete config.headers["Content-Type"];
    }

    if (options.formData) {
        config.body = options.formData;
        const token = getToken();
        const headers: Record<string, string> = {};
        if (token && !options.noAuth) {
            headers["Authorization"] = `Bearer ${token}`;
        }
        config.headers = headers;
    }

    const res = await fetch(`${BASE}${path}`, config);

    if (res.status === 401) {
        const event = new CustomEvent("auth:unauthorized");
        window.dispatchEvent(event);
    }

    if (options.raw) {
        return res as unknown as T;
    }

    const bodyJson = await res.json();

    if (!res.ok || bodyJson.status === "error") {
        const err = bodyJson.error as ApiErrorEnvelope["error"] | undefined;
        let message = res.statusText;
        let code = "unknown";
        if (typeof err === "string") {
            message = err;
            code = "error";
        } else if (err && typeof err === "object") {
            message = err.message || res.statusText;
            code = err.code || "error";
        }
        throw new ApiError(message, res.status, code, bodyJson);
    }

    return (bodyJson as ApiResponseEnvelope<T>).data;
}

export class ApiError extends Error {
    status: number;
    code: string;
    data: unknown;

    constructor(message: string, status: number, code: string, data: unknown) {
        super(message);
        this.name = "ApiError";
        this.status = status;
        this.code = code;
        this.data = data;
    }
}

export type { RequestOptions };
export { getToken, authHeaders, request };
