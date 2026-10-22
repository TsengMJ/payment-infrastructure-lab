import type { CheckoutClient } from "../api";
import { ApiNotReadyError } from "../errors";
import type { CheckoutSession, PaymentLink } from "../types";

export const livePaths = {
  session: (id: string) => `/v1/checkout/sessions/${encodeURIComponent(id)}`,
  confirm: (id: string) => `/v1/checkout/sessions/${encodeURIComponent(id)}/confirm`,
  complete: (id: string) => `/v1/checkout/sessions/${encodeURIComponent(id)}/complete`,
  link: (id: string) => `/v1/payment_links/${encodeURIComponent(id)}`,
  linkSessions: (id: string) => `/v1/payment_links/${encodeURIComponent(id)}/sessions`,
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
  if (response.status === 404 || response.status === 501) {
    throw new ApiNotReadyError(path);
  }
  if (!response.ok) {
    throw new ApiNotReadyError(path);
  }
  return (await response.json()) as T;
}

export const liveClient: CheckoutClient = {
  getSession(id) {
    return request<CheckoutSession | null>(livePaths.session(id));
  },
  confirm(id, scenario) {
    return request<CheckoutSession>(livePaths.confirm(id), {
      method: "POST",
      body: JSON.stringify({ scenario }),
    });
  },
  completeAction(id) {
    return request<CheckoutSession>(livePaths.complete(id), { method: "POST" });
  },
  getLink(id) {
    return request<PaymentLink | null>(livePaths.link(id));
  },
  startFromLink(id) {
    return request<CheckoutSession>(livePaths.linkSessions(id), { method: "POST" });
  },
};
