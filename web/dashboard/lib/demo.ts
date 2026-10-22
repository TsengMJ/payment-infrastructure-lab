export const DEMO_PASSWORD = "local-demo-only";

export const MERCHANT_ACCOUNTS = [
  { email: "owner@demo.test", role: "擁有者" },
  { email: "admin@demo.test", role: "管理員" },
  { email: "developer@demo.test", role: "開發者" },
  { email: "support@demo.test", role: "客服" },
  { email: "viewer@demo.test", role: "檢視者" },
  { email: "lock@demo.test", role: "檢視者，用來示範鎖定" },
];

export const INTERNAL_ACCOUNTS = [
  { email: "reviewer@internal.test", role: "審核" },
  { email: "agent1@internal.test", role: "客服，可發起退款" },
  { email: "agent2@internal.test", role: "客服，可覆核退款" },
];
