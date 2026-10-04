import { useEffect, useState } from "react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { apiPage } from "./api";
import type { Pagination } from "./contracts.generated";

export function useHistory<T>(path: string) {
  const [page, setPage] = useState(1);
  useEffect(() => {
    setPage(1);
  }, [path]);
  const query = useQuery({
    queryKey: [path, "page", page],
    queryFn: () => apiPage<T>(path, page),
    placeholderData: keepPreviousData,
  });
  useEffect(() => {
    if (
      !query.isPlaceholderData &&
      query.data &&
      page > query.data.meta.pages
    ) {
      setPage(query.data.meta.pages);
    }
  }, [page, query.data, query.isPlaceholderData]);
  return {
    ...query,
    data: query.data?.data,
    pagination: query.data?.meta,
    page,
    setPage,
  };
}
export function HistoryNavigation({
  history,
}: {
  history: {
    pagination?: Pagination;
    page: number;
    setPage: (page: number) => void;
    isFetching: boolean;
  };
}) {
  const meta = history.pagination;
  if (!meta || (meta.pages <= 1 && history.page <= 1)) return null;
  const start = meta.total
    ? Math.min((meta.page - 1) * meta.per_page + 1, meta.total)
    : 0;
  const end = Math.min(meta.page * meta.per_page, meta.total);
  return (
    <nav
      aria-label="History pages"
      className="mt-5 flex flex-wrap items-center justify-between gap-3 border-t pt-4"
    >
      <p
        role="status"
        aria-live="polite"
        className="text-xs text-muted-foreground"
      >
        {history.isFetching
          ? "Updating…"
          : `${start.toLocaleString("en-US")}–${end.toLocaleString("en-US")} of ${meta.total.toLocaleString("en-US")}`}
      </p>
      <div className="flex items-center gap-2">
        <Button
          size="sm"
          variant="outline"
          disabled={history.isFetching || history.page <= 1}
          onClick={() => history.setPage(history.page - 1)}
        >
          <ChevronLeft className="size-4" /> Previous
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={history.isFetching || history.page >= meta.pages}
          onClick={() => history.setPage(history.page + 1)}
        >
          Next <ChevronRight className="size-4" />
        </Button>
      </div>
    </nav>
  );
}
