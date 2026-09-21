import { describe, expect, it } from "vitest";
import { flushQueue, queueChange, readOrders, readQueue, writeOrders, type FieldOrder } from "./fieldQueue";

function memory() {
  const bag = new Map<string, string>();
  return {
    getItem: (key: string) => bag.get(key) ?? null,
    setItem: (key: string, value: string) => { bag.set(key, value); },
  };
}

describe("field queue", () => {
  it("keeps a completion until it syncs", async () => {
    const storage = memory();
    const orders: FieldOrder[] = [{ id: "wo_1", title: "Inspect pump", status: "open", notes: "" }];
    writeOrders(storage, orders);
    queueChange(storage, { id: "wo_1", status: "done", notes: "Belt replaced" });
    expect(readOrders(storage)[0].status).toBe("done");
    expect(readQueue(storage)).toHaveLength(1);

    let calls = 0;
    const sent = await flushQueue(storage, async () => { calls++; });
    expect(sent).toBe(1);
    expect(calls).toBe(1);
    expect(readQueue(storage)).toHaveLength(0);
  });

  it("leaves a change queued when the network rejects it", async () => {
    const storage = memory();
    queueChange(storage, { id: "wo_2", status: "done", notes: "" });
    const sent = await flushQueue(storage, async () => { throw new Error("offline"); });
    expect(sent).toBe(0);
    expect(readQueue(storage)).toHaveLength(1);
  });
});
