import type { ReactNode } from "react";
import { Button } from "../ui";
import { Input, Select } from "../ui/Field";
import { Icon } from "../ui/icons";

export interface FacetDef { key: string; label: string; options: readonly string[]; value: string; onChange: (v: string) => void; }

export function TableToolbar({
  query, onQuery, facets = [], onExport, extra,
}: {
  query: string;
  onQuery: (q: string) => void;
  facets?: readonly FacetDef[];
  onExport?: (format: "csv" | "json") => void;
  extra?: ReactNode;
}) {
  return (
    <div className="table-toolbar head-group">
      <span className="tt-search">
        <Icon name="search" size={13} className="tt-search-icon" />
        <Input type="search" placeholder="Search" value={query} onChange={(e) => onQuery(e.target.value)} aria-label="Search table" />
      </span>
      {facets.map((f) => (
        <Select key={f.key} value={f.value} onChange={(e) => f.onChange(e.target.value)} aria-label={f.label}>
          <option value="">All {f.label}</option>
          {f.options.map((o) => <option key={o} value={o}>{o}</option>)}
        </Select>
      ))}
      {extra}
      {onExport && (
        <>
          <Button small icon="export" onClick={() => onExport("csv")}>CSV</Button>
          <Button small icon="export" onClick={() => onExport("json")}>JSON</Button>
        </>
      )}
    </div>
  );
}
