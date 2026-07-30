# Runbook: PostgreSQL 16 → 17 data migration

Applies to any environment whose `pgdata` volume was created while the compose file pinned
`postgres:16-alpine` — today that is **local dev** and **staging**. Production is unaffected:
Postgres there is native 17.10 on `vm-db` and its data directory was created by 17.

## Why this is needed

A PostgreSQL data directory is tied to the major version that created it. Starting a 17 server on a
16 directory does not upgrade it — the server exits immediately with:

```
FATAL:  database files are incompatible with server
DETAIL: The data directory was initialized by PostgreSQL version 16, which is not compatible with this version 17.x
```

Compose will keep restarting the container, and because `api` has
`depends_on: postgres: condition: service_healthy`, the API never starts either. The visible symptom
is the whole environment down, with the real cause buried in the postgres container's logs.

## Before you start

- Take the dump with a **PostgreSQL 16** client. A 17 `pg_dump` against a 16 server works, but the
  reverse — restoring a dump produced by a newer client into an older server — does not, and mixing
  versions here is how people lose the dump they thought they had.
- Do this in a window where the environment can be down. There is no zero-downtime path with this
  approach.

## Steps

Run from the directory containing the compose file. Substitute the right file and credentials:
local dev uses `deploy/compose/local.yml` with user/db `akademi`; staging uses
`deploy/compose/staging.yml` with the `POSTGRES_*` values from its hand-placed `.env`.

**1 — Dump, while the 16 container is still running.**

```bash
docker compose -f deploy/compose/local.yml exec -T postgres \
  pg_dump -U akademi -Fc akademi > pg16-backup.dump
```

Check the file is non-empty and plausible before going further:

```bash
ls -lh pg16-backup.dump && docker run --rm -i postgres:16-alpine pg_restore --list < pg16-backup.dump | head
```

If `pg_restore --list` cannot read it, stop — the dump is unusable and destroying the volume now
would lose the data.

**2 — Remove ONLY the PostgreSQL volume.**

Do **not** use `down -v` — the local dev compose also has a `miniodata` volume holding uploaded
objects, and `-v` would delete those too even though this runbook only backed up Postgres. Stop the
stack, then remove just the PG volume:

```bash
docker compose -f deploy/compose/local.yml down
docker volume rm akademi-bimbel_pgdata
```

Removing the PG16 volume is the point of this step: without it the PG16 data survives and the new
container fails exactly as before. `docker volume rm` refuses while a container still uses the
volume, so the `down` must complete first. (The staging compose has no `miniodata` volume — object
storage there is GCS — so on the staging box `down -v` would be equivalent, but prefer the explicit
form everywhere.)

**3 — Start a fresh PG17 and let it initialise.**

```bash
docker compose -f deploy/compose/local.yml up -d postgres
```

Wait for it to report healthy before restoring.

**4 — Restore.**

```bash
docker compose -f deploy/compose/local.yml exec -T postgres \
  pg_restore -U akademi -d akademi --no-owner < pg16-backup.dump
```

**5 — Bring the rest up and verify.**

```bash
docker compose -f deploy/compose/local.yml up -d
```

```bash
docker compose -f deploy/compose/local.yml exec -T postgres \
  psql -U akademi -d akademi -c "select version();" -c "select count(*) from users;"
```

The version must report 17.x, and the row counts must match what you had before. Migrations are
embedded in the api image and run at boot, so `schema_migrations` should need no manual attention —
but confirm the api container actually reached a healthy state rather than assuming it.

Keep `pg16-backup.dump` until you have used the environment for a day or two. It is the only copy.
