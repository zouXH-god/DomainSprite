import { expect, test } from "@playwright/test";

test("connection screen is keyboard accessible", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "登录域名控制台" })).toBeVisible();
  await expect(page.getByRole("button", { name: "登录" })).toBeVisible();
  await page.keyboard.press("Tab");
  await expect(page.locator(":focus")).toBeVisible();
});

test("quick DDNS does not load stored management credentials", async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem("ds.accessKeyId", "must-not-render");
  });
  await page.goto("/quick");
  await expect(page.getByRole("heading", { name: "让地址始终指向这里。" })).toBeVisible();
  await expect(page.getByText("must-not-render")).toHaveCount(0);
});
