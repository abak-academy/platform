# E4 — Participants & schools

| | |
|---|---|
| **Issue** | [#59](https://github.com/abak-academy/platform/issues/59) |
| **Objective** | A student whose school is not on the list is a first-class participant — findable, registerable, and able to edit their own profile. |
| **Source IDs** | FB-12, FB-14, FB-18, NF-3 |
| **Client items** | 4 |
| **Depends on** | E1 (B-8 — the importer copies a vulnerable writer) |
| **Verified against** | `main` @ `211b7b1`, 2026-07-29 |

Three of the four items are the same underlying fact wearing different clothes: **a student may have
no `school_id` at all**, carrying only a free-text `unlisted_school_name`. Every surface that assumes
a school FK exists then quietly loses them.

---

## 1. FB-12 — new students with an unregistered school don't appear in the participant list

The roster query was already fixed for this — `GetExamRoster` treats a null filter as "all schools"
rather than joining a student out of existence
([`exam.go:1245`](../../backend/internal/repository/exam.go)):

```sql
WHERE reg.exam_id = $1 AND ($2::uuid IS NULL OR u.school_id = $2)
```

The **picker** was not.
[`ParticipantPicker.tsx:52-98`](../../web/components/admin/ParticipantPicker.tsx) is built from a
school facet plus a cross-school search, and a student with `school_id IS NULL` falls between the two
— the facet has no bucket for them and the cross-school search is reached through the same school
dimension.

---

## 2. FB-14 — editing the profile fails when changing the school name

Two candidate causes, both real, both in
[`student.go:189-234`](../../backend/internal/service/student.go). Reproduce before fixing — the
client's report ("registered with a new school, then the profile won't update") matches the second.

**Cause A — an empty string is not "no school".** `schoolID` is parsed as a UUID whenever it is
non-nil (`:198-202`), so an empty value from the form is rejected outright instead of being read as
"no listed school":

```go
if schoolID != nil {
    if _, err := uuid.Parse(*schoolID); err != nil {
        return nil, ErrInvalidUUID
    }
}
```

**Cause B — jenjang is validated against the school being left behind.** The resolved school falls
back to the student's **existing** `school_id` when the submission has none (`:216-234`). Switching
from a listed school to an unlisted one therefore validates the new jenjang against the *old*
school's `school_types`, and fails for any student whose new school is a different level.

The frontend already models the case correctly — `UNLISTED_SCHOOL_VALUE = "_unlisted_"` in
[`profile/page.tsx:48`](../../web/app/(student)/profile/page.tsx), mapping a stored
`unlisted_school_name` back to the free-text input. The gap is on the service side.

---

## 3. FB-18 — participant picking is not visually clear

Same component as FB-12. Fix the affordance while the file is open.

Additionally, the Registrations tab is `<UnderMaintenance>` for `admin_exam` and real only for
`admin_school` / `super_admin`
([`packages/[id]/page.tsx:597-603`](../../web/app/(admin)/admin/exam/packages/[id]/page.tsx)):

```tsx
{tab === "registrations" && (
  role === "admin_school" || role === "super_admin"
    ? <ExamRegistrationsTab examId={id} examName={data.title} />
    : <UnderMaintenance … />
)}
```

**Decide this deliberately.** Giving `admin_exam` the tab is a role-reach change, and F-2b (the RBAC
pass) says that review should be scoped before roles are widened. Either make the case here in
writing, or leave the gate and say so.

---

## 4. NF-3 — bulk upload schools, with a downloadable template

The pattern already exists in
[`BulkImportModal.tsx:26-42`](../../web/components/admin/BulkImportModal.tsx) — header constant,
example row, client-side download, no network call. Copy it.

**Copy it after E1.** The backend half of that pattern is `student_bulk.go`, whose CSV writer has the
formula-injection hole (B-8). Copying it before the sanitiser lands copies the hole into a second
importer.

---

## Acceptance

- A student with `school_id IS NULL` and an `unlisted_school_name` appears in the participant picker
  and can be registered to an exam.
- Switching a profile from a listed school to an unlisted one saves; switching back saves.
- A student whose new unlisted school is a different level than their old school can still save
  (Cause B is genuinely closed, not masked).
- School bulk upload accepts the downloaded template unmodified and reports failed rows.
- The exported failure report is inert for a school named `=cmd|…`.
- The Registrations-tab role decision is recorded in this doc, whichever way it goes.

## Out of scope

- **F-2b** — the full RBAC pass. This epic makes one role decision and documents it; it does not
  review the whole capability map.
- **F-2a** — admin self-service profile. Students can edit their profile; admins cannot. Different
  surface, unscheduled.
- Merging unlisted school names into real `School` rows. Tempting, but the client has not asked for
  it and it needs a dedupe policy.
