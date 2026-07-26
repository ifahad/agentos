import { useMemo, useState } from "react";
import { computeTableView } from "../lib/table/view";
import type { TableView } from "../lib/table/view";
import type { FacetSelection } from "../lib/table/filter";
import type { SortDir, SortSpec } from "../lib/table/sort";

export interface UseTableViewOpts<T> {
  searchFields?: readonly (keyof T)[];
  initialSort?: SortSpec<T> | null;
  pageSize?: number;
}

export function useTableView<T>(rows: readonly T[], opts: UseTableViewOpts<T> = {}) {
  const [query, setQuery] = useState("");
  const [facets, setFacets] = useState<FacetSelection<T>>({});
  const [sort, setSort] = useState<SortSpec<T> | null>(opts.initialSort ?? null);
  const [page, setPage] = useState(0);

  const view: TableView<T> = useMemo(
    () =>
      computeTableView(rows, {
        searchFields: opts.searchFields,
        query,
        facets,
        sort,
        page,
        pageSize: opts.pageSize ?? 0,
      }),
    [rows, opts.searchFields, opts.pageSize, query, facets, sort, page],
  );

  function setFacet(key: keyof T, value: string) {
    setFacets((f) => ({ ...f, [key]: value }));
    setPage(0);
  }
  function toggleSort(key: keyof T, numeric?: boolean) {
    setSort((cur) => {
      const dir: SortDir = cur && cur.key === key && cur.dir === "asc" ? "desc" : "asc";
      return { key, dir, numeric };
    });
    setPage(0);
  }
  function changeQuery(q: string) {
    setQuery(q);
    setPage(0);
  }

  return { view, query, setQuery: changeQuery, facets, setFacet, sort, toggleSort, page, setPage };
}
