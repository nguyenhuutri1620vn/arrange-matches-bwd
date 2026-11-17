# Nhóm luân phiên + tránh 3-1 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Groups alternate together/split (size 2, 4), size-3 groups rotate one pair, groups live in a `session_groups` table, a group can be picked at check-in, and 3-1 lineups are avoided by widening one tier.

**Architecture:** `domain` gets `GroupPhase`, `SessionGroup` and the pure phase transition `NextGroupPhase`. `storage` adds migration 0003 and group queries. `session` advances phases inside `Confirm`'s transaction and feeds `GroupSize`/`GroupPhase` to candidates. `matchmaking` binds only TOGETHER groups of size 2/4 and scores lineups by phase. The simulation reuses `NextGroupPhase`.

**Spec:** `docs/superpowers/specs/2026-11-17-nhom-luan-phien-design.md` (3-1 part: `2026-11-16-tranh-3-1-va-nhom-theo-ti-le-design.md`). Supersedes Tasks 2–8 of `2026-11-16-tranh-3-1-va-nhom-theo-ti-le.md`.

## Global Constraints

- Fairness, gender rules, `BalanceThreshold` stay above group preferences.
- Simulation must PASS: `-matrix -runs 1000`, plus `-groups 2|3|4` and `-group-size 3|4` (3% tolerance for groups from 2 courts, unchanged).
- Phase changes happen in the same transaction as match confirmation.
- `domain.SessionPlayer.Group` becomes the `session_groups.id` (0: none). Display number is `SessionGroup.No`.
- Commit to `develop`, `go test ./...` before each commit, trailer `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. Existing DB with `group_no` values migrates to `session_groups` without losing groups (open session 14/11 has groups).
2. A member leaves a group (set to Không) → empty groups are deleted; size-1 group has no effect.
3. Size-2 SPLIT pair drafted on the same court: they must not be teamed if any balanced split separates them.
4. Group full at check-in: no player inserted.
5. Ended session: no phase updates (sessions only confirm while OPEN).

---

### Task A: Simulation metrics (old plan Task 2)
- [ ] Count per run: matches, 3-1 matches; print `3-1: X%` per config line. Commit `simulation: report 3-1 rate`.

### Task B: Widen the window before 3-1 (old plan Task 3, unchanged)
- [ ] Tests `TestProposeWidensWindowToAvoidThreeOne`, `TestProposeLoneRestedWomanStillPlaysThreeOne`, `TestProposeNoWiderTierKeepsThreeOne`; update third case of `TestProposeAvoidsThreeOneWhenMixedLineupExists`.
- [ ] `bestInWindow(..., skipFreshOdd bool) (Lineup, bool, bool)`; `proposeFrom` widens one tier when best is fresh-odd.
- [ ] Simulation set PASS, 3-1 rate lower. Commit.

### Task C: Domain phases
**Interfaces:**
- `type GroupPhase string`; `GroupTogether = "TOGETHER"`, `GroupSplit = "SPLIT"`.
- `type SessionGroup struct { ID, SessionID int64; No int; Phase GroupPhase; PhaseStartedAt, CreatedAt time.Time }`.
- `func NextGroupPhase(phase GroupPhase, size, onCourt int, teamed, splitDone bool) GroupPhase` — size 2: TOGETHER→SPLIT when `onCourt == 2 && teamed`; size 4: TOGETHER→SPLIT when `onCourt == 4`; SPLIT→TOGETHER when `splitDone`; size 3 or other: unchanged.
- [ ] Table test `TestNextGroupPhase` covering each row. Commit.

### Task D: Storage + migration
- `migrations/0003_session_groups.sql` per spec (create table, add `group_id`, migrate from `group_no`, drop `group_no`).
- `Queries`: `InsertSessionGroup(ctx, SessionGroup) (int64, error)`, `ListSessionGroups(ctx, sessionID) ([]SessionGroup, error)`, `GetSessionGroup(ctx, id)`, `UpdateSessionGroup(ctx, SessionGroup) error`, `DeleteEmptySessionGroups(ctx, sessionID) error`. `session_players` read/write `group_id`.
- [ ] Test migration on a copy: rows with `group_no` end up with matching `group_id` per (session, no). Storage round-trip test. Commit.

### Task E: Session usecases
- `State.Groups []domain.SessionGroup`.
- `CheckIn(ctx, sessionID, playerID int64, group GroupChoice)` with `type GroupChoice struct { ID int64; New bool }`; `SetPlayerGroup(ctx, sessionPlayerID int64, group GroupChoice)`. New → insert group with smallest unused `No`, phase TOGETHER. Full → `ErrGroupFull`. Then `DeleteEmptySessionGroups`.
- `Confirm`: after players go PLAYING, for each group with members in the match compute `onCourt`, `teamed` (two members on one team), `splitDone` (every member has a match with `started_at >= PhaseStartedAt`, counting this one); if `NextGroupPhase` differs, update phase and `PhaseStartedAt = now`.
- `candidateOf` sets `Group`, `GroupSize`, `GroupPhase`.
- [ ] Tests: `TestCheckInWithNewGroup`, `TestCheckInIntoFullGroupFails`, `TestPairPhaseAlternates` (size 2: teamed confirm → SPLIT; both play again → TOGETHER), `TestFourGroupSplitsAfterWholeMatch`, `TestEmptyGroupDeleted`. Update existing group tests to `GroupChoice`. Commit.

### Task F: Matchmaking by phase
- `Candidate.GroupSize int`, `Candidate.GroupPhase domain.GroupPhase`; `func (c *Candidate) bound() bool` = group set, phase TOGETHER, size 2 or 4.
- `GroupQueue`, `newGroupRule`, `MateHeld` consider bound members only.
- `Evaluate`: a same-group team counts −1 in `GroupedTeams` when its group is size 2 SPLIT, +1 otherwise.
- `groupScore` per group present (k on court, t teamed pairs, f facing): size2 TOGETHER `t==1 → +1`; size2 SPLIT `t==1 → −1`; size4 TOGETHER `k==4 → +3`, else `+t` if `!f`; size4 SPLIT `k==4 or f → −1`, else `+t`; size3 `f → −1` else `+t`.
- [ ] Tests: `TestPairTogetherPhaseTeamsUp`, `TestPairSplitPhaseSeparates`, `TestFourTogetherPhasePlaysWhole`, `TestFourSplitPhasePairsFaceOutsiders` (pair rotates: recently teamed pair not reused), `TestThreeGroupOnePairTeamed`, `TestSplitPhaseDoesNotWait` (GroupQueue keeps SPLIT member while mate plays). Existing tests: zero `GroupSize` means old behaviour? No — update helpers to set size/phase. Commit.

### Task G: Simulation phases
- Simulation keeps `SessionGroup`-like state per group, calls `domain.NextGroupPhase` on confirm, fills candidate fields; prints per group size `on-rhythm: X%` (TOGETHER phases completed / phases entered).
- [ ] Full simulation set PASS. If FAIL, try group score below scarcity / adjust; report to host if still failing. Commit.

### Task H: HTTP + frontend
- DTO player: `group` (id), `group_no`, `group_phase`; state: `groups`.
- `POST /api/sessions/{id}/players` body `{player_id, group_id?, new_group?}`; `PUT /api/session-players/{id}/group` body `{group_id, new_group?}` (0 = none).
- `groupOptions(state)` → `Không`, `N<no> (<count>)` for groups with room, `Nhóm mới`. Add-player sheet holds selection; after `Nhóm mới` check-in selects the created group. Queue tag `N1 · chung|tách`.
- [ ] `npm run build`; browser check at 375px. Commit.

### Task I: Docs + final
- [ ] `docs/raw_requirement.md` 5.1, 6.7, 6.8 rewritten for phases (replace 2026-11-16 quota paragraph), log 2026-11-17; CLAUDE.md 4.4. Commit.
- [ ] `go test ./...`, full simulation, final review, report.
