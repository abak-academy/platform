---
title: Admin navigation regrouping and accordion
issue: none yet
status: proposed
created: 2026-11-17
sources: ["web/lib/nav-config.ts", "web/lib/nav-config.test.ts", "web/components/shell/AppSidebar.tsx", "web/lib/i18n.ts", "docs/prototypes/redesign-v2/index.html"]
---

# Admin navigation regrouping and accordion

## Problem and outcome

The super-admin sidebar has six groups (Toko, Ujian, Sekolah, Affiliator, Sistem, plus Beranda) and every group is expanded, so the list is long and the order follows the code, not the daily work. Ujian and Sekolah are separate even though admins move between them constantly.

Outcome: four collapsible groups in work order. Only one group is open at a time, with an animated open/close. The group containing the current page opens automatically.

## Agreed behavior

Decision (Panca, 2026-11-17): Event, Ujian, and Sekolah become one group named **Akademik**.

Super-admin order:

1. **Beranda** (`/admin`), standalone, always visible.
2. **Akademik**: Event *(added by the event-series phase; omit until it exists)* · Testimoni *(added by the testimonials phase; omit until it exists)* · Ujian (`/admin/exam/packages`) · Monitor Sesi · Tes · Bank Soal · Sekolah · Siswa · Pengumuman (`/admin/notifications`, renamed from Notifikasi).
3. **Toko**: Pesanan · Produk · Kursus · Promo · Ringkasan penjualan (`/admin/store`) · Pendapatan.
4. **Affiliator**: Affiliator · Penarikan · Laporan · Pengaturan.
5. **Sistem**: Akun & peran · Konfigurasi · Audit.

Why this order: groups follow how often each area is used (competitions first, then store operations), and inside a group the items follow the workflow (set up → run → monitor → content → people). Within Toko, the order queue comes first because it is the daily task. Summary and revenue come last because they are for reporting.

Other admin roles (`ADMIN_EXAM_NAV`, `ADMIN_SCHOOL_NAV`, store admin) use the same group names and order, filtered to the items their role can see. A group with zero visible items is not rendered.

Accordion behavior:

- A group header shows icon, label, and a chevron. When collapsed, it also shows the sum of its items' attention counts (for example orders waiting for confirmation).
- Clicking a closed header opens it and closes the others. Clicking the open header closes it.
- On navigation, the group that contains the active route is open. A manual open or close holds until the next route change.
- Open/close animates height (grid-rows 0fr → 1fr, about 350ms) and rotates the chevron. No animation under `prefers-reduced-motion`.
- The collapsed rail mode (`collapsed` prop) is unchanged: icons only, no groups.
- Keyboard: the header is a button with `aria-expanded` and `aria-controls`.

## Technical contract

- `nav-config.ts`: replace `nav_group_exam` + `nav_group_school` with `nav_group_academic` in every role config; reorder items as above. Add an optional `group.icon`.
- `AppSidebar.tsx`: replace `open: boolean[]` with `openKey: string | null`, derived from the active route on pathname change; keep the existing `isNavActive`.
- i18n: add `nav_group_academic` (id "Akademik", en "Academics") and rename the notifications nav label to "Pengumuman" / "Announcements". Remove the orphaned `nav_group_exam` / `nav_group_school` keys after grepping all usages and tests.
- No route changes.

## Acceptance

1. Super admin sees Beranda + 4 groups in the order above. `nav-config.test.ts` asserts the order per role.
2. Opening one group closes the others (component test).
3. Visiting `/admin/exam/tests` directly renders Akademik open and the others closed.
4. A collapsed group with attention counts shows their sum.
5. Reduced motion: no transition classes applied (test via `matchMedia` mock).

## Non-goals

- The Event menu item itself (see the event-series phase).
- Restyling sidebar rows beyond what `design-tokens-v2.md` gives for free.
