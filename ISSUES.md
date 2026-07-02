# Notes Management System - GitHub Issue Processing Workflow

This document serves as the entry point and index for Seraphine's issue-processing workflows. It outlines the general rules and lists the specific workflow files for each stage in the issue lifecycle.

**Note:** Always use **native GitHub sub-issues** when defining parent-child issue relationships to ensure proper tracking within GitHub.

---

## 🚫 Critical General Rules
1. **Scope Adherence**: The agent should only address the labeled issue, and it must stop once the issue is unlabeled.
2. **Termination Rule**: **The agent should not proceed to the next label.** Once you have removed a label from the bug (or a PR is merged), you should stop execution immediately. Do not trigger or begin processing the next stage or label in the same run.
3. **Issue Assignment**: **Whenever a new issue (or sub-issue) is created, it MUST be assigned to `brotherlogic-automation`.**

---

## 🏷️ Workflow Stages & Labels

When an issue is labeled, refer to the corresponding workflow document under `.agents/workflows/` for detailed step-by-step instructions:

1. **Deep Research**
   - **Label**: `seraphine-needs-deep-research`
   - **Workflow Guideline**: [seraphine-needs-deep-research.md](file:///workspaces/recordcollection/.agents/workflows/seraphine-needs-deep-research.md)

2. **Requirements gathering**
   - **Label**: `seraphine-needs-requirements` (or variant `seraphine-need-requirements`)
   - **Workflow Guideline**: [seraphine-needs-requirements.md](file:///workspaces/recordcollection/.agents/workflows/seraphine-needs-requirements.md)

3. **Technical implementation plan formulation**
   - **Label**: `seraphine-needs-implementation-plan`
   - **Workflow Guideline**: [seraphine-needs-implementation-plan.md](file:///workspaces/recordcollection/.agents/workflows/seraphine-needs-implementation-plan.md)

4. **Issue breakdown**
   - **Label**: `seraphine-break-down-issue`
   - **Workflow Guideline**: [seraphine-break-down-issue.md](file:///workspaces/recordcollection/.agents/workflows/seraphine-break-down-issue.md)

5. **Component implementation**
   - **Label**: `seraphine-ready-to-implement`
   - **Workflow Guideline**: [seraphine-ready-to-implement.md](file:///workspaces/recordcollection/.agents/workflows/seraphine-ready-to-implement.md)

6. **Bug triage and resolution**
   - **Label**: `seraphine-bug`
   - **Workflow Guideline**: [seraphine-bug.md](file:///workspaces/recordcollection/.agents/workflows/seraphine-bug.md)

---

## 🛠️ Summary of Expected Label State Transitions

| Phase | Parent Issue Label(s) | Sub-Issue Title & Label(s) |
| :--- | :--- | :--- |
| **Deep Research** | `seraphine-needs-deep-research` | *None (Not yet created)* |
| **Deep Research Complete** | `seraphine-needs-deep-research` (Removed) | Labeled with `seraphine-needs-requirements` to initiate requirements gathering |
| **Requirements Gathering** | `seraphine-needs-requirements` | *None (Not yet created)* |
| **Requirements Approved** | *(Label Removed)* | `[Implementation Plan] <Title>` labeled with `seraphine-needs-implementation-plan` |
| **Implementation Plan Drafting** | *None* | `[Implementation Plan] <Title>` labeled with `seraphine-needs-implementation-plan` |
| **Implementation Plan Approved** | *None* | **Implementation Plan:** Label removed (remains Open).<br>**Breakdown Sub-Issue:** `[Breakdown] <Title>` labeled with `seraphine-break-down-issue` |
| **Issue Breakdown** | *None* | **Breakdown Issue:** `seraphine-break-down-issue` removed (remains Open).<br>**Child Sub-Issues:** `[Sub-Issue] <Action>` labeled with `seraphine-ready-to-implement` |
| **Implementation** | *None* | **Breakdown Issue:** Closed when all child sub-issues are closed (cascading to close Implementation Plan and Parent issues).<br>**Child Sub-Issues:** Labeled with `seraphine-ready-to-implement`. Closed programmatically via PR submission. |
| **Bug Triage (Simple)** | `seraphine-bug` | *None (Direct fix implemented and PR submitted)* |
| **Bug Triage (Complex/Failed)** | `seraphine-bug` (Removed) | New issue labeled with `seraphine-needs-requirements` to initiate requirements gathering |
