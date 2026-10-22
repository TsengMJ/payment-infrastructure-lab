import "server-only";

import type { DashboardClient } from "./api";
import { liveClient } from "./live/client";
import { mockClient } from "./mock/client";

export function getClient(): DashboardClient {
  if (process.env.API_MODE === "live") {
    return liveClient;
  }
  return mockClient;
}
