import "server-only";

import type { CheckoutClient } from "./api";
import { liveClient } from "./live/client";
import { mockClient } from "./mock/client";

export function isMockMode(): boolean {
  return process.env.API_MODE !== "live";
}

export function getClient(): CheckoutClient {
  if (isMockMode()) {
    return mockClient;
  }
  return liveClient;
}
