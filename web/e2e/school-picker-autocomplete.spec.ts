import { test, expect } from "@playwright/test";
import { seedSession } from "./helpers/session";

const school = {
  id: "school-taman", name: "TK TAMAN BELAJAR", code: "20561417", npsn: "20561417",
  school_types: ["TK"], provinsi_id: "35", kota_id: "3578",
  kota_name: "KOTA SURABAYA", provinsi_name: "JAWA TIMUR",
};

for (const viewport of [{ width: 1280, height: 900 }, { width: 390, height: 844 }]) {
  test(`autocomplete chooses and submits the school at ${viewport.width}px`, async ({ page, context }, testInfo) => {
    await page.setViewportSize(viewport);
    const user = { id: "student", role: "student", name: "Picker Test", auth_provider: "google" };
    await seedSession(context, { token: "test-token", refreshToken: "", user });
    const searches: URLSearchParams[] = [];
    let saved: Record<string, unknown> | undefined;
    await page.route("**/api/v1/**", async (route) => {
      const url = new URL(route.request().url());
      let body: unknown = {};
      if (url.pathname.endsWith("/students/profile")) {
        if (route.request().method() === "PATCH") saved = route.request().postDataJSON();
        body = user;
      } else if (url.pathname.endsWith("/provinces")) {
        body = [{ id: "35", name: "JAWA TIMUR" }];
      } else if (url.pathname.endsWith("/provinces/35/cities")) {
        body = [{ id: "3578", province_id: "35", name: "KOTA SURABAYA" }];
      } else if (url.pathname.endsWith("/schools")) {
        searches.push(url.searchParams);
        body = { data: [school] };
      }
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(body) });
    });
    await page.goto("/complete-profile");
    const input = page.getByRole("combobox", { name: "Nama sekolah", exact: true });
    await input.fill("ta");
    await expect(page.getByText("Ketik minimal 3 karakter nama sekolah.")).toBeVisible();
    expect(searches).toHaveLength(0);
    await input.fill("taman belajar");
    const option = page.getByRole("option", { name: /TK TAMAN BELAJAR/ });
    await expect(option).toBeVisible();
    expect(searches.at(-1)?.get("q")).toBe("taman belajar");
    expect(searches.at(-1)?.has("province_id")).toBe(false);
    await page.screenshot({ path: testInfo.outputPath("autocomplete.png"), fullPage: true });
    await input.press("ArrowDown");
    await input.press("Enter");
    await expect(input).toHaveCount(0);
    await expect(page.getByRole("group", { name: "Sekolah terpilih" })).toContainText(school.name);
    await expect(page.getByRole("button", { name: "Ganti sekolah" })).toBeFocused();
    await expect(page.getByRole("listbox")).toHaveCount(0);
    await expect(page.getByText("20561417 · TK · KOTA SURABAYA · JAWA TIMUR", { exact: true })).toBeVisible();
    await expect(page.getByRole("combobox", { name: "Provinsi", exact: true })).toHaveText("JAWA TIMUR");
    await expect(page.getByRole("combobox", { name: "Kota/Kabupaten", exact: true })).toHaveText("KOTA SURABAYA");
    for (const name of ["Provinsi", "Kota/Kabupaten", "Jenis Sekolah"]) {
      await expect(page.getByRole("combobox", { name, exact: true })).toBeDisabled();
    }
    expect(saved).toBeUndefined();
    await page.screenshot({ path: testInfo.outputPath("selected.png"), fullPage: true });
    await page.locator("#onboard-grade").click();
    await page.getByRole("option", { name: "7", exact: true }).click();
    await page.locator('button[type="submit"]').click();
    await expect.poll(() => saved?.school_id).toBe(school.id);
    expect(saved?.unlisted_school_name).toBe("");
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  });
}

test("province and city narrow autocomplete and selecting a new name clears the old choice", async ({ page, context }) => {
  const user = { id: "student", role: "student", name: "Picker Test", auth_provider: "google" };
  await seedSession(context, { token: "test-token", refreshToken: "", user });
  const searches: URLSearchParams[] = [];
  const cityRequests: string[] = [];
  await page.route("**/api/v1/**", async (route) => {
    const url = new URL(route.request().url());
    let body: unknown = {};
    if (url.pathname.endsWith("/cities")) cityRequests.push(url.pathname);
    if (url.pathname.endsWith("/students/profile")) body = user;
    else if (url.pathname.endsWith("/provinces")) body = [{ id: "35", name: "JAWA TIMUR" }];
    else if (url.pathname.endsWith("/provinces/35/cities")) body = [{ id: "3578", province_id: "35", name: "KOTA SURABAYA" }];
    else if (url.pathname.endsWith("/schools")) {
      searches.push(url.searchParams);
      body = { data: url.searchParams.get("q") === "unknown" ? [] : [school] };
    }
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(body) });
  });
  await page.goto("/complete-profile");
  await expect(page.getByRole("combobox", { name: "Kota/Kabupaten", exact: true })).toBeDisabled();
  expect(cityRequests).not.toContain("/api/v1/cities");
  await page.getByRole("combobox", { name: "Provinsi", exact: true }).click();
  await page.getByRole("option", { name: "JAWA TIMUR", exact: true }).click();
  await page.getByRole("combobox", { name: "Kota/Kabupaten", exact: true }).click();
  await page.getByRole("option", { name: "KOTA SURABAYA", exact: true }).click();
  const input = page.getByRole("combobox", { name: "Nama sekolah", exact: true });
  await input.fill("taman");
  await expect(page.getByRole("option", { name: /TK TAMAN BELAJAR/ })).toBeVisible();
  expect(searches.at(-1)?.get("city_id")).toBe("3578");
  expect(searches.at(-1)?.get("province_id")).toBe("35");
  expect(cityRequests).toEqual(["/api/v1/provinces/35/cities"]);
  expect(searches.at(-1)?.has("school_type")).toBe(false);
  await page.getByRole("option", { name: /TK TAMAN BELAJAR/ }).click();
  await page.getByRole("button", { name: "Ganti sekolah" }).click();
  await expect(input).toHaveValue("");
  await expect(input).toBeFocused();
  await expect(page.getByRole("combobox", { name: "Kota/Kabupaten", exact: true })).toBeDisabled();
  expect(cityRequests).not.toContain("/api/v1/cities");
  await page.getByRole("combobox", { name: "Provinsi", exact: true }).click();
  await page.getByRole("option", { name: "JAWA TIMUR", exact: true }).click();
  await page.getByRole("combobox", { name: "Kota/Kabupaten", exact: true }).click();
  await page.getByRole("option", { name: "KOTA SURABAYA", exact: true }).click();
  await input.fill("taman");
  await expect(page.getByRole("option", { name: /TK TAMAN BELAJAR/ })).toBeVisible();
  expect(searches.filter(query => query.get("q") === "taman" && query.get("city_id") === "3578")).toHaveLength(1);
  await input.fill("unknown");
  await expect(page.getByText("Sekolah tidak ditemukan.", { exact: true })).toBeVisible();
  await expect(page.getByText("20561417 · TK · KOTA SURABAYA · JAWA TIMUR", { exact: true })).toHaveCount(0);
});
