import { expect, test } from "@playwright/test";

test.beforeEach(async ({ request }) => {
  const response = await request.post("/api/test/reset");
  expect(response.status()).toBe(204);
});

test("成功付款後重新整理仍是同一筆", async ({ page }) => {
  await page.goto("/c/cs_success");
  await expect(page.getByTestId("amount-minor")).toHaveText("120000 TWD");
  await expect(page.locator('input[name="amount"]')).toHaveCount(0);
  await page.getByRole("button", { name: "付款成功" }).click();
  await expect(page.getByTestId("payment-status")).toHaveText("付款成功");
  const paymentId = await page.getByTestId("payment-id").textContent();
  expect(paymentId).toBeTruthy();
  await page.reload();
  await expect(page.getByTestId("payment-id")).toHaveText(paymentId ?? "");
  await expect(page.getByTestId("payment-status")).toHaveText("付款成功");
});

test("3DS 完成後以伺服器狀態顯示成功", async ({ page }) => {
  await page.goto("/c/cs_3ds");
  await page.getByRole("button", { name: "需要 3DS" }).click();
  await expect(page.getByRole("heading", { name: "持卡人驗證" })).toBeVisible();
  await page.getByRole("button", { name: "完成驗證" }).click();
  await expect(page.getByTestId("payment-status")).toHaveText("付款成功");
});

test("處理中不會被頁面標成成功", async ({ page }) => {
  await page.goto("/c/cs_processing_case");
  await page.getByRole("button", { name: "處理中" }).click();
  await expect(page.getByTestId("payment-status")).toHaveText("處理中");
  await expect(page.getByText("webhook")).toBeVisible();
});

test("拒絕會停在失敗", async ({ page }) => {
  await page.goto("/c/cs_decline");
  await page.getByRole("button", { name: "拒絕" }).click();
  await expect(page.getByTestId("payment-status")).toHaveText("付款失敗");
});

test("過期 session 不能付款", async ({ page }) => {
  await page.goto("/c/cs_expired");
  await expect(page.getByRole("button", { name: "付款成功" })).toHaveCount(0);
  await expect(page.getByText("結帳已過期")).toBeVisible();
});

test("JPY 不顯示不支援的支付方式", async ({ page }) => {
  await page.goto("/c/cs_jpy");
  await expect(page.getByTestId("method-card")).toBeVisible();
  await expect(page.getByTestId("method-wallet")).toHaveCount(0);
  await expect(page.getByTestId("method-bank_transfer")).toHaveCount(0);
});

test("一次性連結付款後失效", async ({ page }) => {
  await page.goto("/pay/plink_once");
  await page.getByRole("button", { name: "開始付款" }).click();
  await page.getByRole("button", { name: "付款成功" }).click();
  await expect(page.getByTestId("payment-status")).toHaveText("付款成功");
  await page.goto("/pay/plink_once");
  await expect(page.getByText("這個連結已經使用過")).toBeVisible();
});

test("可重複連結每次產生新的結帳", async ({ page }) => {
  await page.goto("/pay/plink_reusable");
  await page.getByRole("button", { name: "開始付款" }).click();
  await expect(page).toHaveURL(/\/c\/cs_plink_reusable_/);
  const first = page.url();
  await page.goto("/pay/plink_reusable");
  await page.getByRole("button", { name: "開始付款" }).click();
  await expect(page).toHaveURL(/\/c\/cs_plink_reusable_/);
  expect(page.url()).not.toBe(first);
});
