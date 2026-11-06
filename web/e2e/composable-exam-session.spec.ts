import { test, expect } from "@playwright/test";
import { seedSession } from "./helpers/session";
import type { SessionState } from "../lib/types";

for (const viewport of [{ width:390, height:844 }, { width:1280, height:900 }]) {
  for (const scenario of [
    { presentation:"one_per_page", own:'<p>Own passage</p><img src="/material-fixture.svg" alt="Material fixture">', expected:"Own passage" },
    { presentation:"one_per_page", own:null, expected:"Shared passage" },
    { presentation:"all_on_page", own:'<p>Own passage</p><img src="/material-fixture.svg" alt="Material fixture">', expected:"Shared passage" },
  ] as const) {
    test(`${viewport.width}px ${scenario.presentation} ${scenario.own ? "own" : "fallback"} material and answers`, async ({ page, context }) => {
      const state:SessionState = {
        session_id:"composable-browser", registration_id:"registration", status:"in_progress", started_at:"2026-10-03T00:00:00Z",
        format_version:2, navigation_policy:"free", mode:"ielts", timer_mode:"overall", duration_minutes:60, remaining_seconds:3600, answers:[], current_position:0,
        tests:[{ id:"reading", title:"Reading section", subject:"English", question_presentation:scenario.presentation,
          shared_material_html:`<p>Shared passage</p><img src="/material-fixture.svg" alt="Material fixture"><table><tbody><tr><td>${"Long context ".repeat(35)}</td><td>Table context</td></tr></tbody></table>`, instructions_html:"<p>Read the instructions</p>",
          questions:[{id:"q1",test_id:"reading",format:"mcq",body:"Question one",material_html:scenario.own,sort_order:1,options:[{key:"a",text:"Answer A",sort_order:1},{key:"b",text:"Answer B",sort_order:2}]},
            {id:"q2",test_id:"reading",format:"essay",body:"Question two",sort_order:2,options:[]}] }],
      };
      await seedSession(context,{token:"fixture",refreshToken:"",user:{id:"student-fixture",role:"student",name:"Browser Student"}});
      await context.addInitScript(() => {
        Object.defineProperty(Element.prototype,"requestFullscreen",{ configurable:true,value:() => Promise.reject(new Error("layout fixture")) });
      });
      await page.route("**/material-fixture.svg", route => route.fulfill({ contentType:"image/svg+xml",body:'<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="200"><rect width="1200" height="200" fill="#dbeafe"/></svg>' }));
      const saves: {answers:{question_id:string;answer:string}[]}[] = [];
      await page.route("**/api/v1/**", async route => {
        const request=route.request();
        if (request.url().endsWith("/answers")) { saves.push(request.postDataJSON()); await route.fulfill({status:204}); return; }
        if (request.url().endsWith("/exam/sessions/composable-browser")) { await route.fulfill({json:state}); return; }
        await route.fulfill({json:{data:[]}});
      });
      await page.setViewportSize(viewport);
      await page.goto("/exam/sessions/composable-browser");
      await page.getByTestId("enter-fullscreen").click();
      await expect(page.getByText(scenario.expected,{exact:true})).toBeVisible();
      await expect(page.getByText("Read the instructions")).toBeVisible();
      await expect(page.getByText(scenario.expected === "Shared passage" ? "Own passage" : "Shared passage",{exact:true})).toHaveCount(0);
      if (scenario.presentation === "all_on_page") {
        await expect(page.getByText("Question two",{exact:true})).toBeVisible();
      } else { await expect(page.getByText("Question two",{exact:true})).toHaveCount(0); }
      const material=page.getByTestId("effective-material");
      await expect(material.getByRole("img",{name:"Material fixture"})).toBeVisible();
      const imageBox=await material.getByRole("img",{name:"Material fixture"}).boundingBox();
      expect(imageBox!.width).toBeGreaterThan(100);
      expect(imageBox!.width).toBeLessThanOrEqual(viewport.width);
      const box=await material.boundingBox();
      expect(box).not.toBeNull();
      expect(box!.width).toBeGreaterThan(200);
      expect(box!.x+box!.width).toBeLessThanOrEqual(viewport.width+1);
      const width=await page.evaluate(() => ({scroll:document.documentElement.scrollWidth,viewport:window.innerWidth}));
      expect(width.scroll).toBeLessThanOrEqual(width.viewport+1);
      await page.getByRole("radio").first().check();
      await expect.poll(() => saves.some(save => save.answers.some(answer => answer.question_id === "q1" && answer.answer === "a"))).toBe(true);
      if (scenario.presentation === "all_on_page") {
        await page.getByRole("textbox").fill("Essay response");
        await expect.poll(() => saves.some(save => save.answers.some(answer => answer.question_id === "q2" && answer.answer === "Essay response"))).toBe(true);
      }
    });
  }
}
