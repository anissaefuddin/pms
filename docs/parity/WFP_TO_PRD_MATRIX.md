# Matriks WFP → ID PRD (S7.2.2)

**Tanggal:** 13 Nov 2026. **Status:** DRAFT-SIAP-REVIEW. Sumber: register [baseline §4](../PMS_WRIKE_PARITY_DEVELOPMENT_BASELINE.md), hasil [katalog sumber](WRIKE_SOURCE_CATALOG_2026-11-13.md) dan [PRD §5](../PRD.md) (ID v0.3 di §5.1–5.14, ID parity baru di §5.15).

- **ID PRD:** diambil dari kolom "PMS baseline" register bila ada; untuk baris tanpa ID, dipetakan manual ke ID yang sudah ada atau ke ID baru §5.15. `N/A_APPROVED (…)` = di luar rilis 1.0 menurut PRD v0.4 (M13) dan masih menunggu persetujuan pemilik produk (DEC-PAR-01/05/06).
- **Cakupan 1.0:** `Ada` bila ID PRD sudah DONE di M0–M6, `M8..M12` bila ID baru §5.15 menjadi story di milestone itu (lihat [TRACEABILITY](../TRACEABILITY_M7_M12.md)), `M13` bila di luar 1.0.
- [Rekonsiliasi alur layar](WFP_SCREEN_FLOW_RECONCILIATION.md) mencatat bukti per WFP. Retest 14 Nov 2026 mencocokkan 106 ID unik (WFP-001–106) di register, katalog, dan matriks. WFP-102 mengacu pada PRD §6 NFR Aksesibilitas dan gate STAB-07 di TRACEABILITY. Entri M13 mengacu pada keputusan scope. Label Ada (M0–M6) menunjukkan pemetaan milestone dan perlu diverifikasi terhadap bukti masing-masing requirement.

| WFP | Capability | ID PRD | Cakupan | Status katalog |
|---|---|---|---|---|
| WFP-001 | Spaces / team work areas | SPC-01, SPC-02 | Ada (M0–M6) | CONFIRMED |
| WFP-002 | Projects / folders / nested work | SPC-04, SPC-05, SPC-06 | Ada (M0–M6) | CONFIRMED |
| WFP-003 | Task & subtask, assignees | TASK-01, TASK-02, TASK-06 | Ada (M0–M6) | CONFIRMED |
| WFP-004 | Multiple locations / cross-tagging | SPC-10 | Ada (M0–M6) | NOT_IN_TABLE |
| WFP-005 | Work schedules and calendar settings | SET-07 | Ada (M0–M6) | RESOLVED |
| WFP-006 | Project progress tracking | TASK-09 | Ada (M0–M6) | RESOLVED |
| WFP-007 | Template / Blueprint projects | SPC-13 | Ada (M0–M6) | CONFIRMED |
| WFP-008 | Space templates | SPC-18 | Ada (M0–M6) | RESOLVED |
| WFP-009 | Custom item types | CUST-01 | M8 | CONFIRMED |
| WFP-010 | Custom item layout and default fields | CUST-04 | M8 | NOT_IN_TABLE |
| WFP-011 | Custom fields (basic and advanced) | TASK-10 | Ada (M0–M6) | RESOLVED |
| WFP-012 | Formula custom fields | TASK-11B | M8 | NOT_IN_TABLE |
| WFP-013 | Global and scoped search | NAV-06 | Ada (M0–M6) | NOT_IN_TABLE |
| WFP-014 | Starred and shared-with-me | NAV-01, NAV-02 | Ada (M0–M6) | NOT_IN_TABLE |
| WFP-015 | Guest / external collaborators | SPC-11, USR-01 | Ada (M0–M6) | RESOLVED |
| WFP-016 | Table view inline edit / hierarchy | VIEW-01 | Ada (M0–M6) | CONFIRMED |
| WFP-017 | Kanban Board / drag and drop | VIEW-02 | Ada (M0–M6) | CONFIRMED |
| WFP-018 | Gantt bar, milestones | VIEW-05A | Ada (M0–M6) | CONFIRMED |
| WFP-019 | Four dependency types | VIEW-05B | Ada (M0–M6) | NOT_IN_TABLE |
| WFP-020 | Automatic dependency scheduling | VIEW-05B | Ada (M0–M6) | NOT_IN_TABLE |
| WFP-021 | Gantt baselines / variance | VIEW-05C | M10 | RESOLVED |
| WFP-022 | Project/programme portfolio Gantt | SCH-01 | M10 | RESOLVED |
| WFP-023 | Cross-project dependencies | SCH-01 | M10 | RESOLVED |
| WFP-024 | Calendar view | VIEW-06 | Ada (M0–M6) | RESOLVED |
| WFP-025 | Timelog view | VIEW-07 | Ada (M0–M6) | NOT_IN_TABLE |
| WFP-026 | Files view | VIEW-10 | M11 | NOT_IN_TABLE |
| WFP-027 | Chart view | VIEW-10 | M11 | RESOLVED |
| WFP-028 | Saved, custom, personal views | VIEW-08 | Ada (M0–M6) | RESOLVED |
| WFP-029 | Board swimlanes / custom cards | VIEW-03 | Ada (M0–M6) | NOT_IN_TABLE |
| WFP-030 | Quick search/filter in table | VIEW-01, NAV-06 | Ada (M0–M6) | NOT_IN_TABLE |
| WFP-031 | Custom workflow statuses | WF-01..03A | Ada (M0–M6) | CORRECTED |
| WFP-032 | Allowed transitions / guards | WF-03B | M8 | NOT_IN_TABLE |
| WFP-033 | Status-triggered approval | WF-03B, SPC-12 | M8 | RESOLVED |
| WFP-034 | Any/All approval logic | APR-01 | M9 | RESOLVED |
| WFP-035 | Approval history/reassignment | APR-02, APR-03 | M9 | RESOLVED |
| WFP-036 | Request form builder | FORM-01 | Ada (M0–M6) | CONFIRMED |
| WFP-037 | Public and internal forms | FORM-03 | Ada (M0–M6) | CONFIRMED |
| WFP-038 | Conditional form branching | FORM-07 | M8 | RESOLVED |
| WFP-039 | Form-to-CIT / Blueprint routing | FORM-09, SPC-13 | M8 | RESOLVED |
| WFP-040 | Request tracking / My requests | FORM-08 | Ada (M0–M6) | NOT_IN_TABLE |
| WFP-041 | Request confirmation email customization | FORM-04 | M8 | RESOLVED |
| WFP-042 | Automation rule engine WHEN/IF/THEN | AUT-01, AUT-02, AUT-05 | M9 | RESOLVED |
| WFP-043 | Automation audit, quotas, retries | AUT-03, AUT-04 | M9 | RESOLVED |
| WFP-044 | Solution template bundles | SPC-18, SPC-13 (solution bundle: M13) | M13 (di luar 1.0) | RESOLVED |
| WFP-045 | Effort estimation and allocation | TASK-05B | Ada (M0–M6) | RESOLVED |
| WFP-046 | Workload charts by team/user | RSC-02 | M10 | CONFIRMED |
| WFP-047 | Resources View / project allocation | RSC-02 | M10 | RESOLVED |
| WFP-048 | Capacity/calendar exceptions | SET-07 | Ada (M0–M6) | NOT_IN_TABLE |
| WFP-049 | Resource reallocation / conflicts | RSC-03 | M10 | RESOLVED |
| WFP-050 | Job roles and skills | RSC-04 | M10 | CONFIRMED |
| WFP-051 | Resource bookings | RSC-05 | M10 | CONFIRMED |
| WFP-052 | Planned vs actual utilization | RSC-06 | M10 | CONFIRMED |
| WFP-053 | Time tracking / entries | TASK-13 | Ada (M0–M6) | CONFIRMED |
| WFP-054 | Weekly timesheet | TASK-13 | Ada (M0–M6) | CONFIRMED |
| WFP-055 | Timelog lock / approvals | VIEW-07, TASK-13 | Ada (M0–M6) | NOT_IN_TABLE |
| WFP-056 | Project budget field | TASK-12 | Ada (M0–M6) | CORRECTED |
| WFP-057 | Billable/non-billable | TASK-14 | M10 | NOT_IN_TABLE |
| WFP-058 | Cost rates / bill rates | TASK-14 | M10 | CONFIRMED |
| WFP-059 | Actual/planned cost / forecast | TASK-14 | M10 | CONFIRMED |
| WFP-060 | Comments and @mentions | TASK-07 | Ada (M0–M6) | NOT_IN_TABLE |
| WFP-061 | Inbox / notification / activity stream | NAV-03 | Ada (M0–M6) | RESOLVED |
| WFP-062 | Attachments and download authorization | TASK-08A | Ada (M0–M6) | NOT_IN_TABLE |
| WFP-063 | File preview and Files view | VIEW-10, TASK-08A | M11 | NOT_IN_TABLE |
| WFP-064 | File versioning/history | FIL-01 | M11 | NOT_IN_TABLE |
| WFP-065 | File proofing/annotations | FIL-02 | M11 | CONFIRMED |
| WFP-066 | File versions comparison | FIL-03 | M11 | NOT_IN_TABLE |
| WFP-067 | External file reviewers / approvals | FIL-04 | M11 | RESOLVED |
| WFP-068 | External cloud storage | TASK-08B | M11 | RESOLVED |
| WFP-069 | Collaborative docs/editor | N/A_APPROVED (editor dokumen, M13) | M13 (di luar 1.0) | NOT_IN_TABLE |
| WFP-070 | Dashboard and chart widgets | DASH-01..03A | Ada (M0–M6) | CORRECTED |
| WFP-071 | Report builder and export | RPT-01..03 | Ada (M0–M6) | CONFIRMED |
| WFP-072 | Scheduled report subscription | RPT-04 | M11 | NOT_IN_TABLE |
| WFP-073 | Area / pivot / advanced widgets | DASH-03B | M11 | RESOLVED |
| WFP-074 | Analytics board / drilldown | DASH-08 | M11 | CONFIRMED |
| WFP-075 | Public dashboard share with expiry | DASH-07 | M11 | NOT_IN_TABLE |
| WFP-076 | Advanced BI / external BI API | DASH-08, API-01 | M11 | CONFIRMED |
| WFP-077 | Portfolio reporting | RPT-02 | Ada (M0–M6) | RESOLVED |
| WFP-078 | Excel import/export | SPC-15 | Ada (M0–M6) | NOT_IN_TABLE |
| WFP-079 | Public developer API with auth scopes | API-01, API-03 | M11 | RESOLVED |
| WFP-080 | Webhooks / event delivery | API-02 | M11 | NOT_IN_TABLE |
| WFP-081 | Third-party connectors (Google, M365, Slack etc.) | INT-01, INT-03 | M11 | RESOLVED |
| WFP-082 | Two-way sync (Jira/GitHub etc.) | INT-02, INT-03 | M11 | CONFIRMED |
| WFP-083 | Integration automation orchestration | INT-01, AUT-02 | M11, M9 | CONFIRMED |
| WFP-084 | Datahub / linked records | REG-00 (implementasi M13) | M13 (di luar 1.0) | RESOLVED |
| WFP-085 | External data synchronization | REG-00 (implementasi M13) | M13 (di luar 1.0) | RESOLVED |
| WFP-086 | User, group, license/seat types | USR-01..03 | Ada (M0–M6) | RESOLVED |
| WFP-087 | Sharing role and item permission | SPC-11 | Ada (M0–M6) | RESOLVED |
| WFP-088 | TOTP / backup codes | AUTH-05 | Ada (M0–M6) | RESOLVED |
| WFP-089 | SSO / SAML / provisioning | ENT-01, ENT-02 | M12 | CORRECTED |
| WFP-090 | Custom access roles | ENT-03 | M12 | CORRECTED |
| WFP-091 | Security audit / activity reports | ENT-04 | M12 | CORRECTED |
| WFP-092 | Locked spaces and advanced policies | ENT-05 | M12 | CORRECTED |
| WFP-093 | IP/access restrictions | ENT-05 | M12 | CORRECTED |
| WFP-094 | Enterprise encryption/key management | N/A_APPROVED (manajemen kunci enterprise, M13) | M13 (di luar 1.0) | RESOLVED |
| WFP-095 | Backup, export, decommission | USR-04 | Ada (M0–M6) | NOT_IN_TABLE |
| WFP-096 | AI assistant / summary / generation | N/A_APPROVED (AI, M13) | M13 (di luar 1.0) | RESOLVED |
| WFP-097 | AI agents / automated actions | N/A_APPROVED (AI, M13) | M13 (di luar 1.0) | RESOLVED |
| WFP-098 | AI risk/proactive insights | N/A_APPROVED (AI, M13) | M13 (di luar 1.0) | RESOLVED |
| WFP-099 | MCP / AI tool integration | N/A_APPROVED (AI, M13) | M13 (di luar 1.0) | RESOLVED |
| WFP-100 | Whiteboard / visual collaboration | N/A_APPROVED (Whiteboard, M13) | M13 (di luar 1.0) | RESOLVED |
| WFP-101 | Whiteboard-item task conversion | N/A_APPROVED (Whiteboard, M13) | M13 (di luar 1.0) | RESOLVED |
| WFP-102 | Responsive web and accessibility | UI audit (STAB-07), NFR Aksesibilitas | Ada (M0–M6) | NOT_IN_TABLE |
| WFP-103 | Dark/light theme toggle | SET-06 | M8 | NOT_IN_TABLE |
| WFP-104 | Localization and language settings | NFR i18n (SET-03) | Ada (M0–M6) | RESOLVED |
| WFP-105 | Native mobile/desktop experiences | N/A_APPROVED (aplikasi native, M13; DEC-PAR-06) | M13 (di luar 1.0) | RESOLVED |
| WFP-106 | Offline behaviors / push notifications | N/A_APPROVED (offline/push, M13) | M13 (di luar 1.0) | NOT_IN_TABLE |

# Batch 5 reconciliation note — 15 November 2026

The canonical scope mapping remains unchanged. Acceptance evidence changed as follows without changing PRD scope: AUTH-08 gained keyed-HMAC and concurrent-single-use LOCAL_PASS evidence but remains BLOCKED for mail-sink/deployed acceptance; SPC-14 remains implemented under D-99 with deployed evidence PARTIAL; VIEW-04 gained configurable assignee/date/priority LOCAL_PASS evidence but remains PARTIAL until deployed cross-user verification. See `../qa/BATCH_5_EXECUTION_REPORT.md`.
