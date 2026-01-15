# Game Scenario 3: Oddballs vs Rulebenders (Testing Advanced Features)

**Teams:**
*   **Away:** Oddballs (12 Players)
*   **Home:** Rulebenders (12 Players)

**Focus:**
*   Double Plays (DP) & Triple Plays (TP).
*   Batter Out of Order (BOO) - Penalty, Correction, and Moving Plays.
*   Complex Base Running.

## Rosters

| Slot | Oddballs (Away) | Pos | Rulebenders (Home) | Pos |
| :--- | :--- | :--- | :--- | :--- |
| 1 | A. Odd (#1) | SS | B. Bender (#13) | CF |
| 2 | C. Weird (#2) | 2B | D. Twist (#14) | LF |
| 3 | E. Strange (#3) | 1B | F. Loop (#15) | SS |
| 4 | G. Quirk (#4) | P | H. Knot (#16) | 3B |
| 5 | I. Peculiar (#5) | C | J. Kink (#17) | 1B |
| 6 | K. Bizarre (#6) | 3B | L. Warp (#18) | C |
| 7 | M. Curious (#7) | LF | N. Coil (#19) | P |
| 8 | O. Rare (#8) | CF | P. Spiral (#20) | 2B |
| 9 | Q. Unique (#9) | RF | R. Arc (#21) | RF |
| 10 | S. Freak (#10) | EP | T. Curve (#22) | EP |
| 11 | U. Unusual (#11) | EH | V. Swerve (#23) | EH |
| 12 | W. Wild (#12) | F | X. Zigzag (#24) | F |

---

## Inning 1

**Top 1 (Oddballs):**
1.  **#1 Odd:** Hits a **Single (1B)**.
2.  **#2 Weird:** Hits a **Single (1B)**. Odd to 2nd.
3.  **#3 Strange:** Hits a **Single (1B)**. Bases Loaded.
4.  **#4 Quirk:** **Line Out (L6) into Triple Play (TP)**.
    *   Line drive caught by SS (Out 1).
    *   Runner at 3rd (#1 Odd) doubled off (6-5) (Out 2).
    *   Runner at 2nd (#2 Weird) doubled off (6-4) (Out 3).
    *   *Note: This tests the "Triple Play" button and manual runner out selection.*
    *   *Score: Oddballs 0, Rulebenders 0*

**Bottom 1 (Rulebenders):**
1.  **#13 Bender:** **Walk (BB)**.
2.  **#14 Twist:** **Ground Ball Double Play (GDP 6-4-3)**.
    *   Grounder to SS. Bender out at 2nd. Twist out at 1st.
    *   *Note: This tests the standard "Double Play" button logic.*
3.  **#15 Loop:** **Strikeout (K)**.
    *   *Score: Oddballs 0, Rulebenders 0*

## Inning 2

**Top 2 (Oddballs):**
*   **Scenario:** Batter Out of Order - **Correction Mid-AB**.
1.  **#5 Peculiar:** Bats.
    *   Scorer selects #5. Records 2 Balls.
    *   Scorer realizes it is actually **#6 Bizarre** at the plate (skipping #5?).
    *   *Action:* Use **"Correct Batter"** context menu on CSO header to swap active batter to **#6 Bizarre**.
    *   *Result:* #6 Bizarre is now batting with 2-0 count.
2.  **#6 Bizarre:** **Home Run (HR)**.
    *   *Note: #5 Peculiar is skipped for now? Or was it just a mistake? Let's assume lineup shuffle. #5 is skipped.*
3.  **#7 Curious:** **Strikeout (K)**.
4.  **#8 Rare:** **Ground Out (4-3)**.
5.  **#9 Unique:** **Fly Out (F8)**.
    *   *Score: Oddballs 1, Rulebenders 0*

**Bottom 2 (Rulebenders):**
*   **Scenario:** Batter Out of Order - **Penalty Out**.
1.  **#16 Knot:** Due up. But **#17 Kink** comes to plate.
    *   Scorer records **Single (1B)** for #17 Kink (clicking #17's slot).
    *   Defense Appeals. Umpire rules BOO.
    *   *Action:* Scorer uses **"Penalty Out (BOO)"** on **#16 Knot** (the proper batter).
    *   *Result:* #16 Knot is Out. #17 Kink's hit is nullified (removed from base).
    *   *Next Batter:* The proper batter following the out player is #17 Kink.
2.  **#17 Kink:** Bats (again/properly). **Strikeout (K)**.
3.  **#18 Warp:** **Single (1B)**.
4.  **#19 Coil:** **Ground Out (5-3)**. Warp to 2nd.
    *   *Score: Oddballs 1, Rulebenders 0*

## Inning 3

**Top 3 (Oddballs):**
*   **Scenario:** Scorer Mistake - **Move Play**.
1.  **#10 Freak:** Hits a **Double (2B)**.
    *   *Mistake:* Scorer accidentally recorded this in **#11 Unusual's** slot.
    *   *Action:* Scorer Right-Clicks #11's grid cell (containing the 2B). Selects **"Move Play To..."** -> Selects **#10 Freak**.
    *   *Result:* The 2B is transferred to #10 Freak. #11 Unusual is empty/pending.
2.  **#11 Unusual:** **Strikeout (K)**.
3.  **#12 Wild:** **Walk (BB)**.
4.  **#1 Odd:** **Infield Fly (IFF)**. (Batter Out).
5.  **#2 Weird:** **Strikeout (K)**.
    *   *Score: Oddballs 1, Rulebenders 0*

**Bottom 3 (Rulebenders):**
1.  **#21 Arc:** **Triple (3B)**.
2.  **#22 Curve:** **Suicide Squeeze / Bunt**.
    *   Batter bunts. Runner from 3rd scores. Batter safe at 1st.
    *   Record as **Single (1B)** + Adv Runner Home.
3.  **#23 Swerve:** **Strikeout (K)**.
4.  **#24 Zigzag:** **Double Play (Line Drive - Unassisted?)**.
    *   Line Drive to 1B (L3). Batter Out.
    *   Runner at 1st (#22 Curve) caught off base. 1B touches bag (3U).
    *   *Score: Oddballs 1, Rulebenders 1*

## Inning 4

**Top 4 (Oddballs):**
*   **Scenario:** Weird Outs.
1.  **#3 Strange:** **Strikeout (K)** but reaches on **Wild Pitch (WP)** (Dropped 3rd Strike).
2.  **#4 Quirk:** **Bunt**. Pop up to Catcher. **Fly Out (F2)**.
3.  **#5 Peculiar:** (Back in order). **Catcher Interference (CI)**.
    *   Batter awarded 1st. Strange to 2nd.
4.  **#6 Bizarre:** **Fielder's Choice (FC)**.
    *   Grounder to 3B. Strange forced out at 3rd (5). Peculiar to 2nd. Bizarre to 1st.
5.  **#7 Curious:** **Runner Interference (INT)**.
    *   Batted ball hits runner #5 Peculiar.
    *   Runner #5 Out. Batter #7 Safe (FC). Bizarre to 2nd.
    *   *Score: Oddballs 1, Rulebenders 1*

**Bottom 4 (Rulebenders):**
1.  **#13 Bender:** **Home Run (HR)**.
2.  **#14 Twist:** **Walk (BB)**.
3.  **#15 Loop:** **Hit by Pitch (HBP)**. Twist to 2nd.
4.  **#16 Knot:** **Fly Out (F8)**. Runners hold.
5.  **#17 Kink:** **Fly Out (F9)**. Runners hold.
6.  **#18 Warp:** **Strikeout (K)**.
    *   *Score: Oddballs 1, Rulebenders 2*

## Inning 5 (Final)

**Top 5 (Oddballs):**
1.  **#8 Rare:** **Walk (BB)**.
2.  **#9 Unique:** **Strikeout (K)**.
3.  **#10 Freak:** **Double Play (Strikeout - Throw Out)**.
    *   Batter Strikeout (K).
    *   Runner #8 Rare Caught Stealing 2nd (2-6).
    *   *Score: Oddballs 1, Rulebenders 2*

**Bottom 5 (Rulebenders):**
*   Game Over (Home team ahead, middle of 5th? Or play it out? Let's assume 7 inning game, just end here for scenario brevity).
*   *Score: Oddballs 1, Rulebenders 2*

---
**End of Game**
