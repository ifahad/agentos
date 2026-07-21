import { useState } from "react";
import { saveAdminKey } from "../lib/api";

interface Props {
  adminKey: string;
  onSave: (key: string) => void;
  onClose: () => void;
}

export function SettingsModal({ adminKey, onSave, onClose }: Props) {
  const [value, setValue] = useState(adminKey);

  const save = () => {
    const trimmed = value.trim();
    saveAdminKey(trimmed);
    onSave(trimmed);
  };

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h2>Settings</h2>
        <p>
          The admin key authenticates calls to the gateway admin API
          (<code>/admin/keys</code>, <code>/admin/usage</code>, <code>/admin/audit</code>). It is
          stored in this browser only.
        </p>
        <label className="field">
          <span>Gateway admin key</span>
          <input
            type="password"
            className="mono"
            value={value}
            placeholder="AGENTOS_ADMIN_KEY"
            onChange={(e) => setValue(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && save()}
            autoFocus
          />
        </label>
        <div className="modal-actions">
          <button className="btn" onClick={onClose}>
            Cancel
          </button>
          <button className="btn primary" onClick={save}>
            Save
          </button>
        </div>
      </div>
    </div>
  );
}
