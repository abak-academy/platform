# Runbook: moving the repository to the client's GitHub organization

Moves `panca1093/bimbel-abak-academy` to a GitHub **organization owned by the client**, private, with
Panca as a second Owner and room to invite collaborators.

Decision context: GitHub was kept over GitLab because the whole delivery chain is already
GitHub-native (Actions, GHCR, the planned WIF federation), and moving would mean rewriting CI and
re-hosting the registry for no product gain. The org is created under the **client's** identity —
matching the Cloudflare zone and the GCP project — so handover is revoking a membership rather than
performing another transfer. Every move costs an image-path change and a VM re-login, so this is done
once, straight to the final destination.

**Do this BEFORE building the production Workload Identity Federation.** WIF binds to the repository
path in two places (the provider's attribute condition and the `principalSet` member), and a WIF pool
name cannot be reused for 30 days after deletion — so a pool built against the old path is wasted and
its name is blocked.

## Pre-flight

- [ ] Confirm the client will actually hold the org. If they will not create and keep an account, stop
      and reconsider — an org under Panca's account means doing this migration twice.
- [ ] Verify current GitHub plan limits before committing, because both change and both bite only
      after the repo is private: **Actions minutes** on a private repo are metered (a public repo is
      unlimited), and **branch protection / required reviews on private repos** may require a paid
      tier. The pipeline runs ~13 minutes per push to `main`; multiply by the number of people you are
      about to invite.
- [ ] Take note of the currently deployed staging tag so you can roll back:
      `grep IMAGE_TAG .env` from the deploy directory on the staging VM (`abak-app`). **The box does
      not mirror the repo layout** — the repo keeps these under `deploy/`, the VM has them
      hand-placed under `abak-app/`. Paths below say which one they mean.
- [x] Org name: **`abak-academy`**. The client's GitHub account (abak email) already exists.
- [x] Repository renamed to **`platform`** as part of the transfer, so the final path is
      `abak-academy/platform` and images are `ghcr.io/abak-academy/platform/{api,worker,web}`. Done now
      because the image paths change anyway; renaming later would cost this entire migration a second
      time.
- [ ] Merge PR #44 before transferring, so a 59-commit PR is not orphaned in the old namespace.

## What travels, and what does not

| Travels with the transfer | Does not |
|---|---|
| Code, branches, tags, releases | Repo secrets and variables (deleted on transfer) |
| Issues, pull requests, review history | Published GHCR image versions (see below) |
| `.github/workflows/pipeline.yml` (it is just a file) | Anything hard-coded to the old path |
| Git redirects from the old URL | Actions minute allowance (billing moves to the org) |

**Secrets and variables: currently none exist.** `gh secret list` and `gh variable list` both return
empty, so there is nothing to re-create today. `secrets.MIDTRANS_CLIENT_KEY` is referenced by the
`images-web` job but has never been set — harmless, because the storefront fetches the Midtrans client key
from the API at runtime and only falls back to the build arg. The go-live slots
(`vars.PROD_GOOGLE_CLIENT_ID`, `secrets.MIDTRANS_CLIENT_KEY_PROD`, and the two `vars.GCP_*`) are still
empty and get created in the new org, not migrated.

**GHCR images do not reliably follow a repository transfer.** Do not plan around them moving. Treat
every image already published under `ghcr.io/panca1093/...` as abandoned and rebuild after the move —
the tag is a commit SHA, so a rebuild of the same commit is reproducible.

## 1 — Create the org and transfer

- [ ] Client creates a GitHub organization (Free plan is sufficient to start).
- [ ] Client adds Panca as an **Owner**, not a Member. Without Owner you can be locked out of your own
      delivery pipeline if the client goes unresponsive.
- [ ] Transfer the repo: repo **Settings → General → Danger Zone → Transfer ownership**.
- [ ] Set visibility to **Private** (same Danger Zone) if the transfer did not already.
- [ ] Invite the collaborators. Give them a team rather than individual grants — it is the same effort
      now and much less to unpick at handover.

## 2 — Fix what the move breaks

The workflow needs **no edit**: it derives the registry from `ghcr.io/${{ github.repository }}`
(`pipeline.yml:53`), so it follows the repo automatically. Only two things are hard-coded.

- [ ] Update the three image lines in the staging compose file (lines 52, 65, 78) from
      `ghcr.io/panca1093/bimbel-abak-academy/...` to the new org path. Two separate copies, and both
      need it — **there is no git clone on the box**, so neither edit propagates to the other.

```bash
# on the staging VM — hand-placed, filename is whatever was copied there
sed -i 's|ghcr.io/panca1093/bimbel-abak-academy|ghcr.io/abak-academy/platform|g' abak-app/app-staging.yaml

# in the repo — commit this, or the next hand-copy re-introduces the old path
sed -i 's|ghcr.io/panca1093/bimbel-abak-academy|ghcr.io/abak-academy/platform|g' deploy/compose/staging.yml
```
- [ ] Optional, cosmetic: `deploy/pipeline/build-image.sh:5` mentions the old path in an error-message
      example. No behaviour depends on it.

**Authenticate the staging VM against the now-private registry.** A public repo allows anonymous
pulls; a private one does not, and the failure (`denied`) looks like a broken deploy rather than an
auth problem. Create a PAT with `read:packages` scope and log in on the box:

```bash
echo "<PAT>" | docker login ghcr.io -u <github-username> --password-stdin
```

- [ ] Confirm the credential landed in `~/.docker/config.json` on the VM, and remember it is now one
      more hand-placed secret that exists nowhere else — same category as `secrets.yaml`, `.env`, and
      the Cloudflare Origin Certificate.

## 3 — Rebuild and redeploy staging

- [ ] Push any commit to `main` in the new org (or re-run the pipeline) so the image jobs publish under the
      new path. Confirm the run is green and note the commit SHA.
- [ ] On the staging VM, set `IMAGE_TAG` to that SHA, then:

```bash
docker compose -f app-staging.yaml pull && docker compose -f app-staging.yaml up -d
```

- [ ] Verify: containers healthy, `select count(*) from users;` unchanged, and
      `https://stg.abakacademy.id` loads. If `pull` fails with `denied`, the VM login in step 2 did not
      take.

## 4 — Only now, build production WIF

Use the new `<org>/<repo>` everywhere. The GCP-side pieces (enabling `artifactregistry`,
`iamcredentials` and `sts`; creating the AR repo `app` in `asia-southeast2`; creating the `gh-deploy`
service account and scoping `roles/artifactregistry.writer` to that repo) name no repository and can
be built before or after the move. The repo-bound pieces are:

- [ ] WIF pool + OIDC provider, with an attribute condition pinning the repository. **This is
      mandatory, not hardening** — without it any GitHub Actions workflow anywhere can exchange an
      OIDC token for your service account:

```bash
gcloud iam workload-identity-pools providers create-oidc github-oidc --location=global --workload-identity-pool=github --issuer-uri="https://token.actions.githubusercontent.com" --attribute-mapping="google.subject=assertion.sub,attribute.repository=assertion.repository,attribute.ref=assertion.ref" --attribute-condition="assertion.repository == 'abak-academy/platform'" --project=abak-academy-platform
```

- [ ] Bind `roles/iam.workloadIdentityUser` for
      `principalSet://iam.googleapis.com/projects/281709074160/locations/global/workloadIdentityPools/github/attribute.repository/abak-academy/platform`.
- [ ] Set `vars.GCP_WORKLOAD_IDENTITY_PROVIDER` (full resource name using the project **number**
      281709074160, not the project ID — a common failure) and `vars.GCP_DEPLOY_SERVICE_ACCOUNT`.
- [ ] Fill the remaining go-live slots in the new org: `vars.PROD_GOOGLE_CLIENT_ID` and
      `secrets.MIDTRANS_CLIENT_KEY_PROD`.

Note that `images-web-prod` will go green as soon as WIF works, even with those slots empty — the build
defaults them to empty strings rather than failing. An empty `NEXT_PUBLIC_GOOGLE_CLIENT_ID` has **no
runtime fallback**, so Google sign-in is dead in that image. Green here does not mean deployable.

## Rollback

The transfer itself is reversible: transfer the repo back, and GitHub's redirects mean `git` remotes
keep working either way. What is not automatically restored is the visibility change and any org-level
settings. Staging can be rolled back independently at any point by pointing `app-staging.yaml` back at
`ghcr.io/panca1093/...` with the previously deployed `IMAGE_TAG`, since those images still exist —
which is the reason for recording that tag in pre-flight.
