export class ApiError extends Error {
  constructor(public status: number, public url: string) {
    super(`request to ${url} failed with status ${status}`);
    this.name = 'ApiError';
  }
}

export async function fetchJson<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init);
  if (!res.ok) {
    throw new ApiError(res.status, url);
  }
  return res.json() as Promise<T>;
}
