/**
 * Fetch-boundary decoder for `GET /api/get_data`.
 *
 * Turns an unvalidated wire payload into either a fully typed `Envelope`
 * (success track) or throws a typed `ApiError` (malformed payload, service
 * error envelope, or unusable snapshot metadata). Every nested field the UI
 * reads — campuses, buildings, rooms, nodes, free_nodes/free_times,
 * partial_campus ids, the error object, and the date/deadline strings — is
 * validated here; no consumer casts after a shallow check.
 *
 * The wire shapes are mirrored in `./types` from `service/model/realtime_data.go`.
 */
import type {
  ApiError as ApiErrorInfo,
  BuildingInfo,
  CampusInfo,
  FreeTime,
  NodeInfo,
  RoomInfo,
  TodayClassroomsData,
} from "./types";
import { formatShanghaiDate } from "../classTimeUtils";

export const loadingResponse: Envelope = { code: 1, msg: "加载中", data: null };

export const fallbackErrorMessage = "数据获取失败，请稍后重试";

/** Post-validation envelope shape handed to the hook and the UI. */
export interface Envelope {
  code: number;
  msg: string;
  /** Running backend build version; absent when the server sent none. */
  version?: string;
  /** Request id (body `log_id`, else the X-Log-Id header) for the error card. */
  logId?: string;
  data: TodayClassroomsData | null;
}

/**
 * Structured fetch-boundary error. Keeps the real HTTP status, the business
 * envelope code, the server log_id and the running build version on the error
 * object so downstream code never has to guess 500 or parse them back out of
 * a message string.
 */
export interface ApiErrorOptions {
  status?: number;
  code?: number;
  logId?: string;
  version?: string;
}

export class ApiError extends Error {
  status?: number;
  code?: number;
  logId: string;
  /** Build version off the failure envelope; "" when the server sent none. */
  version: string;

  constructor(
    message: string,
    { status, code, logId, version }: ApiErrorOptions = {},
  ) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.logId = logId || "";
    this.version = version || "";
  }
}

type UnknownRecord = Record<string, unknown>;

function isRecord(value: unknown): value is UnknownRecord {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function readString(value: unknown): string | null {
  return typeof value === "string" ? value : null;
}

function readNumber(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

function decodeEachOrNull<T>(
  value: unknown,
  decode: (item: unknown) => T | null,
): T[] | null {
  if (!Array.isArray(value)) {
    return null;
  }
  const out: T[] = [];
  for (const item of value) {
    const decoded = decode(item);
    if (decoded === null) {
      return null;
    }
    out.push(decoded);
  }
  return out;
}

function decodeFreeTime(value: unknown): FreeTime | null {
  if (!isRecord(value)) {
    return null;
  }
  const node = readNumber(value.node);
  const time = readString(value.time);
  if (node === null || time === null) {
    return null;
  }
  return { node, time };
}

function decodeRoom(value: unknown): RoomInfo | null {
  if (!isRecord(value)) {
    return null;
  }
  const name = readString(value.name);
  const capacity = readNumber(value.capacity);
  const freeNodes = decodeEachOrNull(value.free_nodes, readNumber);
  if (name === null || capacity === null || freeNodes === null) {
    return null;
  }
  // Degraded parsing may yield capacity 0 (UI shows "未知"); that is valid.
  // Pre-0.3.x same-day caches may lack display_name — fall back to the raw
  // name instead of rejecting an otherwise usable payload.
  const displayName = readString(value.display_name) ?? name;
  const freeTimes = decodeEachOrNull(value.free_times, decodeFreeTime);
  if (freeTimes === null) {
    return null;
  }
  return {
    name,
    display_name: displayName,
    capacity,
    free_nodes: freeNodes,
    free_times: freeTimes,
  };
}

function decodeBuilding(value: unknown): BuildingInfo | null {
  if (!isRecord(value)) {
    return null;
  }
  const name = readString(value.name);
  if (name === null) {
    return null;
  }
  const rooms = decodeEachOrNull(value.rooms, decodeRoom);
  if (rooms === null) {
    return null;
  }
  return {
    name,
    display_name: readString(value.display_name) ?? name,
    rooms,
  };
}

function decodeNodeInfo(value: unknown): NodeInfo | null {
  if (!isRecord(value)) {
    return null;
  }
  const node = readNumber(value.node);
  const time = readString(value.time);
  const roomCount = readNumber(value.room_count);
  if (node === null || time === null || roomCount === null) {
    return null;
  }
  return { node, time, room_count: roomCount };
}

function decodeCampus(value: unknown): CampusInfo | null {
  if (!isRecord(value)) {
    return null;
  }
  const id = readString(value.id);
  const name = readString(value.name);
  if (id === null || name === null) {
    return null;
  }
  const buildings = decodeEachOrNull(value.buildings, decodeBuilding);
  const nodes = decodeEachOrNull(value.nodes, decodeNodeInfo);
  if (buildings === null || nodes === null) {
    return null;
  }
  return { id, name, buildings, nodes };
}

function decodePartialCampusIds(value: unknown): string[] | null {
  if (!Array.isArray(value)) {
    return null;
  }
  const out: string[] = [];
  for (const item of value) {
    if (typeof item !== "string" && typeof item !== "number") {
      return null;
    }
    const id = String(item).trim();
    if (id) {
      out.push(id);
    }
  }
  return out;
}

function decodeErrorInfo(value: unknown): ApiErrorInfo | null {
  if (!isRecord(value)) {
    return null;
  }
  const type = readString(value.type);
  const message = readString(value.message);
  if (type === null || message === null) {
    return null;
  }
  return { type, message };
}

function decodeSnapshotData(value: unknown): TodayClassroomsData {
  if (!isRecord(value)) {
    throw new ApiError("服务返回数据格式异常");
  }
  if (!Array.isArray(value.campuses)) {
    throw new ApiError("服务返回校区数据异常");
  }
  const date = readString(value.date);
  const updatedAt = readString(value.updated_at);
  const expiresAt = readString(value.expires_at);
  const staleUntil = readString(value.stale_until);
  if (
    date === null ||
    date.trim() === "" ||
    updatedAt === null ||
    expiresAt === null ||
    staleUntil === null ||
    !Number.isFinite(Date.parse(staleUntil))
  ) {
    throw new ApiError("服务返回数据格式异常");
  }
  if (typeof value.stale !== "boolean") {
    throw new ApiError("服务返回数据格式异常");
  }
  const campuses = decodeEachOrNull(value.campuses, decodeCampus);
  if (campuses === null) {
    throw new ApiError("服务返回校区数据异常");
  }
  const partialCampuses =
    value.partial_campuses === undefined
      ? undefined
      : decodePartialCampusIds(value.partial_campuses);
  if (partialCampuses === null) {
    throw new ApiError("服务返回数据格式异常");
  }
  const error = value.error === null ? null : decodeErrorInfo(value.error);
  if (value.error !== null && error === null) {
    throw new ApiError("服务返回数据格式异常");
  }
  return {
    date,
    updated_at: updatedAt,
    expires_at: expiresAt,
    stale_until: staleUntil,
    stale: value.stale,
    campuses,
    ...(partialCampuses ? { partial_campuses: partialCampuses } : {}),
    error,
  };
}

/**
 * Decode one `/api/get_data` JSON payload into a typed envelope.
 *
 * Throws `ApiError` with a user-safe Chinese reason for malformed payloads.
 * Legitimate non-zero service envelopes decode to `{ code, msg, data: null }`
 * (the hook decides whether they throw); success envelopes get a deeply
 * validated snapshot body.
 */
export function decodeGetData(payload: unknown): Envelope {
  if (!isRecord(payload)) {
    throw new ApiError("服务返回格式异常");
  }
  const code = readNumber(payload.code);
  if (code === null) {
    throw new ApiError("服务返回状态异常");
  }

  const msg = (readString(payload.msg) ?? "").trim();
  const version = (readString(payload.version) ?? "").trim();
  const logId = (readString(payload.log_id) ?? "").trim();
  // Written only when present: absent beats a bare "" so the envelope shape
  // is unchanged for pre-0.3.0 backends (the settings row stays hidden).
  const versionField = version ? { version } : {};
  const logIdField = logId ? { logId } : {};

  if (code !== 0) {
    return {
      code,
      msg: msg || fallbackErrorMessage,
      ...versionField,
      ...logIdField,
      data: null,
    };
  }
  return {
    code: 0,
    msg,
    ...versionField,
    ...logIdField,
    data: decodeSnapshotData(payload.data),
  };
}

/** Body parse that fails closed: unreadable JSON decodes to null. */
export async function readJson(response: Response): Promise<unknown> {
  try {
    return await response.json();
  } catch {
    return null;
  }
}

/**
 * True when a decoded snapshot is still safe to display for the current
 * Asia/Shanghai business day: `date` matches today and `stale_until` is
 * still in the future.
 */
export function isUsableBusinessDaySnapshot(
  data: TodayClassroomsData | null | undefined,
  nowMs: number = Date.now(),
): boolean {
  return (
    data != null &&
    data.date === formatShanghaiDate(new Date(nowMs)) &&
    nowMs < Date.parse(data.stale_until)
  );
}

/**
 * Warning text for the stale/partial Alert. Fully typed input: only decoded
 * snapshots reach the UI, so no defensive guards are needed here.
 */
export function classroomWarningMessage(
  data?: TodayClassroomsData | null,
): string {
  const fallback =
    (data?.error?.message.trim() ?? "") ||
    "当前展示的是今天最后一次成功刷新数据";
  const ids = [
    ...new Set(
      (data?.partial_campuses ?? [])
        .map((id) => id.trim())
        .filter((id) => id !== ""),
    ),
  ];
  if (ids.length === 0) {
    return fallback;
  }
  const campuses = data?.campuses ?? [];
  const labels = ids.map((id) => {
    const campus = campuses.find((item) => item.id === id);
    const name = campus?.name.trim() ?? "";
    return name && name !== id ? `${name}（${id}）` : id;
  });
  return `受影响校区：${labels.join("、")}。${fallback}`;
}
