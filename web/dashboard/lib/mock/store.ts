import { DEMO_PASSWORD } from "../demo";
import type {
  AuditEntry,
  InternalUser,
  KybCase,
  PaymentSummary,
  RefundRequest,
  Role,
  TimelineEvent,
  WebhookDelivery,
} from "../types";

export type MerchantRecord = {
  id: string;
  email: string;
  name: string;
  role: Role;
  merchantId: string;
  merchantName: string;
  password: string;
  failures: number;
  locked: boolean;
};

export type InternalRecord = InternalUser & {
  email: string;
  password: string;
  failures: number;
  locked: boolean;
};

export type TokenRecord = {
  kind: "merchant" | "internal";
  userId: string;
};

export type Store = {
  merchants: MerchantRecord[];
  internals: InternalRecord[];
  tokens: Map<string, TokenRecord>;
  payments: PaymentSummary[];
  timelines: Map<string, TimelineEvent[]>;
  deliveries: Map<string, WebhookDelivery>;
  cases: KybCase[];
  viewedKyb: Set<string>;
  refunds: RefundRequest[];
  refundSeq: number;
  audit: AuditEntry[];
};

const STORE_KEY = "__paymentLabDashboardStore";

const MERCHANT = { merchantId: "mer_demo", merchantName: "示範商店" };

function merchant(
  id: string,
  email: string,
  name: string,
  role: Role,
): MerchantRecord {
  return {
    id,
    email,
    name,
    role,
    ...MERCHANT,
    password: DEMO_PASSWORD,
    failures: 0,
    locked: false,
  };
}

function story(): TimelineEvent[] {
  return [
    { id: "ev1", at: "2026-10-20T09:00:00+08:00", kind: "intent", title: "建立付款意圖", detail: "商戶建立 PaymentIntent" },
    { id: "ev2", at: "2026-10-20T09:00:05+08:00", kind: "attempt", title: "授權嘗試逾時", detail: "PSP 逾時，結果為 UNKNOWN" },
    { id: "ev3", at: "2026-10-20T09:01:00+08:00", kind: "recovery", title: "Recovery", detail: "向 PSP 查詢後確認授權成功" },
    { id: "ev4", at: "2026-10-20T09:01:02+08:00", kind: "webhook", title: "PSP webhook", detail: "收到授權成功通知" },
    { id: "ev5", at: "2026-10-20T09:01:03+08:00", kind: "risk", title: "風控決策", detail: "決策 allow" },
    { id: "ev6", at: "2026-10-20T09:01:04+08:00", kind: "ledger", title: "分錄", detail: "分錄已過帳" },
    { id: "ev7", at: "2026-10-20T10:00:00+08:00", kind: "refund", title: "退款完成", detail: "退款 9700 TWD" },
    { id: "ev8", at: "2026-10-21T08:00:00+08:00", kind: "reconciliation", title: "對帳", detail: "對帳相符" },
    { id: "ev9", at: "2026-10-21T08:00:05+08:00", kind: "notification", title: "商戶通知", detail: "商戶通知已送達" },
  ];
}

function seed(): Store {
  return {
    merchants: [
      merchant("usr_owner", "owner@demo.test", "陳擁有", "owner"),
      merchant("usr_admin", "admin@demo.test", "林管理", "admin"),
      merchant("usr_developer", "developer@demo.test", "張開發", "developer"),
      merchant("usr_support", "support@demo.test", "周客服", "support"),
      merchant("usr_viewer", "viewer@demo.test", "吳檢視", "viewer"),
      merchant("usr_lock", "lock@demo.test", "鎖定示範", "viewer"),
    ],
    internals: [
      {
        id: "usr_reviewer",
        email: "reviewer@internal.test",
        name: "林審核",
        canReview: true,
        canRefund: false,
        password: DEMO_PASSWORD,
        failures: 0,
        locked: false,
      },
      {
        id: "usr_agent1",
        email: "agent1@internal.test",
        name: "陳客服",
        canReview: false,
        canRefund: true,
        password: DEMO_PASSWORD,
        failures: 0,
        locked: false,
      },
      {
        id: "usr_agent2",
        email: "agent2@internal.test",
        name: "黃覆核",
        canReview: false,
        canRefund: true,
        password: DEMO_PASSWORD,
        failures: 0,
        locked: false,
      },
    ],
    tokens: new Map(),
    payments: [
      {
        id: "pay_story",
        amount: 9700,
        currency: "TWD",
        status: "已退款",
        createdAt: "2026-10-20T09:00:00+08:00",
        summary: "逾時後 recovery，再退款並對帳",
      },
      {
        id: "pay_ok",
        amount: 120000,
        currency: "TWD",
        status: "成功",
        createdAt: "2026-10-21T11:00:00+08:00",
        summary: "一般授權",
      },
    ],
    timelines: new Map([
      ["pay_story", story()],
      [
        "pay_ok",
        [
          {
            id: "ok1",
            at: "2026-10-21T11:00:00+08:00",
            kind: "intent",
            title: "建立付款意圖",
            detail: "授權成功",
          },
        ],
      ],
    ]),
    deliveries: new Map([
      ["wh_failed", { id: "wh_failed", eventId: "evt_1", type: "payment.succeeded", status: "failed", attempts: 3 }],
      ["wh_ok", { id: "wh_ok", eventId: "evt_2", type: "refund.succeeded", status: "delivered", attempts: 1 }],
      ["wh_support", { id: "wh_support", eventId: "evt_3", type: "payment.updated", status: "failed", attempts: 2 }],
    ]),
    cases: [
      {
        id: "kyb_001",
        merchantName: "示範商店",
        status: "pending",
        maskedOwner: "王＊明",
        vendorLabel: "在驗證廠商開啟",
        vendorUrl: "https://vendor.example/cases/kyb_001",
        screening: "無命中",
        capability: "未開通",
      },
    ],
    viewedKyb: new Set(),
    refunds: [],
    refundSeq: 0,
    audit: [
      {
        id: "aud_1",
        at: "2026-10-18T09:00:00+08:00",
        actor: "系統",
        action: "角色變更",
        before: "viewer",
        after: "support",
        reason: "職務調整",
      },
      {
        id: "aud_2",
        at: "2026-10-19T09:00:00+08:00",
        actor: "系統",
        action: "調帳",
        before: "0",
        after: "-100",
        reason: "對帳差異",
      },
      {
        id: "aud_3",
        at: "2026-10-19T10:00:00+08:00",
        actor: "系統",
        action: "規則變更",
        before: "v1",
        after: "v2",
        reason: "風控規則更新",
      },
    ],
  };
}

function globalStore(): typeof globalThis & { [STORE_KEY]?: Store } {
  return globalThis as typeof globalThis & { [STORE_KEY]?: Store };
}

export function getStore(): Store {
  const holder = globalStore();
  if (!holder[STORE_KEY]) {
    holder[STORE_KEY] = seed();
  }
  return holder[STORE_KEY];
}

export function resetStore(): void {
  globalStore()[STORE_KEY] = seed();
}
