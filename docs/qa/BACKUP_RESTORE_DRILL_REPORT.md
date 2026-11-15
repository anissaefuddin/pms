# Backup and restore drill report — Batch 5

Existing isolated evidence was reviewed; no destructive drill was run against dev-lens or production.

- Local isolated backup completed in 1.84 seconds.
- Local restore reached ready state in 13.92 seconds.
- Row counts and file checksums matched between isolated instances.
- This demonstrates restore mechanics, not operational RPO/RTO compliance.

Open acceptance items:

- no active dev-lens scheduler/timer evidence; observed backup gap 27.97 hours exceeds RPO 24 hours;
- no off-host physical restore or production-scale RTO measurement;
- no large-data migration/rollback rehearsal on the exact candidate;
- clone procedure retains active sessions/password hashes and needs the sanitization controls in the security report;
- remote retention, backup-age alerting and recovery of permissions/file versions need exact-environment evidence.

Result: **PARTIAL / RELEASE BLOCKER**.

