import { methodsFor } from "../methods";
import type { CheckoutSession, PaymentLink } from "../types";

type Store = {
  sessions: Map<string, CheckoutSession>;
  links: Map<string, PaymentLink>;
  linkSeq: number;
};

const STORE_KEY = "__paymentLabCheckoutStore";

function session(
  id: string,
  status: CheckoutSession["status"],
  amount: number,
  currency: string,
  paymentId: string | null = null,
  linkId: string | null = null,
): CheckoutSession {
  return {
    id,
    status,
    amount,
    currency,
    methods: methodsFor(currency),
    paymentId,
    linkId,
  };
}

function seed(): Store {
  const sessions = new Map<string, CheckoutSession>([
    ["cs_success", session("cs_success", "open", 120000, "TWD")],
    ["cs_3ds", session("cs_3ds", "open", 88000, "TWD")],
    ["cs_decline", session("cs_decline", "open", 45000, "TWD")],
    ["cs_processing_case", session("cs_processing_case", "open", 33000, "TWD")],
    ["cs_expired", session("cs_expired", "expired", 20000, "TWD")],
    ["cs_paid", session("cs_paid", "succeeded", 66000, "TWD", "pay_cs_paid")],
    ["cs_processing", session("cs_processing", "processing", 41000, "TWD", "pay_cs_processing")],
    ["cs_jpy", session("cs_jpy", "open", 1500, "JPY")],
  ]);
  const links = new Map<string, PaymentLink>([
    [
      "plink_once",
      { id: "plink_once", title: "一次性連結", reusable: false, status: "active", amount: 50000, currency: "TWD" },
    ],
    [
      "plink_reusable",
      {
        id: "plink_reusable",
        title: "可重複連結",
        reusable: true,
        status: "active",
        amount: 80000,
        currency: "TWD",
      },
    ],
  ]);
  return { sessions, links, linkSeq: 0 };
}

function globalStore(): typeof globalThis & { [STORE_KEY]?: Store } {
  return globalThis as typeof globalThis & { [STORE_KEY]?: Store };
}

export function getStore(): Store {
  const holder = globalStore();
  if (!holder[STORE_KEY]) {
    holder[STORE_KEY] = seed();
  }
  return holder[STORE_KEY];
}

export function resetStore(): void {
  globalStore()[STORE_KEY] = seed();
}
