const API_BASE = import.meta.env.VITE_CORAZA_API_BASE || "/mamotama-api";
const API_KEY = import.meta.env.VITE_API_KEY || "";

function withKey(headers: Headers) {
    if (API_KEY) {
        headers.set("X-API-Key", API_KEY);
    }
}

export async function apiGetText(path: string, init: RequestInit = {}) {
    const headers = new Headers(init.headers || {});
    withKey(headers);

    const res = await fetch(`${API_BASE}${path}`, {
        ...init,
        headers,
        credentials: "include"
    });

    if (! res.ok) {
        throw new Error(`HTTP ${res.status}`);
    }

    return res.text();
}

export async function apiGetJson<T = any>(path: string, init: RequestInit = {}) {
    const headers = new Headers(init.headers || {});
    withKey(headers);

    const res = await fetch(`${API_BASE}${path}`, {
        ...init,
        headers,
        credentials: 'include'
    });
    const data = await res.json().catch(() => ({}));

    if (! res.ok) {
        throw new Error((data as any)?.error || `HTTP ${res.status}`);
    }

    return data as T;
}

export async function apiPostText(path: string, body: string, init: RequestInit = {}) {
    const headers = new Headers(init.headers || {});
    headers.set("Content-Type", "text/plain");
    withKey(headers);

    const res = await fetch(`${API_BASE}${path}`, {
        method: "POST",
        body, ...init,
        headers,
        credentials: "include"
    });

    if (! res.ok) {
        throw new Error(`HTTP ${res.status}`);
    }

    return res;
}
