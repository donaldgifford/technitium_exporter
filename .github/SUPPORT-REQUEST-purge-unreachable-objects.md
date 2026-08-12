# GitHub Support request: purge unreachable objects after credential rewrite

Submit at <https://support.github.com/contact> — category **Account or
repository access → Repository content / other**. Everything below the line is
the ticket body; paste it verbatim.

Delete this file once Support confirms the purge.

---

**Subject:** Purge unreachable objects containing a leaked credential after
`git filter-repo` rewrite — donaldgifford/technitium_exporter

Hello,

I force-pushed a rewritten history to `donaldgifford/technitium_exporter`
(public) to remove an API credential that had been committed to a tracked
`Makefile`. The rewrite succeeded on `main`, but the pre-rewrite commits remain
reachable by SHA through `refs/pull/*` refs, so the credential is still served
by the API and web UI.

I am requesting that you garbage-collect the unreachable objects and drop the
stale pull-request refs for this repository.

**Repository:** `donaldgifford/technitium_exporter` (public, 0 forks, 0 stars)

**Current clean HEAD of `main`:** `dfee94eeb83ac52e992e6cb48a6907050a54618d`

**Pre-rewrite commits containing the credential (all now unreachable from any
branch or tag):**

| SHA                                        | Subject                                     |
| ------------------------------------------ | ------------------------------------------- |
| `00d895e5a9a21d4bc34716bc7378b5709b848a8c` | testing the ci                              |
| `91bbe9899b0585b02c5500bf702aa7c0e4ffd34f` | chore: update repo                          |
| `36c092a4e42af560e9a994e5302f1a02c398f52c` | fix(makefile): remove hardcoded credentials |
| `788ff0e95ee66c949d6379ab287b074ac0f6e088` | Merge pull request #28                      |

**Reproduction — the credential is still retrievable:**

```bash
gh api "repos/donaldgifford/technitium_exporter/contents/Makefile?ref=00d895e"
# line 37 of the decoded content still contains the live token value
```

**What is keeping the objects alive:** all 18 `refs/pull/N/head` refs still
point into the pre-rewrite history. I confirmed these cannot be removed
client-side — `git push origin --delete refs/pull/22/head` is rejected with
`deny updating a hidden ref`, and there is no REST endpoint to delete a pull
request.

**What I have already done:**

- Rewrote history with `git filter-repo --replace-text`, replacing the
  credential and an internal hostname. Verified zero blobs in the rewritten
  history contain the token.
- Force-pushed `main` and all three tags (`v0.1.0`, `v0.2.0`, `v0.3.0`).
- Closed and deleted the branches for the 6 open pull requests that carried the
  old history (#22-#27).
- Rotated the exposed credential on the origin server, so the leaked value is no
  longer valid. This request is to complete the cleanup, not to mitigate an
  active exposure.
  <!-- ^ ROTATE THE TOKEN BEFORE SENDING, or delete this bullet. Do not tell
       Support the credential is dead if it is still live -- it changes how they
       triage the request, and it is the one claim here that is not yet true. -->

Please confirm once the unreachable objects have been purged, so I can verify
the old SHAs no longer resolve.

Thank you, Donald Gifford
