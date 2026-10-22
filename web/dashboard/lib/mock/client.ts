import { randomBytes } from "node:crypto";

import type { DashboardClient } from "../api";
import { AuthError, NotFoundError, PermissionError } from "../errors";
import { canReplay } from "../roles";
import type { AuditEntry, InternalUser, KybCase, MerchantUser, RefundRequest, WebhookDelivery } from "../types";
import { getStore, type InternalRecord, type MerchantRecord } from "./store";

const MAX_FAILURES = 5;

function publicMerchant(user: MerchantRecord): MerchantUser {
  return {
    id: user.id,
    email: user.email,
    name: user.name,
    role: user.role,
    merchantId: user.merchantId,
    merchantName: user.merchantName,
  };
}

function publicInternal(user: InternalRecord): InternalUser {
  return {
    id: user.id,
    email: user.email,
    name: user.name,
    canReview: user.canReview,
    canRefund: user.canRefund,
  };
}

function loginRecord<T extends { email: string; password: string; failures: number; locked: boolean }>(
  records: T[],
  email: string,
  password: string,
): T {
  const user = records.find((item) => item.email === email);
  if (!user || user.locked) {
    if (user?.locked) {
      throw new AuthError("帳號已鎖定");
    }
    throw new AuthError("密碼不正確");
  }
  if (user.password !== password) {
    user.failures += 1;
    if (user.failures >= MAX_FAILURES) {
      user.locked = true;
      throw new AuthError("帳號已鎖定");
    }
    throw new AuthError("密碼不正確");
  }
  user.failures = 0;
  return user;
}

function issueToken(kind: "merchant" | "internal", userId: string): string {
  const token = randomBytes(16).toString("hex");
  getStore().tokens.set(token, { kind, userId });
  return token;
}

function appendAudit(entry: Omit<AuditEntry, "id" | "at">) {
  const store = getStore();
  store.audit.push({
    id: `aud_${store.audit.length + 1}`,
    at: new Date().toISOString(),
    ...entry,
  });
}

function netOf(row: { gross: number; fee: number; platformFee: number; refund: number; dispute: number }): number {
  return row.gross - row.fee - row.platformFee - row.refund - row.dispute;
}

function replay(deliveryId: string, actor: string): WebhookDelivery {
  const delivery = getStore().deliveries.get(deliveryId);
  if (!delivery) {
    throw new NotFoundError("找不到投遞紀錄");
  }
  const next: WebhookDelivery = {
    ...delivery,
    status: "replayed",
    attempts: delivery.attempts + 1,
  };
  getStore().deliveries.set(deliveryId, next);
  appendAudit({
    actor,
    action: "重送 webhook",
    before: delivery.eventId,
    after: delivery.eventId,
    reason: "手動重送",
  });
  return next;
}

export const mockClient: DashboardClient = {
  async loginMerchant(email, password) {
    const user = loginRecord(getStore().merchants, email, password);
    return { token: issueToken("merchant", user.id), user: publicMerchant(user) };
  },

  async loginInternal(email, password) {
    const user = loginRecord(getStore().internals, email, password);
    return { token: issueToken("internal", user.id), user: publicInternal(user) };
  },

  async merchantFromToken(token) {
    const record = getStore().tokens.get(token);
    if (!record || record.kind !== "merchant") {
      return null;
    }
    const user = getStore().merchants.find((item) => item.id === record.userId);
    return user ? publicMerchant(user) : null;
  },

  async internalFromToken(token) {
    const record = getStore().tokens.get(token);
    if (!record || record.kind !== "internal") {
      return null;
    }
    const user = getStore().internals.find((item) => item.id === record.userId);
    return user ? publicInternal(user) : null;
  },

  async listPayments() {
    return getStore().payments;
  },

  async getTimeline(paymentId) {
    const events = getStore().timelines.get(paymentId);
    if (!events) {
      throw new NotFoundError("找不到付款");
    }
    return events;
  },

  async getSettlement() {
    const rows = [
      { id: "stl_1", gross: 100000, fee: 2900, platformFee: 1000, refund: 0, dispute: 0, currency: "TWD" },
      { id: "stl_2", gross: 80000, fee: 2320, platformFee: 800, refund: 5000, dispute: 0, currency: "TWD" },
      { id: "stl_3", gross: 20000, fee: 580, platformFee: 200, refund: 0, dispute: 2000, currency: "TWD" },
    ].map((row) => ({ ...row, net: netOf(row) }));
    const payoutAmount = rows.reduce((sum, row) => sum + row.net, 0);
    return { periodLabel: "2026-10-01 至 2026-10-21", currency: "TWD", rows, payoutAmount };
  },

  async listDeliveries() {
    return [...getStore().deliveries.values()];
  },

  async replayDelivery(actor, deliveryId) {
    if (!canReplay(actor.role)) {
      throw new PermissionError("沒有權限");
    }
    return replay(deliveryId, actor.email);
  },

  async replayAsInternal(actor, deliveryId) {
    if (!actor.canRefund) {
      throw new PermissionError("沒有權限");
    }
    return replay(deliveryId, actor.name);
  },

  async listKyb(actor) {
    const store = getStore();
    for (const item of store.cases) {
      const key = `${actor.id}:${item.id}`;
      if (store.viewedKyb.has(key)) {
        continue;
      }
      store.viewedKyb.add(key);
      appendAudit({
        actor: actor.name,
        action: "查看入駐申請",
        before: item.id,
        after: item.id,
        reason: "審核需要",
      });
    }
    return store.cases;
  },

  async decideKyb(actor, caseId, decision) {
    if (!actor.canReview) {
      throw new PermissionError("沒有權限");
    }
    const item = getStore().cases.find((entry) => entry.id === caseId);
    if (!item) {
      throw new NotFoundError("找不到申請");
    }
    if (item.status !== "pending") {
      return item;
    }
    const next: KybCase = {
      ...item,
      status: decision,
      capability: decision === "approved" ? "可收款" : "未開通",
    };
    const index = getStore().cases.findIndex((entry) => entry.id === caseId);
    getStore().cases[index] = next;
    appendAudit({
      actor: actor.name,
      action: decision === "approved" ? "核准入駐" : "拒絕入駐",
      before: "未開通",
      after: next.capability,
      reason: "人工審核",
    });
    return next;
  },

  async listRefunds() {
    return getStore().refunds;
  },

  async requestRefund(actor, paymentId, amount, reason) {
    if (!actor.canRefund) {
      throw new PermissionError("沒有權限");
    }
    if (!Number.isInteger(amount) || amount <= 0) {
      throw new PermissionError("金額必須是正整數");
    }
    if (!getStore().timelines.has(paymentId)) {
      throw new NotFoundError("找不到付款");
    }
    const store = getStore();
    store.refundSeq += 1;
    const refund: RefundRequest = {
      id: `rfd_${store.refundSeq}`,
      paymentId,
      amount,
      currency: "TWD",
      reason,
      status: "pending",
      makerId: actor.id,
      makerName: actor.name,
      checkerName: null,
    };
    store.refunds.push(refund);
    appendAudit({
      actor: actor.name,
      action: "發起退款",
      before: "無",
      after: String(amount),
      reason,
    });
    return refund;
  },

  async approveRefund(actor, refundId) {
    if (!actor.canRefund) {
      throw new PermissionError("沒有權限");
    }
    const refund = getStore().refunds.find((item) => item.id === refundId);
    if (!refund) {
      throw new NotFoundError("找不到退款");
    }
    if (refund.makerId === actor.id) {
      throw new PermissionError("不能自己核准");
    }
    if (refund.status === "approved") {
      return refund;
    }
    refund.status = "approved";
    refund.checkerName = actor.name;
    appendAudit({
      actor: actor.name,
      action: "核准退款",
      before: "待覆核",
      after: "已核准",
      reason: refund.reason,
    });
    return refund;
  },

  async listAudit() {
    return getStore().audit;
  },
};
