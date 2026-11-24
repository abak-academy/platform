# Redesign v2 prototype

`index.html` is a self-contained, clickable prototype (open it in a browser, no build needed). Panca reviewed and approved it on 2026-11-17. It is the visual and behavioral reference for the redesign phases in `docs/specs/redesign-v2-roadmap.md`.

- **Use it for:** layout, hierarchy, copy, flows, and interaction behavior.
- **Do not use it for:** code. The prototype is vanilla JS with sample data. Build with `web/components/ui`, the tokens in `docs/specs/design-tokens-v2.md`, and the inventory in `COMPONENTS.md`.
- **Demo bar** (bottom right): switches between Guest, Student, and Admin. The reset button restores the sample data.
- **Assets:**
  - `avatars-hd.webp`: the upscaled avatar sheet (7×5 grid, 320px per face, transparent outside the circle).
  - `avatars/01.webp…35.webp`: the same faces as separate files, ready to upload to `avatars/sheet/` (see `docs/specs/profile-avatars.md`). Source: supplied by Panca; open license.
  - `event-series-seed.json`: content for the 10 events, Indonesian taken from each official site on 2026-11-17, English as a draft translation (see `docs/specs/event-series.md`). Review both before publishing.
  - Testimonials in the prototype are sample data marked "Data contoh". Real ones come through the admin page in `docs/specs/testimonials.md`.

Key screens: `/` (landing), `#/kompetisi` (Event page), student home, `#/app/kompetisi` (Kompetisi Saya), `#/app/kursus` (Belajar), `#/admin` and every admin list, `#/admin/event/<id>` (event editor, ID/EN tabs), `#/admin/testimoni`.
