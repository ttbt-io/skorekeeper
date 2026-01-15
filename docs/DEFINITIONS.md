# Skorekeeper Definitions & Statistics Reference

This document serves as the authoritative glossary for all baseball/softball terminology, acronyms, and statistical formulas used within the Skorekeeper application. It is intended for developers, AI coding assistants, and users to ensure consistent understanding and implementation of game logic.

## 1. Game Events & Outcomes

These codes represent the result of a Plate Appearance (PA) or specific in-game actions.

### Hit Types
*   **1B (Single):** Batter reaches first base safely on a hit.
*   **2B (Double):** Batter reaches second base safely on a hit.
*   **3B (Triple):** Batter reaches third base safely on a hit.
*   **HR (Home Run):** Batter scores a run on a hit.
    *   *Note:* Inside-the-park home runs are also recorded as HR.

### Outs
*   **K (Strikeout Swinging):** Batter strikes out swinging. Recorded as an Out.
*   **ꓘ (Strikeout Looking):** Batter strikes out without swinging. Recorded as an Out.
*   **D3 (Dropped 3rd Strike):** Batter reaches first base safely after a dropped 3rd strike. Recorded as a Strikeout for the pitcher, but NOT an Out for the team. Can also result in an Out (K) or Fielder's Choice (FC).
*   **F [Pos] (Fly Out):** Batter hits a fly ball caught by a fielder (e.g., F8 = Fly out to Center Field).
*   **L [Pos] (Line Out):** Batter hits a line drive caught by a fielder.
*   **P [Pos] (Pop Out):** Batter hits a pop-up caught by a fielder.
*   **[Sequence] (Ground Out):** Batter hits a ground ball resulting in an out (e.g., 6-3 = Shortstop throws to First Base).
*   **IFF (Infield Fly):** Infield Fly Rule declared. Batter is automatically out.
*   **SF (Sacrifice Fly):** Batter hits a fly ball that scores a runner. Not counted as an At Bat (AB).
*   **SH (Sacrifice Hit/Bunt):** Batter bunts to advance a runner and is put out. Not counted as an At Bat (AB).
*   **DP (Double Play):** Defense records two outs on one play. Automatically detected and prefixed in the application.
*   **TP (Triple Play):** Defense records three outs on one play. Automatically detected and prefixed in the application.

### Other Outcomes
*   **BB (Base on Balls / Walk):** Batter advances to 1st after 4 balls. Not an AB.
*   **IBB (Intentional Walk):** Batter is intentionally walked. Not an AB.
*   **HBP (Hit By Pitch):** Batter is hit by the pitch and awarded 1st. Not an AB.
*   **E [Pos] (Error):** Batter reaches base or advances due to a defensive error (e.g., E5). Counts as an AB (0 for 1).
*   **FC (Fielder's Choice):** Batter reaches base because the defense attempted to put out another runner. Counts as an AB (0 for 1).
*   **CI (Catcher's Interference):** Batter awarded 1st due to interference. Not an AB.
*   **INT (Interference):** Offensive interference. Batter/Runner is out.
*   **BOO (Batting Out of Order):** Penalty out recorded against the proper batter when an improper batter completes their turn.

## 2. Base Running Actions

*   **SB (Stolen Base):** Runner advances without a hit or error.
*   **CS (Caught Stealing):** Runner is put out while attempting to steal.
*   **PO (Picked Off):** Runner is put out by the pitcher/catcher while off base.
*   **LE (Left Early):** Runner leaves the base before the pitch or on a fly ball. Recorded as an Out.
*   **CR (Courtesy Runner):** A substitute runner allowed for the pitcher or catcher (or per league rules) that does not count as a formal substitution.
*   **Adv (Advanced):** Runner advances on a play (e.g., wild pitch, passed ball, or defensive indifference).
*   **WP (Wild Pitch):** Pitcher throws a ball the catcher cannot handle.
*   **PB (Passed Ball):** Catcher misses a playable pitch.

## 3. Statistical Definitions & Formulas

### Hitting Statistics

| Stat | Name | Formula / Definition |
| :--- | :--- | :--- |
| **G** | Games Played | Total number of unique games the player appeared in. |
| **PA** | Plate Appearances | Total completed turns at bat. |
| **AB** | At Bats | `PA - (BB + IBB + HBP + SF + SH + CI)` |
| **H** | Hits | `1B + 2B + 3B + HR` |
| **K** | Strikeouts | Total of K (swinging) and ꓘ (looking). |
| **F** | Flyouts | Outcomes starting with `F` (Fly) or `P` (Pop). |
| **L** | Lineouts | Outcomes starting with `L` (Line). |
| **G** | Groundouts | Outcomes with defensive sequences (e.g., `6-3`) or explicitly marked ground balls. |
| **O** | Other Outs | Outcomes like `IFF` (Infield Fly), `INT` (Interference), or penalty outs. |
| **BB** | Walks | Total of BB and IBB. |
| **ROE** | Reached on Error | Total times reached base safely on a defensive error (outcomes starting with `E`). |
| **HBP** | Hit By Pitch | Total times hit by a pitch. |
| **CS** | Called Strikes | Total pitches of type `strike` where the batter did not swing, plus ꓘ outcomes. |
| **R** | Runs | Total times the player touched home plate safely. |
| **RBI** | Runs Batted In | Runs scored as a direct result of the batter's action. |
| **AVG** | Batting Average | `H / AB` |
| **OBP** | On-Base Percentage | `(H + BB + HBP) / (AB + BB + HBP + SF)` |
| **SLG** | Slugging Percentage | `(1B + 2*2B + 3*3B + 4*HR) / AB` |
| **OPS** | On-Base + Slugging | `OBP + SLG` |

### Pitching Statistics

| Stat | Name | Formula / Definition |
| :--- | :--- | :--- |
| **IP** | Innings Pitched | `Total Outs Recorded / 3`. Often displayed as `X.Y` where Y is 1 or 2 outs. |
| **BF** | Batters Faced | Total count of plate appearances against this pitcher. |
| **H** | Hits Allowed | Total hits surrendered. |
| **K** | Strikeouts | Total strikeouts recorded. |
| **BB** | Walks | Total walks issued (including IBB). |
| **HBP** | Hit By Pitch | Total batters hit by a pitch. |
| **DO** | Defensive Outs | `Total Outs Recorded - K`. (Outs made by the defense behind the pitcher). |
| **E** | Errors | Total defensive errors committed by the pitcher. |
| **B** | Balls | Total pitches recorded as 'ball'. |
| **S** | Strikes | Total pitches of type 'strike', 'foul', 'out', plus any ball put 'in-play'. |
| **ERA** | Earned Run Average | `(Earned Runs * Standard Innings) / IP`. (Standard Innings = 7). |
| **WHIP** | Walks + Hits per IP | `(BB + H) / IP` |
| **Str%** | Strike Percentage | `S / (B + S)` |
| **BB%** | Walk Percentage | `BB / BF` |
| **K%** | Strikeout Percentage | `K / BF` |
| **PC** | Pitch Count | `B + S` |

**Note on Ball-in-Play (BiP):** For statistical purposes, any ball put into play (regardless of the outcome) is counted as a **Strike** for the pitcher's strike percentage and pitch count.

**Note on Earned Runs (ER):** Currently, the system simplifies ER calculation by treating most runs as Earned unless manually flagged. Future updates may implement complex reconstruction of innings to determine unearned runs caused by errors.

## 4. Scorekeeping Concepts

*   **Active Game:** The game currently being scored or viewed.
*   **Action Log:** An append-only list of every event (`PITCH`, `UNDO`, `SUBSTITUTION`) that serves as the single source of truth for the game state.
*   **Snapshot/State:** The computed result (score, outs, runners) derived from replaying the Action Log.
*   **Context (Ctx):** Defines the "cursor" position in the grid: `{ b: BatterIndex, i: Inning, col: ColumnID }`.
*   **Shadow State:** Temporary UI state (e.g., pending runner moves) that exists before an Action is dispatched to the log.

## 5. Abbreviations

*   **CSO:** Contextual Scoring Overlay (The main input modal).
*   **BiP:** Ball in Play.
*   **LOB:** Left on Base (Runners stranded at end of inning).
