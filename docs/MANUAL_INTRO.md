# Introduction to Scorekeeping

Welcome to the art of scorekeeping! Whether you are a parent tracking your child's game or a fan wanting to keep a detailed record, scorekeeping is a rewarding way to engage with baseball and softball.

Skorekeeper is designed to bridge the gap between traditional paper scorecards and modern digital tracking. This guide will introduce you to the basics of scorekeeping and show you how to apply them within the app.

## The Scorecard Grid

In a traditional scorebook, the game is laid out in a grid.
*   **Rows** represent the batters in the lineup.
*   **Columns** represent the innings.

The intersection of a row and a column is a **Plate Appearance (PA) Cell**. This is where you record what happened when that specific player came to bat.

### The Cell Layout

In Skorekeeper, each cell represents the baseball diamond and tracks the status of the at-bat:

![Empty Cell](../frontend/assets/manual/cell-empty.png)

1.  **The Diamond:** The central square represents the four bases.
    *   Bottom: Home Plate
    *   Right: 1st Base
    *   Top: 2nd Base
    *   Left: 3rd Base
2.  **Balls & Strikes (Top Left):** The dots track the current count. Top row for 4 Balls, bottom row for 3 Strikes.
3.  **Outs (Top Right):** If the batter is put out, a circle appears with the number (1, 2, or 3) indicating their out position in the inning.
4.  **Outcome (Bottom Right):** The text describes the result of the play (e.g., "1B", "F8", "K").

## Recording Plays

Here is how common plays are recorded on paper and how they appear in Skorekeeper.

### 1. The Strikeout (K)

A strikeout occurs when a batter accumulates three strikes.
*   **K**: Swinging strikeout.
*   **ꓘ** (Backwards K): Called strikeout (the batter did not swing).

![Strikeout](../frontend/assets/manual/cell-strikeout.png)

### 2. The Walk (BB)

A "Base on Balls" (BB) occurs when the batter receives four balls.
*   **Paper:** A line is drawn from Home to 1st Base and "BB" is written.
*   **App:** The path to 1st base is filled in, and "BB" is displayed in the corner.

![Walk](../frontend/assets/manual/cell-walk.png)

### 3. Hits (1B, 2B, 3B, HR)

When a batter hits the ball safely:
*   **Single (1B):** Batter reaches 1st base. Path to 1st is filled.
*   **Double (2B):** Batter reaches 2nd base. Paths to 1st and 2nd are filled.
*   **Triple (3B):** Batter reaches 3rd base. Paths to 1st, 2nd, and 3rd are filled.
*   **Home Run (HR):** Batter scores. All four paths are filled, and the diamond is shaded in.

![Single](../frontend/assets/manual/cell-single.png) ![Double](../frontend/assets/manual/cell-double.png) ![Home Run](../frontend/assets/manual/cell-homerun.png)

### 4. Outs on Balls in Play

Defensive players are assigned numbers 1-9. We use these to record how an out was made:
1. Pitcher | 2. Catcher | 3. 1st Base | 4. 2nd Base | 5. 3rd Base | 6. Shortstop | 7. Left Field | 8. Center Field | 9. Right Field

*   **Fly Out:** Caught in the air. "F8" means the Center Fielder (8) caught it.
*   **Ground Out:** Hit on the ground. "6-3" means the Shortstop (6) threw to the 1st Baseman (3) for the out.

![Fly Out](../frontend/assets/manual/cell-flyout.png) ![Ground Out](../frontend/assets/manual/cell-groundout.png)

### 5. Moving Runners

One of the best parts of a scorecard is seeing runners advance. 

**Scenario:** Batter 1 hits a single (1B). Batter 2 then hits a double (2B), and Batter 1 moves from 1st to 3rd base. Batter 1's cell will update to show his progress to 3rd, even though his own at-bat only got him to 1st.

![Runner Advance](../frontend/assets/manual/cell-advance.png)

Skorekeeper handles these advancements automatically based on the play, keeping your scorecard perfectly in sync.

## Behind the Scenes: AI-Assisted Development

Skorekeeper is a showcase of "AI-in-the-loop" software engineering. This entire application was architected, implemented, and documented with the assistance of Google's Gemini AI. Gemini acted as a senior technical lead and pair programmer, ensuring that every line of code follows best practices for security, reliability, and performance, while providing the user-friendly experience you see today.
