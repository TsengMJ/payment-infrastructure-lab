import type { DashboardClient } from "../api";
import { ApiNotReadyError } from "../errors";

export const livePaths = {
  merchantSession: "/v1/dashboard/sessions",
  internalSession: "/v1/internal/sessions",
  payments: "/v1/payments",
  timeline: (id: string) => `/v1/payments/${encodeURIComponent(id)}/timeline`,
  settlements: "/v1/reports/settlements",
  deliveries: "/v1/webhook_deliveries",
  replay: (id: string) => `/v1/webhook_deliveries/${encodeURIComponent(id)}/replay`,
  cases: "/v1/onboarding/cases",
  decision: (id: string) => `/v1/onboarding/cases/${encodeURIComponent(id)}/decision`,
  refunds: "/v1/internal/refunds",
  approveRefund: (id: string) => `/v1/internal/refunds/${encodeURIComponent(id)}/approve`,
  audit: "/v1/audit_logs",
};

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const base = process.env.API_BASE_URL;
  if (process.env.API_MODE !== "live" || !base) {
    throw new ApiNotReadyError(path);
  }
  let response: Response;
  try {
    response = await fetch(new URL(path, base), {
      ...init,
      headers: {
        accept: "application/json",
        "content-type": "application/json",
        ...init?.headers,
      },
    });
  } catch {
    throw new ApiNotReadyError(path);
  }
  if (!response.ok) {
    throw new ApiNotReadyError(path);
  }
  return (await response.json()) as T;
}

function notReady(path: string): Promise<never> {
  return request(path);
}

export const liveClient: DashboardClient = {
  loginMerchant() {
    return notReady(livePaths.merchantSession);
  },
  loginInternal() {
    return notReady(livePaths.internalSession);
  },
  merchantFromToken() {
    return notReady(livePaths.merchantSession);
  },
  internalFromToken() {
    return notReady(livePaths.internalSession);
  },
  listPayments() {
    return notReady(livePaths.payments);
  },
  getTimeline(id) {
    return notReady(livePaths.timeline(id));
  },
  getSettlement() {
    return notReady(livePaths.settlements);
  },
  listDeliveries() {
    return notReady(livePaths.deliveries);
  },
  replayDelivery(_actor, id) {
    return notReady(livePaths.replay(id));
  },
  replayAsInternal(_actor, id) {
    return notReady(livePaths.replay(id));
  },
  listKyb() {
    return notReady(livePaths.cases);
  },
  decideKyb(_actor, id) {
    return notReady(livePaths.decision(id));
  },
  listRefunds() {
    return notReady(livePaths.refunds);
  },
  requestRefund() {
    return notReady(livePaths.refunds);
  },
  approveRefund(_actor, id) {
    return notReady(livePaths.approveRefund(id));
  },
  listAudit() {
    return notReady(livePaths.audit);
  },
};
