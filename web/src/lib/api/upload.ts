import { ApiError, getToken } from "./client";
import type { Track } from "./tracks";

// ---------------------------------------------------------------------------
// Track upload
// ---------------------------------------------------------------------------

export interface UploadMeta {
    title?: string;
    artist?: string;
    album?: string;
    genre?: string;
}

export interface UploadOptions {
    onProgress?: (percent: number) => void;
    meta?: UploadMeta;
    optimize?: boolean;
}

export interface UploadResult {
    added: boolean;
    track: Track;
}

export function uploadTrack(
    file: File,
    { onProgress, meta, optimize = true }: UploadOptions = {},
): Promise<UploadResult> {
    return new Promise((resolve, reject) => {
        const token = getToken();
        const form = new FormData();
        form.append("file", file);

        // Append the optimize flag (defaults to true).
        form.append("optimize", optimize ? "true" : "false");

        // Append any provided metadata fields so the server can override
        // the embedded audio tags and derive the on-disk filename from the title.
        if (meta?.title?.trim())  form.append("title",  meta.title.trim());
        if (meta?.artist?.trim()) form.append("artist", meta.artist.trim());
        if (meta?.album?.trim())  form.append("album",  meta.album.trim());
        if (meta?.genre?.trim())  form.append("genre",  meta.genre.trim());

        const xhr = new XMLHttpRequest();

        xhr.upload.addEventListener("progress", (e) => {
            if (e.lengthComputable && onProgress) {
                onProgress(Math.round((e.loaded / e.total) * 100));
            }
        });

        xhr.addEventListener("load", () => {
            let envelope: { status?: string; data?: unknown; error?: string | { message?: string; code?: string } };
            try {
                envelope = JSON.parse(xhr.responseText);
            } catch {
                reject(new ApiError("Invalid server response", xhr.status, "invalid_response", null));
                return;
            }
            if (xhr.status === 401) {
                window.dispatchEvent(new CustomEvent("auth:unauthorized"));
            }
            if (xhr.status >= 200 && xhr.status < 300 && envelope.status === "ok") {
                resolve(envelope.data as UploadResult);
            } else {
                const errField = envelope?.error;
                let msg = xhr.statusText;
                let code = "unknown";
                if (typeof errField === "string") {
                    msg = errField;
                    code = "error";
                } else if (errField && typeof errField === "object") {
                    msg = errField.message ?? xhr.statusText;
                    code = errField.code ?? "error";
                }
                reject(new ApiError(msg, xhr.status, code, envelope));
            }
        });

        xhr.addEventListener("error", () => {
            reject(new ApiError("Network error during upload", 0, "network_error", null));
        });

        xhr.addEventListener("abort", () => {
            reject(new ApiError("Upload was cancelled", 0, "aborted", null));
        });

        xhr.open("POST", "/api/tracks/upload");
        if (token) {
            xhr.setRequestHeader("Authorization", `Bearer ${token}`);
        }
        xhr.send(form);
    });
}
