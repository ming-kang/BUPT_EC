/**
 * Structured fetch-boundary error. Keeps the real HTTP status, the business
 * envelope code and the server log_id on the error object so downstream code
 * never has to guess 500 or parse them back out of a message string.
 */
export class ApiError extends Error {
  constructor(message, { status, code, logId } = {}) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.logId = logId || "";
  }
}
