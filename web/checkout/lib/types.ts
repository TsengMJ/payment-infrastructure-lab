export type MethodKind = "card" | "wallet" | "bank_transfer";

export type PaymentMethod = {
  kind: MethodKind;
  label: string;
};

export type SessionStatus =
  | "open"
  | "requires_action"
  | "processing"
  | "succeeded"
  | "failed"
  | "expired";

export type ConfirmScenario = "success" | "3ds" | "decline" | "processing";

export type CheckoutSession = {
  id: string;
  status: SessionStatus;
  amount: number;
  currency: string;
  methods: PaymentMethod[];
  paymentId: string | null;
  linkId: string | null;
};

export type PaymentLink = {
  id: string;
  title: string;
  reusable: boolean;
  status: "active" | "used";
  amount: number;
  currency: string;
};

export type ActionState = {
  error: string;
};
