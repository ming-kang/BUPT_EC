import { useCallback, useEffect, useState } from "react";
import {
  extractMessage,
  fallbackErrorMessage,
  loadingResponse,
  normalizeResponse,
  readJson,
} from "./todayClassroomsResponse";

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
