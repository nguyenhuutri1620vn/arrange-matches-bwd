# Tránh 3-1 chặt hơn, nhóm theo tỉ lệ, chọn nhóm lúc check-in — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fewer 3-1 lineups, group pairs teamed up about 2/3 of their matches, and a group picker in the "Thêm người" sheet.

**Architecture:** 3-1 avoidance widens the candidate window by one priority tier when the best lineup is a penalized 3-1. Group quota adds per-candidate session counts (`GroupMatches`, `GroupTeamed`) loaded by `session` from match history and ranked in `matchmaking` against a configurable `GroupTeamPct`. Check-in takes an optional group in the same transaction.

**Tech Stack:** Go 1.26, SQLite (`modernc.org/sqlite`), React + Vite + TypeScript.

**Spec:** `docs/superpowers/specs/2026-11-16-tranh-3-1-va-nhom-theo-ti-le-design.md`

## Global Constraints

- Fairness (`PriorityCount` order, window expansion keeps most players at `min`), gender rules and `BalanceThreshold` stay above every new criterion.
- `go run ./simulation -matrix -runs 1000` must PASS, also with `-groups 2`, `-groups 3`, `-groups 4` and `-group-size 3`, `-group-size 4` (CLAUDE.md 7.2; 3% tolerance only for groups from 2 courts).
- No DB schema change. New setting is a row in the key-value `settings` table, key `group_team_pct`, default `67`, valid `0..100`.
- Integer comparisons only: behind quota ⇔ `GroupMatches == 0 || 100*GroupTeamed < GroupTeamPct*GroupMatches`.
- UI text Vietnamese, code/comments English. Buttons ≥ 44px. Commit straight to `develop`, one logical change per commit, `go test ./...` before each commit.
- Every commit ends with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. A lone woman at the lowest count when the rest are men: the widened window must not leave her waiting more than one turn (existing "rested ≥1 → 3-1 not penalized" exception keeps her in). Test in Task 3.
2. Host changes a player's group mid-session: earlier matches with the old group count as "not teamed", so the player is behind quota and gets lifted. Accepted; pinned by a test in Task 5.
3. Settings saved from an old frontend build without `group_team_pct` would send 0. The DTO must treat a missing field as the stored/default value, not 0. Test in Task 4.
4. Check-in into a full group must fail with `ErrGroupFull` and must not check the player in (same transaction). Test in Task 7.
5. Group with 3-4 players: quota counts per player, a whole-group match (mates facing each other) must not count as teamed for the facing pairs. Test in Task 5.

---

### Task 1: Requirement doc

**Files:**
- Modify: `docs/raw_requirement.md` (5.1, 6.6, 6.7, 6.8, change log)

- [ ] **Step 1:** In 5.1 add: host may pick the group in "Thêm người" before check-in; the choice stays for the next check-ins.
- [ ] **Step 2:** In 6.6 add after the line about 3-1 only when no 4-0/2-2 among the due players: "Nếu chỉ còn 3-1, app mở rộng sang người hơn 1 trận (giữ tối đa người ở mức thấp nhất) để tìm 4-0 hoặc 2-2; không có mới dùng 3-1. Ngoại lệ người lẻ giới đã ngồi ngoài 1 trận giữ nguyên."
- [ ] **Step 3:** In 6.7 add "Tỉ lệ đứng chung": per pair target `group_team_pct` (default 67%) of a player's matches in the session; below it → priority to team with a group mate; at/above → group pairing ranks below mixing so pairs split for variety. Whole-group facing match is not "teamed".
- [ ] **Step 4:** In 6.8 note the quota criterion's position (filled in after Task 6 tuning; write "trên người ngồi ngoài 2 trận" now, adjust in Task 6).
- [ ] **Step 5:** Change log line `2026-11-16: Tránh 3-1 chặt hơn (phương án A), nhóm đứng chung theo tỉ lệ 2/3, chọn nhóm lúc check-in. Host duyệt.`
- [ ] **Step 6:** Commit together with the spec and this plan (`docs/superpowers/`): `docs: stricter 3-1 avoidance, group team quota, group at check-in`.

### Task 2: Simulation metrics

**Files:**
- Modify: `simulation/main.go`

**Interfaces:**
- Produces: each result line followed by `  3-1: X.X% of matches  group teamed: Y.Y% of group players' matches` (group part only when `-groups > 0`).

- [ ] **Step 1:** Add to `result`/`outcome` the counts `matches`, `oddMatches`, `groupPlayerMatches`, `groupTeamed`; count in `runSession` per finished lineup (odd = men count 1 or 3; teamed = teammate has same non-zero `Group`). No globals.
- [ ] **Step 2:** Run `go run ./simulation -matrix -runs 1000` and `-groups 2`, `-groups 1 -group-size 4`. Expected: PASS, numbers close to the spec's baseline (3-1 16–26% on 1 court; group teamed 33–37% on 1 court). Save the output in the commit message body as the baseline.
- [ ] **Step 3:** Commit `simulation: report 3-1 rate and group-teamed rate`.

### Task 3: Widen the window before accepting 3-1

**Files:**
- Modify: `internal/matchmaking/matchmaking.go` (`proposeFrom`, `bestInWindow`)
- Test: `internal/matchmaking/matchmaking_test.go`

**Interfaces:**
- `bestInWindow(sorted []Candidate, end int, rules domain.MatchRules, useLookahead bool, groups *groupRule, skipFreshOdd bool) (Lineup, bool, bool)` — third result reports the chosen option's `oddGender`. With `skipFreshOdd`, options where `oddOneOutFresh(four)` is true are skipped before tie-keeping.
- `proposeFrom`: when a window's best is fresh-odd and a next window exists (`windowEnd(sorted, end) > end`), try `bestInWindow(sorted, next, …, skipFreshOdd=true)`; use it if found, else keep the 3-1. Only one tier of widening.

- [ ] **Step 1: Write failing tests**
  - `TestProposeWidensWindowToAvoidThreeOne`: men 1,2,3 (TB) and woman 4 (TBY) at count 0 with rest 0; woman 5 (TBY) at count 1. Want `contains(ids,4) && contains(ids,5)` and exactly two of 1,2,3.
  - Update `TestProposeAvoidsThreeOneWhenMixedLineupExists` third case (woman 5 at count 1, woman 4 rest 0): now expects 2-2 with 4 and 5. Comment why (spec 2026-11-16, phương án A).
  - `TestProposeLoneRestedWomanStillPlaysThreeOne`: same as above but woman 4 `ConsecutiveRest = 1` → want `{1,2,3,4}`.
  - `TestProposeNoWiderTierKeepsThreeOne`: 1,2,3 men + 4 woman all count 0, nobody else → want `{1,2,3,4}`.
- [ ] **Step 2:** `go test ./internal/matchmaking -run 'ThreeOne' -v` → new/updated tests FAIL.
- [ ] **Step 3:** Implement as in Interfaces.
- [ ] **Step 4:** `go test ./...` → PASS.
- [ ] **Step 5:** `go run ./simulation -matrix -runs 1000` and with `-groups 2|3|4`, `-group-size 3|4` → all PASS; 3-1 rate lower than Task 2 baseline. If any FAIL, stop and report numbers to the host (do not loosen the criterion).
- [ ] **Step 6:** Commit `matchmaking: widen to the next tier before proposing 3-1` with before/after 3-1 rates.

### Task 4: `GroupTeamPct` setting

**Files:**
- Modify: `internal/domain/settings.go`, `internal/storage/players.go`, `internal/httpapi/dto.go`, `web/src/api.ts`, `web/src/pages/SettingsPage.tsx`
- Test: `internal/domain` (settings test file, create `settings_test.go` if none), `internal/httpapi` existing tests or new `dto_test.go`

**Interfaces:**
- `domain.MatchRules.GroupTeamPct int` (default 67). `Validate` returns `ErrInvalidInput` when `< 0 || > 100`.
- Storage key `group_team_pct`.
- DTO field `GroupTeamPct *int \`json:"group_team_pct"\``; `toDomain(current domain.Settings)` keeps `current.Rules.GroupTeamPct` when nil. Update the settings handler to pass the stored settings.
- Settings page: number input "Tỉ lệ nhóm đứng chung đội (%)", min 0, max 100, using the same `setRule` helper.

- [ ] **Step 1: Failing tests:** `TestDefaultGroupTeamPct` (=67), `TestValidateGroupTeamPctRange` (101 and -1 rejected), `TestSettingsDTOKeepsGroupTeamPctWhenMissing` (decode JSON without the field → stored 67 kept), storage round-trip in existing storage tests if present.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** `go test ./...` PASS; `cd web && npm run build` PASS.
- [ ] **Step 5:** Commit `settings: add group team quota (default 67%)`.

### Task 5: Group counts on candidates

**Files:**
- Modify: `internal/matchmaking/matchmaking.go` (`Candidate`), `internal/session/service.go` (`history`, `loadHistory`, `candidateOf`), `internal/session/matches.go` (callers), `simulation/main.go` (`candidate`, `simPlayer`)
- Test: `internal/session/service_test.go`, `internal/matchmaking/matchmaking_test.go`

**Interfaces:**
- `Candidate.GroupMatches int` — the player's ONGOING+FINISHED matches in this session.
- `Candidate.GroupTeamed int` — of those, matches whose teammate has the player's current non-zero `Group`.
- `func (c *Candidate) behindGroupQuota(pct int) bool` — false when `Group == 0`; otherwise the Global Constraints formula.
- `loadHistory` reads all session matches; the lookback limit applies only to `teammates`/`opponents`. `recent` gains `played int` and `teamedWith []int64` (all-session teammates); `candidateOf` computes `GroupTeamed` using the current groups of `byID`, so it needs the session players map (adjust its signature to `candidateOf(p *domain.SessionPlayer, h history, groupOf map[int64]int)`).
- Simulation `simPlayer` tracks `played`/`teamed` and fills the two fields.

- [ ] **Step 1: Failing tests:**
  - `TestBehindGroupQuota` at pct 67: Group 0 → false; Group 1 with 0 of 0 → true; 2 of 3 → true (200 < 201); 3 of 4 → false; 1 of 3 → true; 4 of 6 → true (400 < 402); 5 of 6 → false.
  - `TestCandidatesCountGroupTeamedMatches` (session): group of A,B plus C,D,E,F; play a match A+B vs C+D, then A+C vs B+E (facing mates), then finish; candidates show A `GroupMatches=2, GroupTeamed=1`.
  - `TestGroupTeamedUsesCurrentGroup`: A and C teamed earlier with no group, then A,B set to group 1 → A `GroupTeamed=0`.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** `go test ./...` PASS. Simulation unchanged results (fields unused yet).
- [ ] **Step 5:** Commit `session: load per-session group teamed counts for candidates`.

### Task 6: Rank by group quota

**Files:**
- Modify: `internal/matchmaking/matchmaking.go` (`Lineup`, `Evaluate`, `splitBetter`, `option`, `option.better`, `groupScore`, `bestInWindow`, doc comment listing the order)
- Test: `internal/matchmaking/matchmaking_test.go`

**Interfaces:**
- `Lineup.GroupLift int` — players on a same-group team who are `behindGroupQuota(rules.GroupTeamPct)`. Computed in `Evaluate`.
- `splitBetter` (balanced splits): `GroupLift` desc → `Repeats` asc → `GroupedTeams` desc → `Diff` asc.
- `option.grouped` becomes the lift score: `groupScore` returns `GroupLift` (instead of `GroupedTeams`) with the existing whole-group-facing penalty (-1). New `option.groupSated int` = `GroupedTeams` of teams whose players are both at/above quota, ranked after `courtMates`, before `rests` (more first).
- Placement of `grouped` in `better`: try first directly after `oddGender` (above `scarcity`); run the full simulation set. If any FAIL, put it back after `scarcity` (current place) and rerun. Record which placement passed.

- [ ] **Step 1: Failing tests:**
  - `TestArrangeSplitsSatedGroupPairForVariety`: four TB men, 1 and 2 in group 1 with `GroupMatches=3, GroupTeamed=3`, 1 and 2 recently teamed (`RecentTeammates`) → want 1 and 2 not teammates.
  - `TestArrangeKeepsBehindGroupPairTogether`: same but `GroupTeamed=1` → want teammates.
  - `TestProposeLiftsBehindPairAboveMixing`: reuse `eightAfterTwoMatches`, make 1 and 5 group 1 with `GroupMatches=2, GroupTeamed=0` → want 1 and 5 teammates.
  - Existing group tests keep passing (zero counts ⇒ behind quota ⇒ same behaviour as before).
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement with the higher placement.
- [ ] **Step 4:** `go test ./...` PASS.
- [ ] **Step 5:** Simulation set from Global Constraints. Expected: all PASS; group teamed rate near 67% on 2–3 courts and higher than baseline on 1 court. Fall back to the lower placement on any FAIL. If 1 court is still far below 2/3, keep the passing version and report the numbers.
- [ ] **Step 6:** Update `docs/raw_requirement.md` 6.8 with the final position.
- [ ] **Step 7:** Commit `matchmaking: team group pairs toward a 2/3 quota, split when sated` with before/after rates.

### Task 7: Group at check-in

**Files:**
- Modify: `internal/session/players.go` (`CheckIn`, extract room check from `SetPlayerGroup`), `internal/httpapi/handlers.go` (`checkIn`), all `CheckIn` callers (`internal/session/service_test.go`, others found by `grep -rn "CheckIn(" internal cmd`), `web/src/api.ts`, `web/src/pages/SessionPage.tsx` (`AddPlayerSheet`, `GroupPicker`)
- Test: `internal/session/service_test.go`

**Interfaces:**
- `func (s *Service) CheckIn(ctx context.Context, sessionID, playerID int64, group int) (*State, error)` — `group < 0` → `ErrInvalidGroup`; group full → `ErrGroupFull`, nothing inserted (same transaction).
- `func groupHasRoom(ctx context.Context, q *storage.Queries, sessionID int64, group int) error` used by both `CheckIn` and `SetPlayerGroup`.
- Request body `{ "player_id": number, "group"?: number }` (missing = 0).
- `api.checkIn(sessionId: number, player_id: number, group = 0)`.
- Frontend: extract `groupOptions(players: SessionPlayer[], current: number): [number, string][]` from `GroupPicker` (Không, groups in use with room, next new number labeled "Nhóm mới"); `AddPlayerSheet` holds `group` state (default 0), shows a "Nhóm đi chung" `Segmented` above the search, passes it to both check-in paths, and keeps it after each check-in (recompute options from fresh state so a just-filled group drops out; if the selected group becomes full, reset to 0).

- [ ] **Step 1: Failing tests:** `TestCheckInWithGroup` (player lands in group 2), `TestCheckInIntoFullGroupFails` (4 in group 1, 5th check-in with group 1 → `ErrGroupFull`, player count unchanged).
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement backend; update callers with `0`.
- [ ] **Step 4:** `go test ./...` PASS.
- [ ] **Step 5:** Implement frontend; `cd web && npm run build` PASS.
- [ ] **Step 6:** Verify in the browser preview at 375px: open "Thêm người", pick "Nhóm mới", check in two players → both show tag `N<n>` in the queue; pick a full group not offered. Screenshot.
- [ ] **Step 7:** Commit `Pick a group when checking players in`.

### Task 8: Final check and report

- [ ] **Step 1:** `go test ./...` → PASS.
- [ ] **Step 2:** Full simulation set (Global Constraints) at `-runs 1000` → PASS.
- [ ] **Step 3:** Update CLAUDE.md 4.2/4.5 lines only if a rule's wording changed (3-1 widening, group quota), and 7.3 commands if any changed.
- [ ] **Step 4:** Report to the host: before/after table of 3-1 rate and group teamed rate per court count, placement chosen in Task 6, anything not reaching 2/3.
