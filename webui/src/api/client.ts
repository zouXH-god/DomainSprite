import { useSessionStore } from "../stores/session";
export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public requestId: string,
  ) {
    super(message);
  }
}
type Envelope<T> = { code: number; message: string; data: T };
export async function api<T>(
  url: string,
  init: RequestInit = {},
  retry = true,
): Promise<T> {
  const session = useSessionStore();
  const requestId = crypto.randomUUID();
  const headers = new Headers(init.headers);
  headers.set("X-Request-ID", requestId);
  if (session.csrfToken && (init.method ?? "GET") !== "GET")
    headers.set("X-CSRF-Token", session.csrfToken);
  if (init.body && !headers.has("Content-Type"))
    headers.set("Content-Type", "application/json");
  let response: Response;
  try {
    response = await fetch(url, { ...init, headers, credentials: "include" });
  } catch (e) {
    if ((init.method ?? "GET") === "GET" && retry)
      return api<T>(url, init, false);
    throw new ApiError(0, "无法连接到 DomainSprite", requestId);
  }
  if (response.status === 401 || response.status === 403) session.clear();
  let body: Envelope<T>;
  try {
    body = await response.json();
  } catch {
    throw new ApiError(
      response.status,
      "服务器返回了无法解析的响应",
      requestId,
    );
  }
  if (!response.ok || body.code >= 400)
    throw new ApiError(response.status, body.message || "请求失败", requestId);
  return body.data;
}
export async function download(url: string) {
  const session = useSessionStore();
  const response = await fetch(url, {
    credentials: "include",
    headers: { "X-CSRF-Token": session.csrfToken },
  });
  if (!response.ok) throw new ApiError(response.status, "下载失败", "");
  const blob = await response.blob();
  const cd = response.headers.get("content-disposition") || "";
  const name = /filename="?([^";]+)"?/.exec(cd)?.[1] || "download";
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = name;
  a.click();
  URL.revokeObjectURL(a.href);
}
