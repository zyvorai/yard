import { useEffect, useRef } from "react";
import { api, getToken } from "./api";

export type StreamEvent = { kind: string; data: unknown };

/** Subscribe to Yard SSE. Reconnects with backoff while the tab is open. */
export function useYardStream(onEvent: (ev: StreamEvent) => void, enabled = true) {
  const cb = useRef(onEvent);
  cb.current = onEvent;

  useEffect(() => {
    if (!enabled) return;
    let closed = false;
    let es: EventSource | null = null;
    let timer: number | undefined;
    let delay = 1000;

    async function connect() {
      const tok = getToken();
      if (!tok || closed) return;
      let ticket: { ticket: string };
      try {
        ticket = await api<{ ticket: string }>("/api/v1/stream/ticket", { method: "POST" });
      } catch {
        if (closed) return;
        timer = window.setTimeout(connect, delay);
        delay = Math.min(delay * 2, 15000);
        return;
      }
      if (closed) return;
      es = new EventSource(`/api/v1/stream?ticket=${encodeURIComponent(ticket.ticket)}`);
      es.onmessage = (msg) => {
        try {
          const parsed = JSON.parse(msg.data) as StreamEvent;
          cb.current(parsed);
          delay = 1000;
        } catch {
          /* ignore */
        }
      };
      es.onerror = () => {
        es?.close();
        es = null;
        if (closed) return;
        timer = window.setTimeout(connect, delay);
        delay = Math.min(delay * 2, 15000);
      };
    }

    connect();
    return () => {
      closed = true;
      if (timer) window.clearTimeout(timer);
      es?.close();
    };
  }, [enabled]);
}

export function notifyCritical(title: string, body: string) {
  if (typeof Notification === "undefined") return;
  if (Notification.permission === "granted") {
    try {
      new Notification(title, { body, icon: "/logo.svg" });
    } catch {
      /* ignore */
    }
    return;
  }
  if (Notification.permission === "default") {
    void Notification.requestPermission();
  }
}
