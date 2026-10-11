import { useCallback, useEffect, useRef, useState } from "react";
import type { ApiPageResult } from "@/types/api";

type UseListPageOptions<TFilters, TRecord, TQuery extends Record<string, unknown> = Record<string, unknown>> = {
  fetch: (query: TQuery) => Promise<ApiPageResult<TRecord>>;
  defaultFilters: TFilters;
  toQuery: (filters: TFilters, page: number, pageSize: number) => TQuery;
  defaultPageSize?: number;
  defaultPage?: number;
  onError?: (error: unknown) => void;
  /** 加载失败时保留上一次成功的数据，避免轮询失败清空已经在看的记录。 */
  keepDataOnError?: boolean;
};

type UseListPageReturn<TFilters, TRecord> = {
  data: TRecord[];
  total: number;
  loading: boolean;
  error: string;
  page: number;
  pageSize: number;
  setPage: (page: number) => void;
  setPageSize: (size: number) => void;
  reload: () => void;
  /** 后台轮询使用：不切换加载状态，失败也不清空列表。 */
  silentReload: () => void;
  filters: TFilters;
  setFilter: <K extends keyof TFilters>(key: K, value: TFilters[K]) => void;
  setFilters: (updater: (prev: TFilters) => TFilters) => void;
  submitFilters: () => void;
  resetFilters: () => void;
};

export function useListPage<TFilters, TRecord, TQuery extends Record<string, unknown> = Record<string, unknown>>(
  options: UseListPageOptions<TFilters, TRecord, TQuery>,
): UseListPageReturn<TFilters, TRecord> {
  const { fetch, defaultFilters, toQuery, defaultPageSize = 10, defaultPage = 1, onError, keepDataOnError = false } = options;

  const [data, setData] = useState<TRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const [page, setPage] = useState(defaultPage);
  const [pageSize, setPageSize] = useState(defaultPageSize);
  const [queryVersion, setQueryVersion] = useState(0);

  const [filters, setFiltersState] = useState<TFilters>(defaultFilters);
  const [appliedFilters, setAppliedFilters] = useState<TFilters>(defaultFilters);

  // 用 ref 缓存调用方传入的函数，避免内联函数引用变化导致循环请求
  const fetchRef = useRef(fetch);
  fetchRef.current = fetch;
  const toQueryRef = useRef(toQuery);
  toQueryRef.current = toQuery;
  const onErrorRef = useRef(onError);
  onErrorRef.current = onError;
  const keepDataRef = useRef(keepDataOnError);
  keepDataRef.current = keepDataOnError;
  // 递增的请求序号：只有最新一次请求的结果可以写入状态，旧响应不得覆盖新页数据。
  const requestRef = useRef(0);

  const loadData = useCallback(async (silent = false) => {
    const request = requestRef.current + 1;
    requestRef.current = request;
    if (!silent) {
      setLoading(true);
    }
    setError("");

    try {
      const result = await fetchRef.current(toQueryRef.current(appliedFilters, page, pageSize));
      if (request !== requestRef.current) {
        return;
      }
      setData(result.records);
      setTotal(result.total);
    } catch (loadError) {
      if (request !== requestRef.current) {
        return;
      }
      if (!keepDataRef.current) {
        setData([]);
        setTotal(0);
      }
      if (onErrorRef.current) {
        onErrorRef.current(loadError);
      }
      setError(
        loadError instanceof Error ? loadError.message : "加载失败",
      );
    } finally {
      if (!silent && request === requestRef.current) {
        setLoading(false);
      }
    }
  }, [appliedFilters, page, pageSize]);

  useEffect(() => {
    void loadData();
  }, [loadData, queryVersion]);

  const setPageSizeAndReset = useCallback((size: number) => {
    setPageSize(size);
    setPage(1);
  }, []);

  const setFilter = useCallback(
    <K extends keyof TFilters>(key: K, value: TFilters[K]) => {
      setFiltersState((prev) => ({ ...prev, [key]: value }));
    },
    [],
  );

  const setFilters = useCallback(
    (updater: (prev: TFilters) => TFilters) => {
      setFiltersState(updater);
    },
    [],
  );

  const submitFilters = useCallback(() => {
    setPage(1);
    setAppliedFilters(filters);
    setQueryVersion((version) => version + 1);
  }, [filters]);

  const resetFilters = useCallback(() => {
    setFiltersState(defaultFilters);
    setAppliedFilters(defaultFilters);
    setPage(1);
  }, [defaultFilters]);

  const reload = useCallback(() => {
    void loadData();
  }, [loadData]);

  const silentReload = useCallback(() => {
    void loadData(true);
  }, [loadData]);

  return {
    data,
    total,
    loading,
    error,
    page,
    pageSize,
    setPage,
    setPageSize: setPageSizeAndReset,
    reload,
    silentReload,
    filters,
    setFilter,
    setFilters,
    submitFilters,
    resetFilters,
  };
}
