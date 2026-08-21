import type { TodayClassroomsData } from "./api/types";

export const loadingResponse: Envelope = { code: 1, msg: "加载中", data: null };

export const fallbackErrorMessage = "数据获取失败，请稍后重试";

/**
 * Post-validation envelope shape. `msg` is always present here (the wire
 * omits it on success); `data` shape is only fully trusted once
 * `isUsableBusinessDaySnapshot` passes (see parent design D3).
 */
export interface Envelope {
  code: number;
  msg: string;
  data: TodayClassroomsData | null;
}

export type NormalizeResult =
  | { ok: true; resp: Envelope }
  | { ok: false; reason: string };

/** Structural view this module reads off an unvalidated payload. */
export interface PayloadView {
  code?: unknown;
  msg?: unknown;
  log_id?: string;
  error?: { message?: unknown } | null;
  partial_campuses?: unknown;
  campuses?: unknown;
  [key: string]: unknown;
}

export function classroomWarningMessage(data?: PayloadView | null): string {
  const fallback =
    (typeof data?.error?.message === "string" &&
    data.error.message.trim() !== ""
      ? data.error.message.trim()
      : "") || "当前展示的是今天最后一次成功刷新数据";
  const ids = Array.isArray(data?.partial_campuses)
    ? [
        ...new Set(
          data.partial_campuses
            .filter(
              (value) => typeof value === "string" || typeof value === "number"
            )
            .map((value) => String(value).trim())
            .filter(Boolean)
        ),
      ]
    : [];
  if (ids.length === 0) {
    return fallback;
  }

  const campuses = Array.isArray(data?.campuses) ? data.campuses : [];
  const labels = ids.map((id) => {
    const campus = campuses.find((item) => String(item?.id) === id);
    const name =
      typeof campus?.name === "string" ? campus.name.trim() : "";
    return name && name !== id ? `${name}（${id}）` : id;
  });
  return `受影响校区：${labels.join("、")}。${fallback}`;
}

export function extractMessage(payload?: PayloadView | null): string {
  return typeof payload?.msg === "string" && payload.msg.trim() !== ""
    ? payload.msg.trim()
    : "";
}

/**
 * Discriminated result, no throw-as-control-flow: `{ ok: true, resp }` for a
 * safe envelope (including legitimate non-zero service envelopes), or
 * `{ ok: false, reason }` for malformed payloads. Throwing is the caller's
 * (fetch boundary) decision, not this parser's.
 */
export function normalizeResponse(payload: unknown): NormalizeResult {
  const view = payload as PayloadView | null | undefined;

  if (!view || typeof view !== "object") {
    return { ok: false, reason: "服务返回格式异常" };
  }

  const code = Number(view.code);
  if (!Number.isFinite(code)) {
    return { ok: false, reason: "服务返回状态异常" };
  }

  if (code !== 0) {
    return {
      ok: true,
      resp: {
        code,
        msg: extractMessage(view) || fallbackErrorMessage,
        data: null,
      },
    };
  }

  if (!view.data || typeof view.data !== "object") {
    return { ok: false, reason: "服务返回数据格式异常" };
  }
  if (!Array.isArray((view.data as PayloadView).campuses)) {
    return { ok: false, reason: "服务返回校区数据异常" };
  }

  const data = view.data as PayloadView & object;
  return {
    ok: true,
    resp: {
      code: 0,
      msg: extractMessage(view),
      // Envelope-level fields are validated above; the snapshot body is
      // re-validated by isUsableBusinessDaySnapshot before the UI trusts it.
      data: { ...data, campuses: data.campuses } as TodayClassroomsData,
    },
  };
}

export async function readJson(response: Response): Promise<unknown> {
  try {
    return await response.json();
  } catch {
    return null;
  }
}
