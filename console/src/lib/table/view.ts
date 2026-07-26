import { filterByFacets } from "./filter";
import type { FacetSelection } from "./filter";
import { filterBySearch } from "./search";
import { paginate } from "./paginate";
import { sortRows } from "./sort";
import type { SortSpec } from "./sort";

export interface TableViewConfig<T> {
  searchFields?: readonly (keyof T)[];
  query?: string;
  facets?: FacetSelection<T>;
  sort?: SortSpec<T> | null;
  page?: number;
  /** 0 (or omitted) = no pagination. */
  pageSize?: number;
}

export interface TableView<T> {
  /** The visible page after the full pipeline. */
  rows: T[];
  /** Filtered + sorted, BEFORE pagination — what export uses. */
  filtered: T[];
  total: number;
  filteredCount: number;
  page: number;
  pageCount: number;
}

/** filter → search → sort → paginate, as pure data. */
export function computeTableView<T>(rows: readonly T[], config: TableViewConfig<T>): TableView<T> {
  const faceted = filterByFacets(rows, config.facets ?? {});
  const searched =
    config.searchFields && config.query
      ? filterBySearch(faceted, config.searchFields, config.query)
      : faceted;
  const sorted = sortRows(searched, config.sort ?? null);
  const paged = paginate(sorted, config.page ?? 0, config.pageSize ?? 0);
  return {
    rows: paged.rows,
    filtered: sorted,
    total: rows.length,
    filteredCount: sorted.length,
    page: paged.page,
    pageCount: paged.pageCount,
  };
}
