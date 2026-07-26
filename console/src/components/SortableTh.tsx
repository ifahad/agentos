import { Icon } from "../ui/icons";

export function SortableTh<T>({
  label, sortKey, active, dir, numeric, onSort, className,
}: {
  label: string;
  sortKey: keyof T;
  active: boolean;
  dir: "asc" | "desc";
  numeric?: boolean;
  onSort: (key: keyof T, numeric?: boolean) => void;
  className?: string;
}) {
  return (
    <th className={className} aria-sort={active ? (dir === "asc" ? "ascending" : "descending") : "none"}>
      <button type="button" className={`th-sort${active ? " active" : ""}`} onClick={() => onSort(sortKey, numeric)}>
        {label}
        <Icon name="sort" size={12} className={`th-sort-icon${active ? ` dir-${dir}` : ""}`} />
      </button>
    </th>
  );
}
