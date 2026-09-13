import type { ReactNode } from "react";

type Tone = "accent" | "info" | "ok" | "warn" | "bad" | "stale";

export function GroupedList({ title, children }: { title?: string; children: ReactNode }) {
  return (
    <div>
      {title && <div className="group-title">{title}</div>}
      <div className="settings-group">{children}</div>
    </div>
  );
}

export function GroupedRow({
  icon,
  tone = "accent",
  label,
  description,
  trailing,
  clickable,
  onClick,
}: {
  icon?: ReactNode;
  tone?: Tone;
  label: ReactNode;
  description?: ReactNode;
  trailing?: ReactNode;
  clickable?: boolean;
  onClick?: () => void;
}) {
  const content = (
    <>
      {icon !== undefined && <span className={`row-icon tone-${tone}`}>{icon}</span>}
      <span className="row-body">
        <span className="row-label">{label}</span>
        {description && <span className="row-description">{description}</span>}
      </span>
      {trailing && <span className="row-trailing">{trailing}</span>}
      {clickable && <span className="chevron" aria-hidden="true">›</span>}
    </>
  );

  if (clickable && onClick) {
    return (
      <button type="button" className="settings-row clickable" onClick={onClick}>
        {content}
      </button>
    );
  }
  return <div className="settings-row">{content}</div>;
}
