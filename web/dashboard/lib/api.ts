import type {
  AuditEntry,
  InternalUser,
  KybCase,
  MerchantUser,
  PaymentSummary,
  RefundRequest,
  SettlementReport,
  TimelineEvent,
  WebhookDelivery,
} from "./types";

export interface DashboardClient {
  loginMerchant(email: string, password: string): Promise<{ token: string; user: MerchantUser }>;
  loginInternal(email: string, password: string): Promise<{ token: string; user: InternalUser }>;
  merchantFromToken(token: string): Promise<MerchantUser | null>;
  internalFromToken(token: string): Promise<InternalUser | null>;
  listPayments(): Promise<PaymentSummary[]>;
  getTimeline(paymentId: string): Promise<TimelineEvent[]>;
  getSettlement(): Promise<SettlementReport>;
  listDeliveries(): Promise<WebhookDelivery[]>;
  replayDelivery(actor: MerchantUser, deliveryId: string): Promise<WebhookDelivery>;
  replayAsInternal(actor: InternalUser, deliveryId: string): Promise<WebhookDelivery>;
  listKyb(actor: InternalUser): Promise<KybCase[]>;
  decideKyb(actor: InternalUser, caseId: string, decision: "approved" | "rejected"): Promise<KybCase>;
  listRefunds(): Promise<RefundRequest[]>;
  requestRefund(actor: InternalUser, paymentId: string, amount: number, reason: string): Promise<RefundRequest>;
  approveRefund(actor: InternalUser, refundId: string): Promise<RefundRequest>;
  listAudit(): Promise<AuditEntry[]>;
}
