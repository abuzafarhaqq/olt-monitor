import { useCallback, useEffect, useState } from "react";
import { dataSource } from "../services/data";
import type { OverviewResponse } from "../types/domain";

interface Resource<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  reload: () => void;
}

/**
 * Minimal async loader: no cache, no retry, no cancellation library.
 *
 * The one rule it does enforce: a response that arrives after the component
 * unmounted (or after a newer request started) is dropped. Without that, a slow
 * first request overwrites a fast second one and the page shows stale data.
 */
export function useResource<T>(load: () => Promise<T>, deps: unknown[]): Resource<T> {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);

  const reload = useCallback(() => setNonce((value) => value + 1), []);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    setError(null);

    load()
      .then((result) => {
        if (!alive) return;
        setData(result);
        setLoading(false);
      })
      .catch((caught: unknown) => {
        if (!alive) return;
        setError(caught instanceof Error ? caught.message : "Could not load data");
        setLoading(false);
      });

    return () => {
      alive = false;
    };
  }, [...deps, nonce]);

  return { data, loading, error, reload };
}

export function useOverview(): Resource<OverviewResponse> {
  return useResource<OverviewResponse>(() => dataSource.overview(), []);
}
