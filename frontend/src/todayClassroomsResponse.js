export const loadingResponse = { code: 1, msg: "加载中", data: null };

export const fallbackErrorMessage = "数据获取失败，请稍后重试";

export function extractMessage(payload) {
  return typeof payload?.msg === "string" && payload.msg.trim() !== ""
    ? payload.msg.trim()
    : "";
}

export function normalizeResponse(payload) {
  if (!payload || typeof payload !== "object") {
    throw new Error("服务返回格式异常");
  }

  const code = Number(payload.code);
  if (!Number.isFinite(code)) {
    throw new Error("服务返回状态异常");
  }

  if (code !== 0) {
    return {
      code,
      msg: extractMessage(payload) || fallbackErrorMessage,
      data: null,
    };
  }

  if (!payload.data || typeof payload.data !== "object") {
    throw new Error("服务返回数据格式异常");
  }
  if (!Array.isArray(payload.data.campuses)) {
    throw new Error("服务返回校区数据异常");
  }

  return {
    code: 0,
    msg: extractMessage(payload),
    data: {
      ...payload.data,
      campuses: payload.data.campuses,
    },
  };
}

export async function readJson(response) {
  try {
    return await response.json();
  } catch {
    return null;
  }
}
