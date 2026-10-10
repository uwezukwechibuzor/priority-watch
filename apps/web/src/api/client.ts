const BASE = import.meta.env.VITE_API_BASE ?? "";

class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { "Content-Type": "application/json", ...init?.headers },
    ...init,
  });
  if (!res.ok) {
    let msg = res.statusText;
    try {
      const body = (await res.json()) as { detail?: string; message?: string };
      msg = body.detail ?? body.message ?? msg;
    } catch {
      // ignore parse failure
    }
    throw new ApiError(res.status, msg);
  }
  return res.json() as Promise<T>;
}

export { apiFetch, ApiError };
