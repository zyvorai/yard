import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../lib/api";

type Step = { id: string; title: string; body: string; done?: boolean; href?: string };

export default function Onboarding() {
  const [steps, setSteps] = useState<Step[]>([]);
  const [completed, setCompleted] = useState(0);
  const [total, setTotal] = useState(0);

  useEffect(() => {
    api<{ steps: Step[]; completed: number; total: number }>("/api/v1/onboarding").then((r) => {
      setSteps(r.steps || []);
      setCompleted(r.completed || 0);
      setTotal(r.total || r.steps?.length || 0);
    });
  }, []);

  return (
    <>
      <div className="topbar">
        <div>
          <h1>Get started</h1>
          <p className="lede">
            {completed}/{total} steps complete. Yard works alone — connectors are optional.
          </p>
        </div>
        <Link className="btn accent" to="/">Open Overview</Link>
      </div>
      <div className="grid" style={{ gridTemplateColumns: "1fr", gap: 12, maxWidth: 640 }}>
        {steps.map((s, i) => (
          <div className="card" key={s.id} style={{ display: "flex", gap: 14, alignItems: "flex-start" }}>
            <div
              className={`pill ${s.done ? "ok" : "stale"}`}
              style={{ minWidth: 28, justifyContent: "center", textAlign: "center" }}
            >
              {s.done ? "✓" : i + 1}
            </div>
            <div style={{ flex: 1 }}>
              <h2 style={{ marginBottom: 4 }}>{s.title}</h2>
              <p className="lede">{s.body}</p>
              {s.href && !s.done && (
                <Link className="btn small" to={s.href} style={{ marginTop: 8, display: "inline-flex" }}>
                  Continue
                </Link>
              )}
            </div>
          </div>
        ))}
      </div>
    </>
  );
}
