# Cycle 3 backlog — console information design

Structural findings noticed during the Cycle 2 visual sweep. None were fixed
there: Cycle 2 is visual-only, which is what keeps rendering sufficient as its
verification.

| Page | Finding |
|---|---|
| Users | Panel subtitle reads "Inviting a user issues a one-time agu- token; removing one revokes it." — `agu-` is not a word; the copy looks like it lost a fragment during an edit (`pages/Users.tsx:106`). Needs a copy fix, not a colour one. |
| Overview, Keys, Audit, Orgs, Users, Secrets, Provisioning | With no admin key configured, these seven pages render nothing but the "No admin key configured" notice — no scaffold, form, or preview of what the page shows once configured. Documents, Improve, Multiverse and Operators, by contrast, keep their full panel structure visible alongside the same notice. Worth deciding, page by page, whether the quieter pages should do the same or whether the blank page is intentional. |
| Documents, Improve | Primary action buttons ("Ingest document", "Run evals", "Propose improvement") stay enabled with empty required fields and only disable while the request is in flight. Playground ("Run"), Multiverse ("Queue") and Operators ("Create operator") disable until required input is present, in addition to disabling in flight. The two groups validate differently; worth picking one convention. |
