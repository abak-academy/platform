import { test, expect } from "@playwright/test";
import { seedSession } from "./helpers/session";

for (const viewport of [{ width: 390, height: 844 }, { width: 1280, height: 900 }]) {
  test(`${viewport.width}px admin saves presentation and clears shared material`, async ({ page, context }) => {
    await seedSession(context, { token: "fixture", refreshToken: "", user: { id: "admin-fixture", role: "admin_exam", name: "Browser Admin" } });
    const section = { id: "reading", title: "Reading section", subject: "English", topic: "Reading", duration_minutes: 30, shared_material_html: "<p>Shared passage</p>", instructions_html: "<p>Read carefully</p>" };
    let presentation = "one_per_page";
    let attachmentPayload: unknown;
    let materialPayload: Record<string, unknown> | undefined;
    await page.route("**/api/v1/**", async route => {
      const request = route.request();
      const path = new URL(request.url()).pathname;
      if (request.method() === "PUT" && path.endsWith("/admin/exams/composable/tests")) {
        attachmentPayload = request.postDataJSON();
        presentation = "all_on_page";
        await route.fulfill({ status: 204 });
      } else if (request.method() === "PATCH" && path.endsWith("/admin/tests/reading")) {
        materialPayload = request.postDataJSON();
        await route.fulfill({ json: { ...section, ...materialPayload } });
      } else if (path.endsWith("/admin/exams/composable")) {
        await route.fulfill({ json: { id: "composable", title: "Composable exam", format_version: 2, navigation_policy: "free", timer_mode: "overall", duration_minutes: 60, mode: "standard", status: "draft", card_notes: [], tests: [{ exam_id: "composable", test_id: "reading", sort_order: 0, question_presentation: presentation, test: section }] } });
      } else if (path.endsWith("/admin/tests")) {
        await route.fulfill({ json: { data: [section], total: 1 } });
      } else {
        await route.fulfill({ json: { data: [] } });
      }
    });
    await page.setViewportSize(viewport);
    await page.goto("/admin/exam/packages/composable");
    await page.getByRole("button", { name: "Tes", exact: true }).click();
    const selector = page.getByRole("combobox", { name: "Tampilan soal Reading section" });
    await selector.selectOption("all_on_page");
    await page.getByRole("button", { name: "Simpan", exact: true }).click();
    await expect.poll(() => attachmentPayload).toEqual([{ test_id: "reading", question_presentation: "all_on_page" }]);
    await page.reload();
    await page.getByRole("button", { name: "Tes", exact: true }).click();
    await expect(selector).toHaveValue("all_on_page");
    const overflow = await page.evaluate(() => Array.from(document.querySelectorAll("main *")).filter(el => el.getBoundingClientRect().right > innerWidth + 1).map(el => ({ tag: el.tagName, class: el.className, right: el.getBoundingClientRect().right })).slice(0, 8));
    expect(overflow).toEqual([]);
    await page.goto("/admin/exam/tests");
    await page.getByRole("button", { name: "Edit", exact: true }).click();
    const shared = page.getByRole("textbox", { name: "Materi bersama", exact: true });
    await expect(shared).toContainText("Shared passage");
    await shared.fill("");
    await page.getByRole("dialog").getByRole("button", { name: /Simpan/ }).click();
    await expect.poll(() => materialPayload?.shared_material_html).toBeNull();
    expect(materialPayload?.instructions_html).toBe("<p>Read carefully</p>");
  });
}

for (const viewport of [{ width: 390, height: 844 }, { width: 1280, height: 900 }]) {
  for (const status of ["in_progress", "overdue"]) {
    test(`${viewport.width}px monitoring adds outage compensation to ${status} session`, async ({ page, context }) => {
      await seedSession(context, { token: "fixture", refreshToken: "", user: { id: "admin-fixture", role: "admin_exam", name: "Browser Admin" } });
      let requestBody: unknown;
      await page.route("**/api/v1/**", async route => {
        const request = route.request();
        const path = new URL(request.url()).pathname;
        if (path.endsWith("/admin/sessions/target-session/reopen")) {
          requestBody = request.postDataJSON();
          await route.fulfill({ json: { status: "ok" } });
        } else if (path.endsWith("/admin/sessions/monitor/available")) {
          await route.fulfill({ json: { data: [{ id: "exam", title: "Outage exam", state: "live", total_registered: 1, active_count: 1, not_started_count: 0 }] } });
        } else if (path.endsWith("/admin/sessions/monitor")) {
          await route.fulfill({ json: { exam: { id: "exam", title: "Outage exam", duration_minutes: 60 }, violations_recent: [], rows: [{ registration_id: "reg", student_id: "student", student_name: "Affected student", school_name: "School", status, answers_saved: 1, total_questions: 2, violation_count: 0, session_id: "target-session", admin_submitted: false, extended_until: null }] } });
        } else { await route.fulfill({ json: { data: [] } }); }
      });
      await page.setViewportSize(viewport);
      await page.goto("/admin/exam/monitor");
      await page.getByRole("button", { name: "Pantau", exact: true }).click();
      await expect(page.getByText("Affected student", { exact: true })).toBeVisible();
      page.once("dialog", dialog => dialog.accept("15"));
      await page.getByRole("button", { name: "Perpanjang", exact: true }).click();
      await expect.poll(() => requestBody).toEqual({ extend_minutes: 15 });
      await expect(page.getByText(/Waktu sesi ditambah/)).toBeVisible();
    });
  }
}
