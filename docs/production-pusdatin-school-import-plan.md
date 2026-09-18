# Manual Pusdatin school import

`backend/cmd/import-pusdatin` is a standalone operator command. It never runs through migrations, API startup, deployment hooks, or a scheduler. Deploy PR #171's schema/application changes before importing. No Pusdatin metadata is added to `school`.

## Contract

- Match only by normalized NPSN, including inactive schools. Never match or merge by school name.
- Insert missing NPSNs with code `NPSN-<NPSN>`, active application status, source name/address, `school_types = [UPPER(Bentuk)]`, and resolved city/province. Source `Status` is public/private ownership, not the application's active/deactivated status, and is not imported.
- Preserve existing school ID, name, code, NPSN representation, status, populated address, and student links. Fill empty address, location, and type fields. Retain an existing multi-type array if it contains the source type; otherwise report a conflict. Do not infer additional student levels from `Bentuk` or replace existing types.
- Resolve `Kabupaten` against `city.name`, preserving the distinction between a kabupaten and a kota. Matching normalizes case/whitespace and expands `KAB.`/`KAB` to `KABUPATEN`. Derive province from the selected city's current `province_id`.
- Unknown/ambiguous cities, conflicting existing locations/types, multiple NPSN owners, invalid NPSNs/names/types, generated-code collisions, and conflicting source rows block the entire apply. No partial-success mode.
- Collapse byte-equivalent decoded duplicate CSV rows. If any column differs for one NPSN, hold that identity. `source_rows` refers to CSV record numbers, including the header, rather than physical lines within quoted multiline fields.
- All database writes happen in one transaction. Apply takes a school write lock and region read locks before recomputing the plan. School writes from the app may wait until it completes; use a maintenance window and serialize with other school cleanup/import operations. Lock acquisition times out after 5 seconds, SQL statements after 10 minutes, and the command after 30 minutes.
- Apply requires the exact SHA256 of a reviewed dry-run plan. A changed source or changed affected school/location plan invalidates approval. A retry must use a fresh dry-run; repeating an already imported, unchanged dataset yields only `unchanged` rows.

The admin school bulk uploader remains a separate feature that matches by school code and accepts at most 1,000 rows.

## Source

Workspace source (outside Git): `docs/Data Induk Satuan Pendidikan  - DAFTAR Nasional 360 - ASC - 17 Agustus 2026.csv` in the parent project directory.

The 17 September 2026 audit counted 554,885 rows and 554,882 distinct NPSNs. There is no province column. Duplicate NPSNs are `69931346`, `70005045`, and `70005078`; `69931346` has conflicting address/locality values. Keep the original file unchanged. Resolve disputed rows in a reviewed copy, retaining a separate record of the resolution and supporting evidence.

The importer uses the type catalog from PR #171. It does not infer that SLB, PKBM, or a foundation serves only one student level, nor change student eligibility rules. A school-type label in the source is imported literally as the corresponding uppercase catalog value.

## Prerequisites

- Reviewed PR #171 schema, including `provinsi_id`, `kota_id`, and removal of `category`.
- Existing normalized-NPSN unique index on `UPPER(BTRIM(npsn))`, either unfiltered or filtered by `npsn IS NOT NULL`. The command checks the actual index definition and validity; it does not create indexes or alter schema.
- Database region master populated and reviewed. Resolve missing locations explicitly; never guess a province from a school name.
- `DATABASE_URL` injected securely for the intended database. No database URL fallback. Use a read-only database role for initial dry-runs where available. Apply additionally needs school writes, temporary table creation, and the stated locks.
- A writable parent directory for reports. Each invocation requires a **new** output directory; existing reports are never overwritten. Reports include school before-images and proposed values, so retain them privately. The command creates directories with mode `0700` and files with mode `0600`.

## Build and dry-run

From `app/backend`, build once:

```sh
go build -o /tmp/import-pusdatin ./cmd/import-pusdatin
```

With `DATABASE_URL` already supplied through the operator's normal secure environment:

```sh
/tmp/import-pusdatin \
  --csv '/absolute/path/to/reviewed-pusdatin.csv' \
  --out '/absolute/path/to/reports/dry-run-01'
```

No `--apply` means a read-only database transaction. Reports:

- `plan.jsonl`: source checksum followed by one action per distinct normalized NPSN, source record numbers, source city, before-image, proposed school, and conflict reason where applicable. Insert IDs/timestamps are assigned by PostgreSQL at apply time, so they are empty in the plan.
- `result.json`: source checksum, plan checksum, source/distinct counts, insert/update/unchanged/conflict counts, and operation status. The plan SHA256 is the SHA256 of `plan.jsonl` itself.

Any conflict exits nonzero after writing the report. Review all conflicts, fix the reviewed source or provide reviewed city aliases, and dry-run again. Zero conflicts means the plan is executable, not that every source fact has been independently verified.

For city names that require explicit resolution, prepare a CSV containing the **source** name and reviewed application city ID:

```csv
kabupaten,kota_id
SOURCE CITY NAME,REVIEWED_CITY_ID
```

Pass `--city-map '/absolute/path/to/reviewed-city-map.csv'` to both dry-run and apply. IDs must already exist in the database. Do not use aliases to conceal a real location conflict. The command never mutates the region master.

## Apply and verify

First apply and repeat the process against staging. Before production, verify the deployed schema/picker and rerun dry-run against fresh production state. Review the exact plan and obtain the operator's production-execution authorization.

```sh
/tmp/import-pusdatin \
  --csv '/absolute/path/to/reviewed-pusdatin.csv' \
  --out '/absolute/path/to/reports/apply-01' \
  --expect-plan '<plan_sha256 from reviewed dry-run>' \
  --apply
```

Include the same `--city-map` when used. The command recomputes the plan under locks, rejects stale hashes, bulk-loads changed rows through a temporary table, checks affected counts and persisted values, then commits. It does not delete schools, reassign users/students, or activate existing deactivated schools.

`applied` confirms commit. `prepared` does not confirm a commit. `commit_unknown` means commit confirmation is missing; inspect the database through a fresh dry-run before deciding what to retry. An error after commit explicitly states that the database committed but final report writing failed. Preserve all artifacts; never assume an interrupted apply wrote nothing.

Run another read-only dry-run with a new output directory. Expect zero inserts, updates, and conflicts, with every distinct source NPSN counted as unchanged. Independently verify existing school IDs/statuses/student links against before-images and sample picker searches by NPSN and location/type. Only call the national import complete when the entire reviewed source is accounted for and zero exceptions remain. This command does not automate a destructive rollback after a successful commit.

## Verification

```sh
go test -race -count=1 ./cmd/import-pusdatin
go vet ./cmd/import-pusdatin
```

Tests use an isolated PostgreSQL container and the real application migrations. They cover identity preservation, source and mapping conflicts, stale plans, rollback after a failed insert, preserved student links and multi-type schools, and a 10,000-school import followed by a dry-run with zero changes.

To additionally dry-run the real national CSV against that disposable test database:

```sh
PUSDATIN_TEST_CSV='/absolute/path/to/source.csv' \
PUSDATIN_TEST_REPORT='/absolute/path/to/new-report-directory' \
go test -race -count=1 -v ./cmd/import-pusdatin
```

This is parser/mapping validation on a test database, not a production dry-run or proof that production conflicts are resolved.
