import { useEffect, useState } from "react";
import { api } from "../lib/api";

type Report = {
  energy_cents: number;
  downtime_cents: number;
  repair_cents: number;
  replacement_cents: number;
  carbon_grams: number;
  forecast_cents: number;
};

function dollars(cents: number) {
  return `$${(cents / 100).toFixed(2)}`;
}

export default function Cost() {
  const [row, setRow] = useState<Report | null>(null);
  const [err, setErr] = useState("");

  useEffect(() => {
    api<Report>("/api/v1/reports/cost").then(setRow).catch((e: Error) => setErr(e.message));
  }, []);

  return (
    <>
      <div className="topbar">
        <div>
          <h1>Cost</h1>
          <p className="lede">Energy, open-incident downtime, and repair next to replacement.</p>
        </div>
      </div>
      {err && <p className="lede">{err}</p>}
      {row && (
        <div className="split">
          <div className="card"><p className="kicker">Energy</p><h2>{dollars(row.energy_cents)}</h2></div>
          <div className="card"><p className="kicker">Downtime</p><h2>{dollars(row.downtime_cents)}</h2></div>
          <div className="card"><p className="kicker">Repair / replace</p><h2>{dollars(row.repair_cents)} / {dollars(row.replacement_cents)}</h2></div>
          <div className="card"><p className="kicker">Carbon</p><h2>{row.carbon_grams.toFixed(0)} g</h2></div>
          <div className="card"><p className="kicker">7-day forecast</p><h2>{dollars(row.forecast_cents)}</h2></div>
        </div>
      )}
    </>
  );
}
