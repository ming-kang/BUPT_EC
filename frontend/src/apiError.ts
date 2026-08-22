/**
 * Structured fetch-boundary error. Keeps the real HTTP status, the business
 * envelope code, the server log_id and the running build version on the error
 * object so downstream code never has to guess 500 or parse them back out of a
 * message string.
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
    { status, code, logId, version }: ApiErrorOptions = {}
  ) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.logId = logId || "";
    this.version = version || "";
  }
}
