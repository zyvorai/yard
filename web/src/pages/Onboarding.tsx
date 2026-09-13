import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../lib/api";
import { GroupedList, GroupedRow } from "../components/GroupedList";
import { useDialogA11y } from "../lib/useDialogA11y";

type Step = { id: string; title: string; body: string; done?: boolean; href?: string };

function CheckIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M5 12.5l4.5 4.5L19 7" stroke="currentColor" strokeWidth="2.25" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function CloseIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M6 6l12 12M18 6L6 18" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" />
    </svg>
  );
}

function StepPanel({ step, index, onClose }: { step: Step; index: number; onClose: () => void }) {
  const cardRef = useRef<HTMLDivElement>(null);
  useDialogA11y(cardRef, true, onClose);
  return (
    <div className="step-overlay" role="presentation" onClick={onClose}>
      <div ref={cardRef} className="step-card" role="dialog" aria-modal="true" aria-labelledby="step-panel-title" tabIndex={-1} onClick={(e) => e.stopPropagation()}>
        <button type="button" className="icon-btn step-card-close" aria-label="Close" onClick={onClose}>
          <CloseIcon />
        </button>
        <div className="step-icon">{index + 1}</div>
        <h2 id="step-panel-title">{step.title}</h2>
        <p>{step.body}</p>
        <div className="step-actions">
          {step.href ? (
            <Link className="btn accent" to={step.href} onClick={onClose}>Continue</Link>
          ) : (
            <button type="button" className="btn accent" onClick={onClose}>Got it</button>
          )}
          <button type="button" className="btn ghost" onClick={onClose}>Not now</button>
        </div>
      </div>
    </div>
  );
}

export default function Onboarding() {
  const [steps, setSteps] = useState<Step[]>([]);
  const [completed, setCompleted] = useState(0);
  const [total, setTotal] = useState(0);
  const [focusedIndex, setFocusedIndex] = useState<number | null>(null);

  useEffect(() => {
    api<{ steps: Step[]; completed: number; total: number }>("/api/v1/onboarding").then((r) => {
      setSteps(r.steps || []);
      setCompleted(r.completed || 0);
      setTotal(r.total || r.steps?.length || 0);
    });
  }, []);

  const focused = focusedIndex !== null ? steps[focusedIndex] : undefined;

  return (
    <>
      <div className="topbar">
        <div>
          <h1>Get started</h1>
          <p className="lede">Yard works alone — connectors are optional.</p>
        </div>
        <div style={{ display: "flex", flexDirection: "column", alignItems: "flex-end", gap: 10 }}>
          <div className="onboarding-progress" aria-label={`${completed} of ${total} steps complete`}>
            {steps.map((s) => (
              <span key={s.id} className={`dot${s.done ? " done" : ""}`} />
            ))}
          </div>
          <Link className="btn accent" to="/">Open Overview</Link>
        </div>
      </div>
      <div className="settings-stack">
        <GroupedList>
          {steps.map((s, i) => (
            <GroupedRow
              key={s.id}
              icon={s.done ? <CheckIcon /> : <>{i + 1}</>}
              tone={s.done ? "ok" : "accent"}
              label={s.title}
              description={s.body}
              clickable={!s.done}
              onClick={!s.done ? () => setFocusedIndex(i) : undefined}
              trailing={s.done ? <span className="pill ok">Done</span> : undefined}
            />
          ))}
        </GroupedList>
      </div>
      {focused && focusedIndex !== null && (
        <StepPanel step={focused} index={focusedIndex} onClose={() => setFocusedIndex(null)} />
      )}
    </>
  );
}
