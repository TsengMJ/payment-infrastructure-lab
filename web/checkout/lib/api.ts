import type { CheckoutSession, ConfirmScenario, PaymentLink } from "./types";

export interface CheckoutClient {
  getSession(id: string): Promise<CheckoutSession | null>;
  confirm(id: string, scenario: ConfirmScenario): Promise<CheckoutSession>;
  completeAction(id: string): Promise<CheckoutSession>;
  getLink(id: string): Promise<PaymentLink | null>;
  startFromLink(id: string): Promise<CheckoutSession>;
}
