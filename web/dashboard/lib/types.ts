export type Role = "owner" | "admin" | "developer" | "support" | "viewer";

export type MerchantUser = {
  id: string;
  email: string;
  name: string;
  role: Role;
  merchantId: string;
  merchantName: string;
};

export type InternalUser = {
  id: string;
  email: string;
  name: string;
  canReview: boolean;
  canRefund: boolean;
};

export type PaymentSummary = {
  id: string;
  amount: number;
  currency: string;
  status: string;
  createdAt: string;
  summary: string;
};

export type TimelineEvent = {
  id: string;
  at: string;
  kind: string;
  title: string;
  detail: string;
};

export type SettlementRow = {
  id: string;
  gross: number;
  fee: number;
  platformFee: number;
  refund: number;
  dispute: number;
  net: number;
  currency: string;
};

export type SettlementReport = {
  periodLabel: string;
  currency: string;
  rows: SettlementRow[];
  payoutAmount: number;
};

export type WebhookDelivery = {
  id: string;
  eventId: string;
  type: string;
  status: "delivered" | "failed" | "replayed";
  attempts: number;
};

export type KybCase = {
  id: string;
  merchantName: string;
  status: "pending" | "approved" | "rejected";
  maskedOwner: string;
  vendorLabel: string;
  vendorUrl: string;
  screening: string;
  capability: string;
};

export type RefundRequest = {
  id: string;
  paymentId: string;
  amount: number;
  currency: string;
  reason: string;
  status: "pending" | "approved";
  makerId: string;
  makerName: string;
  checkerName: string | null;
};

export type AuditEntry = {
  id: string;
  at: string;
  actor: string;
  action: string;
  before: string;
  after: string;
  reason: string;
};

export type ActionState = {
  error: string;
};
