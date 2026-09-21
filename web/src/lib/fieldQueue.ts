export type FieldOrder = {
  id: string;
  title: string;
  status: string;
  notes: string;
  kind?: string;
  priority?: string;
  checklist?: string;
  asset_id?: string;
};

export type FieldChange = {
  id: string;
  status: string;
  notes: string;
};

type Bag = {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
};

const ORDERS = "yard.field.orders";
const QUEUE = "yard.field.queue";

export function readOrders(storage: Bag): FieldOrder[] {
  return parse(storage.getItem(ORDERS));
}

export function writeOrders(storage: Bag, rows: FieldOrder[]) {
  storage.setItem(ORDERS, JSON.stringify(rows));
}

export function readQueue(storage: Bag): FieldChange[] {
  return parse(storage.getItem(QUEUE));
}

export function queueChange(storage: Bag, change: FieldChange) {
  const queue = readQueue(storage).filter((item) => item.id !== change.id);
  queue.push(change);
  storage.setItem(QUEUE, JSON.stringify(queue));
  const orders = readOrders(storage).map((row) => (row.id === change.id ? { ...row, status: change.status, notes: change.notes } : row));
  writeOrders(storage, orders);
}

export async function flushQueue(storage: Bag, send: (change: FieldChange) => Promise<void>): Promise<number> {
  const left: FieldChange[] = [];
  let sent = 0;
  for (const change of readQueue(storage)) {
    try {
      await send(change);
      sent++;
    } catch {
      left.push(change);
    }
  }
  storage.setItem(QUEUE, JSON.stringify(left));
  return sent;
}

function parse<T>(raw: string | null): T[] {
  if (!raw) return [];
  try {
    const v = JSON.parse(raw);
    return Array.isArray(v) ? v : [];
  } catch {
    return [];
  }
}
