import React, { FormEvent, useEffect, useRef, useState } from "react";

type Props = {
  open: boolean;
  initialValue: string;
  onClose: () => void;
  onSubmit: (title: string) => void;
};

export function RenameDialog({ open, initialValue, onClose, onSubmit }: Props) {
  const [value, setValue] = useState(initialValue);
  const inputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (!open) {
      return;
    }

    setValue(initialValue);
    window.setTimeout(() => {
      inputRef.current?.focus();
      inputRef.current?.select();
    }, 0);
  }, [initialValue, open]);

  const handleSubmit = (event: FormEvent) => {
    event.preventDefault();
    const next = value.trim();
    if (!next) {
      return;
    }
    onSubmit(next);
  };

  return (
    <>
      <div className={`settings-backdrop ${open ? "open" : ""}`} onClick={onClose} aria-hidden="true" />
      <div className={`rename-dialog ${open ? "open" : ""}`} aria-hidden={!open}>
        <form className="rename-dialog-surface" onSubmit={handleSubmit}>
          <div className="rename-dialog-copy">
            <span className="eyebrow">Conversation</span>
            <h3>Rename topic</h3>
            <p>Give this conversation a clearer title.</p>
          </div>

          <label className="field rename-dialog-field">
            <span>Title</span>
            <input
              ref={inputRef}
              maxLength={120}
              onChange={(event) => setValue(event.target.value)}
              placeholder="Topic title"
              type="text"
              value={value}
            />
          </label>

          <div className="rename-dialog-actions">
            <button className="ghost" onClick={onClose} type="button">
              Cancel
            </button>
            <button className="action primary" disabled={!value.trim()} type="submit">
              Save
            </button>
          </div>
        </form>
      </div>
    </>
  );
}
