/**
 * Typed mirror of the backend's public API shapes.
 *
 * SOURCE OF TRUTH: `service/model/realtime_data.go` (public structs) and the
 * envelopes written in `handler.go`. When changing a public JSON tag or field
 * on either side, update BOTH files in the same change (see the JSON Model
 * Boundary rule in .trellis/spec/backend/directory-structure.md).
 *
 * These types describe WIRE shapes (what `normalizeResponse` validates), not
 * the post-validation normalized shape — that typing lives with the data layer.
 * Runtime validation at the payload boundary is mandatory regardless of these
 * types; never trust a fetch response for shape.
 */

/** `handler.go` success envelope: {code, log_id, version, data}; failures add `msg`. */
export interface ApiEnvelope<TData> {
  code: number;
  /** Present on service-error envelopes; absent (or empty) on success. */
  msg?: string;
  log_id?: string;
  /** Running backend build version; present on both success and failure. */
  version?: string;
  data: TData | null;
}

export interface ApiError {
  type: string;
  message: string;
}

export interface FreeTime {
  node: number;
  time: string;
}

export interface NodeInfo {
  node: number;
  time: string;
  room_count: number;
}

export interface RoomInfo {
  name: string;
  display_name: string;
  capacity: number;
  free_nodes: number[];
  free_times: FreeTime[];
}

export interface BuildingInfo {
  name: string;
  /** Backend-normalized label (alias/numeric-prefix rules live in the builder). */
  display_name: string;
  rooms: RoomInfo[];
}

export interface CampusInfo {
  id: string;
  name: string;
  buildings: BuildingInfo[];
  nodes: NodeInfo[];
}

/** Go `time.Time` serializes as an RFC 3339 string on the wire. */
export interface TodayClassroomsData {
  date: string;
  updated_at: string;
  expires_at: string;
  stale_until: string;
  stale: boolean;
  campuses: CampusInfo[];
  /** Omitted by the backend (`omitempty`) unless a refresh partially failed. */
  partial_campuses?: string[];
  error: ApiError | null;
}
