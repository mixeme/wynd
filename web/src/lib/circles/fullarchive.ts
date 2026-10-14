import { apiJson, apiPath } from "$lib/api/client";
import type { ArchiveKey } from "$lib/archive/fullkey";

/** Полный архив круга (план 51): что с ним на сервере. */
export interface FullArchive {
  id: string;
  status: "building" | "ready" | "failed" | "removed";
  requested_at: string;
  snapshot_at?: string;
  ready_at?: string;
  expires_at?: string;
  downloaded_at?: string;
  removed_at?: string;
  removed_reason?: "downloaded" | "expired" | "admin" | "owner" | "canceled";
  delete_circle: boolean;
  encrypted: boolean;
  size_bytes: number;
}

export interface FullArchiveState {
  estimate_bytes: number;
  posts: number;
  members: number;
  /** Готовый файл помещается на сервер; нет — 6.43, «Скачать сразу». */
  fits: boolean;
  retention_hours: number;
  archive: FullArchive | null;
}

export function fetchFullArchive(
  origin: string,
  circleId: string,
): Promise<FullArchiveState> {
  return apiJson<FullArchiveState>(origin, `/circles/${circleId}/full_archive`);
}

/** «Собрать архив» (6.29). Ключ — открытый ключ и соль, пароль остаётся тут. */
export function startFullArchive(
  origin: string,
  circleId: string,
  key: ArchiveKey | undefined,
): Promise<FullArchive> {
  return apiJson<FullArchive>(origin, `/circles/${circleId}/full_archive`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ key }),
  });
}

/** «Отменить сборку» (6.40) и «Удалить архив, не скачивая» (6.33). */
export async function removeFullArchive(
  origin: string,
  circleId: string,
): Promise<void> {
  await apiJson(origin, `/circles/${circleId}/full_archive`, {
    method: "DELETE",
  });
}

function openDownload(origin: string, url: string) {
  const a = document.createElement("a");
  a.href = apiPath(origin, url.replace(/^\/api\/v1/, ""));
  a.rel = "noopener";
  a.click();
}

/** Скачать готовый файл: качает сам браузер, с ходом и докачкой. */
export async function downloadFullArchive(
  origin: string,
  circleId: string,
): Promise<void> {
  const { url } = await apiJson<{ url: string }>(
    origin,
    `/circles/${circleId}/full_archive/link`,
    {
      method: "POST",
    },
  );
  openDownload(origin, url);
}

/** «Скачать сразу» (6.43): сервер собирает файл прямо в скачивание. */
export async function streamFullArchive(
  origin: string,
  circleId: string,
  key: ArchiveKey | undefined,
): Promise<void> {
  const { url } = await apiJson<{ url: string }>(
    origin,
    `/circles/${circleId}/full_archive/stream_link`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key }),
    },
  );
  openDownload(origin, url);
}
