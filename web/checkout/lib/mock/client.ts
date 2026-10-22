import { CheckoutError } from "../errors";
import { methodsFor } from "../methods";
import type { CheckoutClient } from "../api";
import type { CheckoutSession, ConfirmScenario } from "../types";
import { getStore } from "./store";

function mustSession(id: string): CheckoutSession {
  const current = getStore().sessions.get(id);
  if (!current) {
    throw new CheckoutError("找不到結帳");
  }
  return current;
}

function paymentIdFor(session: CheckoutSession): string {
  return session.paymentId ?? `pay_${session.id}`;
}

export const mockClient: CheckoutClient = {
  async getSession(id) {
    return getStore().sessions.get(id) ?? null;
  },

  async confirm(id, scenario) {
    const current = mustSession(id);
    if (current.status === "expired") {
      throw new CheckoutError("結帳已過期");
    }
    if (current.status !== "open") {
      return current;
    }
    const next = applyScenario(current, scenario);
    getStore().sessions.set(id, next);
    if (next.status === "succeeded" && next.linkId) {
      markLinkUsed(next.linkId);
    }
    return next;
  },

  async completeAction(id) {
    const current = mustSession(id);
    if (current.status === "succeeded") {
      return current;
    }
    if (current.status !== "requires_action") {
      throw new CheckoutError("目前不能完成驗證");
    }
    const next: CheckoutSession = { ...current, status: "succeeded", paymentId: paymentIdFor(current) };
    getStore().sessions.set(id, next);
    if (next.linkId) {
      markLinkUsed(next.linkId);
    }
    return next;
  },

  async getLink(id) {
    return getStore().links.get(id) ?? null;
  },

  async startFromLink(id) {
    const store = getStore();
    const link = store.links.get(id);
    if (!link || link.status !== "active") {
      throw new CheckoutError("這個連結已經使用過");
    }
    if (!link.reusable) {
      const existing = [...store.sessions.values()].find(
        (item) => item.linkId === link.id && item.status !== "failed",
      );
      if (existing) {
        return existing;
      }
    }
    store.linkSeq += 1;
    const created: CheckoutSession = {
      id: `cs_${link.id}_${store.linkSeq}`,
      status: "open",
      amount: link.amount,
      currency: link.currency,
      methods: methodsFor(link.currency),
      paymentId: null,
      linkId: link.id,
    };
    store.sessions.set(created.id, created);
    return created;
  },
};

function applyScenario(current: CheckoutSession, scenario: ConfirmScenario): CheckoutSession {
  const paymentId = paymentIdFor(current);
  if (scenario === "3ds") {
    return { ...current, status: "requires_action", paymentId };
  }
  if (scenario === "decline") {
    return { ...current, status: "failed", paymentId };
  }
  if (scenario === "processing") {
    return { ...current, status: "processing", paymentId };
  }
  return { ...current, status: "succeeded", paymentId };
}

function markLinkUsed(linkId: string) {
  const link = getStore().links.get(linkId);
  if (!link || link.reusable) {
    return;
  }
  getStore().links.set(linkId, { ...link, status: "used" });
}
