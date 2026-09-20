# Manual Pusdatin school import

## Objective

Import the complete Pusdatin dataset into the existing `school` registry, including school type, province, and city. Preserve existing schools and student relationships. Do not add a second registry, Pusdatin-specific school fields, automatic synchronization, or automatic cleanup.

The import is a separate operation run explicitly by an operator or agent. It must not run in a database migration, API startup, deployment hook, or scheduled job. Deploying PR #171 only prepares the schema and application behavior. Migration 0064 is retained for databases that already applied it; migration 0065 removes its category column and replaces the location index. Neither migration imports Pusdatin records or rewrites `school_types` or student links. Rolling back 0065 restores an empty category column, not its former values.

## Source and unresolved decisions

- Source: workspace `docs/Data Induk Satuan Pendidikan  - DAFTAR Nasional 360 - ASC - 17 Agustus 2026.csv` (outside the Git repository).
- Verified on 17 September 2026: 554,885 rows and 554,882 distinct NPSNs.
- The source contains `Bentuk` and `Kabupaten`, but no province column. Resolve the city against the application's region master, then derive its province. Report unknown or ambiguous region matches instead of guessing.
- Duplicate NPSNs: `69931346`, `70005045`, and `70005078`. Exact duplicates may be collapsed; conflicting rows require an explicit resolution. `69931346` has conflicting address/locality values.
- `school_types` remains an array and is the only school-type field. There is no separate `category`. The 17 September production audit found five multi-type schools linked to 34 students; retain their arrays and existing student eligibility. New Pusdatin schools use the reviewed source-to-type mapping.
- `Bentuk` must be mapped deliberately to the school-type contract because `school_types` currently also validates student `jenjang`. Do not infer that a school such as an SLB serves only one student level.

## Matching and preservation

1. Match by normalized NPSN, never by school-name similarity.
2. Insert NPSNs absent from the registry with complete resolved location and type fields.
3. For exactly one existing NPSN owner, report proposed field changes in the dry-run. Preserve its ID, code, status, and all student relationships. Conflicting existing school types need an explicit resolution before replacement.
4. Leave schools without a matching NPSN unchanged, including similarly named records.
5. Report ambiguous NPSN ownership, conflicting source rows, code collisions, and missing location/type mappings. Unresolved exceptions must remain visible; do not call a partial import complete.

The admin school bulk uploader is not the national Pusdatin importer: it updates by normalized school code and accepts at most 1,000 rows. A separate manual import command remains to be implemented as a separate operational deliverable; this document is not an executable importer.

## Execution order

1. Prepare and validate the dataset and proposed mappings; produce a read-only dry-run with insert/update/unchanged/conflict counts and an exact proposed change report.
2. Verify the final schema and picker behavior in staging. Run the manual importer there and verify that repeating it creates no duplicate schools.
3. Deploy the reviewed application/schema changes. Verify the location fields, indexes, and the existing normalized-NPSN uniqueness prerequisite before the production import.
4. Re-run the production dry-run against fresh state. Serialize the import with other school cleanup/import operations.
5. Execute the separately reviewed production import manually, then verify independently. Keep the source checksum, dry-run, before-images of changed schools, and result report as operational artifacts outside the school model.

## Completion checks

- Every distinct source NPSN is accounted for exactly once as imported, unchanged, or explicitly unresolved.
- Every imported school has the expected type, city, and province; each city belongs to its assigned province.
- Existing school IDs, codes, statuses, and student school references are unchanged.
- Repeating the same resolved dataset inserts zero additional identities and yields zero expected-field differences.
- Picker searches can find sampled imported schools by NPSN and by location/type.
- Zero unresolved records are required before claiming the complete dataset was imported.
