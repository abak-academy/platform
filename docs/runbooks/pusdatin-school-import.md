# Pusdatin school import runbook

This runbook imports the verified Pusdatin school dataset directly into the existing `school` table. It does not create a second registry. Matching is only by normalized NPSN; never merge or update schools based only on similar names.

Production import is a manual operation. Do not run `apply` until a fresh dry-run report has been reviewed, the duplicate-resolution file has been approved, and the operator has explicit production authorization.

## Inputs

Use the verified source file from `akademi-bimbel/docs`:

- Source: `docs/Data Induk Satuan Pendidikan  - DAFTAR Nasional 360 - ASC - 17 Agustus 2026.csv`
- Source SHA-256: `ebaf0e05b047d87270f78b4a44f7c35ddb586bc888672280df303f15dcc22887`
- Geographic aliases: `docs/pusdatin-geographic-aliases.json`

The source currently has one conflicting duplicate NPSN, `69931346`. A dry run without a resolution must block apply. Identical duplicate rows are collapsed automatically.

A duplicate-resolution file is an array of exact reviewed choices:

```json
[
  {
    "source_sha256": "ebaf0e05b047d87270f78b4a44f7c35ddb586bc888672280df303f15dcc22887",
    "npsn": "69931346",
    "record_number": 123,
    "row_hash": "row hash copied from the dry-run duplicate group"
  }
]
```

The row hash prevents stale resolution reuse after the source file changes.

## Fresh dry run

Run this against the target database and save the full JSON report:

```bash
go run ./cmd/import-pusdatin \
  -mode dry-run \
  -source "docs/Data Induk Satuan Pendidikan  - DAFTAR Nasional 360 - ASC - 17 Agustus 2026.csv" \
  -expected-sha256 ebaf0e05b047d87270f78b4a44f7c35ddb586bc888672280df303f15dcc22887 \
  -aliases docs/pusdatin-geographic-aliases.json \
  -resolutions /path/to/pusdatin-duplicate-resolutions.json \
  > /secure/path/pusdatin-dry-run.json
```

Review these fields before apply:

- `source_sha256` equals the expected SHA above.
- `transformed_checksum` and `reviewed_checksum` are recorded in the change ticket.
- `counts.inserted`, `counts.updated`, `counts.unchanged`, `counts.target_total`, and source row counts are plausible for the target environment.
- `duplicate_groups` are present for audit; conflict groups must have approved resolutions.
- `blockers` is empty. If not empty, stop.
- Existing matched schools refresh Pusdatin name, address, and school types, but keep their existing `id`, `code`, `status`, and relationships.

## Relationship-integrity preflight

Before apply, capture relationship checksums from the same database session target:

```sql
SELECT
  COUNT(*) AS users_total,
  COUNT(school_id) AS users_with_school,
  md5(string_agg(id::text || ':' || COALESCE(school_id::text, ''), ',' ORDER BY id)) AS users_school_checksum
FROM users;

SELECT COUNT(*) AS schools_total, COUNT(DISTINCT UPPER(BTRIM(npsn))) FILTER (WHERE npsn IS NOT NULL) AS distinct_normalized_npsn
FROM school;

SELECT UPPER(BTRIM(npsn)) AS normalized_npsn, COUNT(*)
FROM school
WHERE npsn IS NOT NULL
GROUP BY UPPER(BTRIM(npsn))
HAVING COUNT(*) > 1;
```

The duplicate query must return no rows before apply. The importer also checks the expected external unique index on normalized NPSN before writing.

## Source-scale query behavior check

School picker traffic must remain bounded before and after import:

- `GET /api/v1/schools` with no filters returns an empty `{data,next_cursor}` envelope.
- Name search requires province and at least three name characters; limit is capped at 50.
- NPSN search uses exact normalized NPSN and returns at most one school.
- Single-school display uses `GET /api/v1/schools/:id`.

After loading a staging-size dataset in a non-production environment, run `EXPLAIN (ANALYZE, BUFFERS)` for representative searches and keep the output with the dry-run evidence:

```sql
EXPLAIN (ANALYZE, BUFFERS)
SELECT s.id, s.name, s.code, s.npsn
FROM school s
WHERE s.status = 'active'
  AND UPPER(BTRIM(s.npsn)) = '12345678'
ORDER BY s.name ASC, s.id ASC
LIMIT 2;

EXPLAIN (ANALYZE, BUFFERS)
SELECT s.id, s.name, s.code, s.npsn
FROM school s
JOIN city c ON c.id = s.city_id
JOIN province p ON p.id = c.province_id
WHERE s.status = 'active'
  AND p.id = '<province_id>'
  AND LOWER(s.name) LIKE '%sma%' ESCAPE '\\'
ORDER BY s.name ASC, s.id ASC
LIMIT 51;
```

The evidence must show bounded result counts and no browser path that loads the full registry.

## Apply

Use the `reviewed_checksum` from the reviewed dry-run. Save the rollback manifest with restrictive file permissions:

```bash
go run ./cmd/import-pusdatin \
  -mode apply \
  -source "docs/Data Induk Satuan Pendidikan  - DAFTAR Nasional 360 - ASC - 17 Agustus 2026.csv" \
  -expected-sha256 ebaf0e05b047d87270f78b4a44f7c35ddb586bc888672280df303f15dcc22887 \
  -aliases docs/pusdatin-geographic-aliases.json \
  -resolutions /path/to/pusdatin-duplicate-resolutions.json \
  -reviewed-checksum "<reviewed_checksum_from_dry_run>" \
  -manifest-out /secure/path/pusdatin-apply-manifest.json \
  > /secure/path/pusdatin-apply-report.json
```

The apply is idempotent. Re-running with the same reviewed checksum after a successful import should report no material changes.

## Post-apply verification

Run manifest verification:

```bash
go run ./cmd/import-pusdatin \
  -mode verify \
  -manifest /secure/path/pusdatin-apply-manifest.json
```

Re-run the relationship checksum SQL. `users_total`, `users_with_school`, and `users_school_checksum` must match the preflight values. Existing school IDs in the manifest must match before and after images. Newly inserted schools must have no `users.school_id` references unless a later separate linking operation was explicitly approved.

## Rollback plan

Rollback uses the apply manifest. It is guarded: an inserted school is deleted only if no user references it, and an updated school is reverted only if its current row still matches the apply after-image. This prevents rolling back over later legitimate changes.

```bash
go run ./cmd/import-pusdatin \
  -mode rollback \
  -manifest /secure/path/pusdatin-apply-manifest.json
```

After rollback, re-run the relationship checksum SQL and a dry run. If rollback blocks, stop and inspect the reported error before changing data manually.
