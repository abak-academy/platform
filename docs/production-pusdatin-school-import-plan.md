# Production Pusdatin school import plan — 2026-09-06

## Current evidence

- Source: `docs/Data Induk Satuan Pendidikan  - DAFTAR Nasional 360 - ASC - 17 Agustus 2026.csv`.
- Source SHA-256: `ebaf0e05b047d87270f78b4a44f7c35ddb586bc888672280df303f15dcc22887`.
- Source rows: 554,885.
- Unique non-empty NPSNs: 554,882.
- Duplicate NPSNs: three values, each appearing twice. Two pairs are exact duplicate rows; NPSN `69931346` has two address variants.
- Deduplicated artifact: 554,882 rows with unique NPSN. SHA-256: `c15efeb364dba67cedc8e3de141e3dded1b31a100bd545a67e512c49531ec40b`.
- The source has no blank NPSN, blank name, or punctuation-only name. It has 424 blank or dash-only addresses.
- The source includes formal and non-formal institutions: PAUD, TK, KB, RA, SD/MI, SMP/MTS, SMA/MA/SMK, SLB, PKBM, courses, and pesantren.

## Production overlap

- Current `school` rows: 1,270.
- Current production NPSNs matching Pusdatin: 104.
- New Pusdatin identities when matching only by NPSN: 554,778.
- Existing production records without NPSN:
  - 379 have one exact normalized-name match in Pusdatin.
  - 43 have one broader canonical-name match.
  - 39 have multiple same-name candidates and cannot be merged safely by name alone.
  - 657 have no Pusdatin name match.
- Production has no unique constraint on `school.npsn` or `school.code`.
- Production still contains repeated placeholder NPSNs `11111111`, `22222222`, and `33333333`; these values do not exist in the Pusdatin source.

## Why direct insertion into `school` is unsafe

- The active-school option endpoint returns the complete active registry without pagination. Inserting 554,778 active records would make every picker request return roughly 555,000 rows.
- `school` cannot retain all Pusdatin identity fields: Kabupaten, Kecamatan, Kelurahan, Bentuk, Jenis, ownership status, Jalur, and Pembina would be discarded.
- Name uniqueness is invalid: 29,128 canonical names are shared by multiple Pusdatin schools. NPSN must remain the authoritative key.
- Existing production schools without NPSN require a separate reconciliation step. Automatically merging them by name would conflate legitimate schools with the same name.

## Recommended import shape

1. Create a dedicated Pusdatin directory table containing all 12 source columns, source date, and timestamps.
2. Enforce one row per non-empty NPSN with a database primary/unique constraint.
3. Load the 554,882-row deduplicated dataset through a staging table and `COPY`, then upsert into the directory by NPSN.
4. Add indexed server-side school search over NPSN, name, and location; do not return the full directory in one response.
5. Keep the current `school` table as the set of schools already linked to application users. Reconcile or materialize an application school only after an official NPSN is selected.
6. Preserve every current user and `school_id`; ambiguous existing records remain unchanged until their official identity is known.

## Import verification

- Staging row count must be 554,882.
- Staging distinct non-empty NPSN count must be 554,882.
- Reject any blank NPSN/name or duplicate NPSN before upsert.
- After upsert, every staged NPSN must exist exactly once with the expected metadata.
- Re-running the same import must insert zero new identities and produce zero metadata mismatch.
- Global user/student ID and `school_id` checksums must remain unchanged because the directory import does not rewrite user linkage.
