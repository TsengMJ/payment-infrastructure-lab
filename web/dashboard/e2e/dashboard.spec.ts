import { expect, type Page, test } from "@playwright/test";

test.beforeEach(async ({ request }) => {
  const response = await request.post("/api/test/reset");
  expect(response.status()).toBe(204);
});

async function login(page: Page, email: string, path = "/login") {
  await page.goto(path);
  await page.getByLabel("電子郵件").fill(email);
  await page.getByLabel("密碼").fill("local-demo-only");
  await page.getByRole("button", { name: "登入" }).click();
  await expect(page.getByRole("button", { name: "登出" })).toBeVisible();
}

test("密碼錯誤會鎖定", async ({ page }) => {
  test.setTimeout(60_000);
  await page.goto("/login");
  const submit = page.getByRole("button", { name: "登入" });
  for (let attempt = 0; attempt < 5; attempt += 1) {
    await expect(submit).toBeEnabled();
    await page.getByLabel("電子郵件").fill("lock@demo.test");
    await page.getByLabel("密碼").fill("wrong-password");
    const submitted = page.waitForResponse((response) => response.request().method() === "POST");
    await submit.click();
    await submitted;
    await expect(submit).toBeEnabled();
    await expect(page.locator("p[role=alert]")).toHaveText(attempt === 4 ? "帳號已鎖定" : "密碼不正確");
  }
  await page.getByLabel("電子郵件").fill("lock@demo.test");
  await page.getByLabel("密碼").fill("local-demo-only");
  const submitted = page.waitForResponse((response) => response.request().method() === "POST");
  await submit.click();
  await submitted;
  await expect(page.locator("p[role=alert]")).toHaveText("帳號已鎖定");
  await expect(page).toHaveURL(/\/login$/);
});

test("登出後不能再看付款", async ({ page }) => {
  await login(page, "viewer@demo.test");
  await page.getByRole("button", { name: "登出" }).click();
  await expect(page).toHaveURL(/\/login$/);
  await page.goto("/");
  await expect(page).toHaveURL(/\/login$/);
});

test("時間線包含逾時、recovery、退款與對帳", async ({ page }) => {
  await login(page, "owner@demo.test");
  await page.goto("/payments/pay_story");
  await expect(page.getByText("UNKNOWN")).toBeVisible();
  await expect(page.getByText("查詢後確認授權成功")).toBeVisible();
  await expect(page.getByText("退款完成")).toBeVisible();
  await expect(page.getByText("對帳相符")).toBeVisible();
  await expect(page.getByText("風控決策")).toBeVisible();
  await expect(page.getByText("分錄已過帳")).toBeVisible();
  await expect(page.getByText("商戶通知已送達")).toBeVisible();
});

test("報表 net 加總等於出款", async ({ page }) => {
  await login(page, "viewer@demo.test");
  await page.goto("/reports");
  const nets = await page.getByTestId("row-net").allTextContents();
  const sum = nets.reduce((total, value) => total + Number(value), 0);
  await expect(page.getByTestId("payout-amount")).toHaveText(String(sum));
});

test("viewer 不能重送 webhook", async ({ page }) => {
  await login(page, "viewer@demo.test");
  await page.goto("/webhooks");
  await expect(page.getByRole("button", { name: "重送" })).toHaveCount(0);
  const response = await page.request.post("/api/webhook-deliveries/wh_failed/replay");
  expect(response.status()).toBe(403);
});

test("developer 可以重送 webhook", async ({ page }) => {
  await login(page, "developer@demo.test");
  await page.goto("/webhooks");
  const row = page.getByRole("row", { name: /evt_1/ });
  await row.getByRole("button", { name: "重送" }).click();
  await expect(row).toContainText("已重送");
  await expect(row).toContainText("evt_1");
});

test("商戶帳號進不了內部審核", async ({ page }) => {
  await login(page, "owner@demo.test");
  await page.goto("/internal/kyb");
  await expect(page).toHaveURL(/\/internal\/login$/);
  await expect(page.getByRole("heading", { name: "內部人員登入" })).toBeVisible();
  await expect(page.getByText("可收款")).toHaveCount(0);
});

test("審核只看遮罩個資，核准後能力生效", async ({ page }) => {
  await login(page, "reviewer@internal.test", "/internal/login");
  await expect(page.getByText("王＊明")).toBeVisible();
  await expect(page.getByRole("link", { name: "在驗證廠商開啟" })).toBeVisible();
  await expect(page.getByText("身分證")).toHaveCount(0);
  await page.getByRole("button", { name: "核准" }).click();
  await expect(page.getByTestId("capability")).toHaveText("可收款");
  await page.goto("/internal/audit");
  await expect(page.getByText("查看入駐申請")).toBeVisible();
  await expect(page.getByText("核准入駐")).toBeVisible();
  await expect(page.getByRole("button", { name: "刪除" })).toHaveCount(0);
});

test("手動退款要第二人核准", async ({ browser }) => {
  const makerContext = await browser.newContext();
  const maker = await makerContext.newPage();
  await login(maker, "agent1@internal.test", "/internal/login");
  await maker.goto("/internal/support");
  await maker.getByLabel("查詢付款編號").fill("pay_story");
  await maker.getByRole("button", { name: "查詢" }).click();
  await expect(maker.getByText("UNKNOWN")).toBeVisible();
  await maker.getByLabel("付款編號", { exact: true }).fill("pay_story");
  await maker.getByLabel("金額（最小貨幣單位）").fill("1000");
  await maker.getByLabel("理由").fill("客戶要求");
  await maker.getByRole("button", { name: "送出退款" }).click();
  await expect(maker.getByText("待覆核")).toBeVisible();
  await maker.getByRole("button", { name: "核准" }).click();
  await expect(maker.locator("p[role=alert]")).toHaveText("不能自己核准");

  const checkerContext = await browser.newContext();
  const checker = await checkerContext.newPage();
  await login(checker, "agent2@internal.test", "/internal/login");
  await checker.goto("/internal/support");
  await checker.getByRole("button", { name: "核准" }).click();
  await expect(checker.getByText("已核准")).toBeVisible();
  await makerContext.close();
  await checkerContext.close();
});
