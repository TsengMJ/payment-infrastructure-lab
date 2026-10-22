import type { PaymentMethod } from "./types";

const CATALOG: Array<PaymentMethod & { currencies: string[] }> = [
  { kind: "card", label: "卡片", currencies: ["TWD", "USD", "JPY"] },
  { kind: "wallet", label: "電子錢包", currencies: ["TWD", "USD"] },
  { kind: "bank_transfer", label: "銀行轉帳", currencies: ["TWD"] },
];

export function methodsFor(currency: string): PaymentMethod[] {
  return CATALOG.filter((item) => item.currencies.includes(currency)).map(({ kind, label }) => ({
    kind,
    label,
  }));
}
