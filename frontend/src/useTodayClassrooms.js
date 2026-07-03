import { useCallback, useEffect, useState } from "react";

const loadingResponse = { code: 1, msg: "加载中", data: null };
const fallbackErrorMessage = "数据获取失败，请稍后重试";

function extractMessage(payload) {
  return typeof payload?.msg === "string" && payload.msg.trim() !== ""
    ? payload.msg.trim()
    : "";
}

function normalizeResponse(payload) {
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

async function readJson(response) {
  try {
    return await response.json();
  } catch {
    return null;
  }
}

export default function useTodayClassrooms() {
  const [reloadKey, setReloadKey] = useState(0);
  const [spinning, setSpinning] = useState(true);
  const [resp, setResp] = useState(loadingResponse);

  useEffect(() => {
    const controller = new AbortController();

    async function loadData() {
      setSpinning(true);
      setResp((current) => (current.code === 0 ? current : loadingResponse));

      try {
        const response = await fetch("/api/get_data", {
          signal: controller.signal,
          headers: { Accept: "application/json" },
        });
        const payload = await readJson(response);

        if (!response.ok) {
          throw new Error(
            extractMessage(payload) || `请求失败 (${response.status})`
          );
        }

        setResp(normalizeResponse(payload));
      } catch (error) {
        if (controller.signal.aborted) {
          return;
        }
        setResp({
          code: 500,
          msg: error instanceof Error ? error.message : fallbackErrorMessage,
          data: null,
        });
      } finally {
        if (!controller.signal.aborted) {
          setSpinning(false);
        }
      }
    }

    loadData();
    return () => controller.abort();
  }, [reloadKey]);

  const retry = useCallback(() => {
    setReloadKey((current) => current + 1);
  }, []);

  useEffect(() => {
    if (resp.code !== 0 || !resp.data?.expires_at) {
      return;
    }
    const expiresAt = new Date(resp.data.expires_at).getTime();
    if (!Number.isFinite(expiresAt)) {
      return;
    }
    const delay = Math.max(expiresAt - Date.now(), 60_000);
    const timer = setTimeout(() => {
      setReloadKey((current) => current + 1);
    }, delay);
    return () => clearTimeout(timer);
  }, [resp]);

  return {
    resp,
    spinning,
    isError: resp.code !== 0 && !spinning,
    retry,
  };
}
