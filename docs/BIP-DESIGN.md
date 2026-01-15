# Ball-in-Play (BiP) Design Specification

This document serves as the authoritative reference for all Ball-in-Play (BiP) logic within the Skorekeeper application. It defines the expected behavior for user interactions, outcome determination, and runner advancements.

## 1. Scope

**Ball-in-Play (BiP)** refers to any event where the batter puts the ball into play or reaches base via a non-pitch event (e.g., Dropped 3rd Strike, Catcher's Interference).

## 2. User Interaction (CSO Modal)

The Contextual Scoring Overlay (CSO) provides the interface for recording BiP events.

### 2.1 Standard Mode
*   **Result**: User selects from Ground, Fly, Line, IFF, Safe, or Out.
*   **Type**: User selects a sub-type (e.g., HIT, ERR, FC, SF, SH).
*   **Sequence**: User records the defensive sequence (e.g., 6-3).
*   **Automation**: Double Plays (DP) and Triple Plays (TP) are NOT manually selectable. They are determined by the total number of outs recorded on the play.

### 2.2 Dropped 3rd Strike Mode
*   **Trigger**: Long-press "Strike" and select "Dropped".
*   **Safe Outcomes**:
    *   **D3**: Batter reaches 1st base safely.
    *   **FC**: Batter reaches 1st base safely via fielder's choice.
*   **Out Outcomes**:
    *   **K**: Batter is put out (e.g., tagged or thrown out).
*   **Sequence**: Optional. If provided, it is appended to the outcome (e.g., "K 2-3").

## 3. Outcome Text Generation

The system generates a concise text representation of the play based on the following rules:

### 3.1 Standard Result Codes
| Result | Type | Outcome Text | Example |
| :--- | :--- | :--- | :--- |
| Safe | HIT | Reached base code | 1B, 2B, 3B, HR |
| Safe | ERR | E + sequence | E-5 |
| Safe | FC | FC + sequence | FC-6 |
| Fly | OUT | F + sequence | F8 |
| Fly | SF | SF + sequence | SF8 |
| Line | OUT | L + sequence | L4 |
| IFF | OUT | IFF + sequence | IFF4 |
| Ground | OUT | Sequence | 6-3 |
| Ground | SH | SH + sequence | SH5-3 |

### 3.2 Automated DP/TP Prefixes
*   **2 Outs**: Prefix outcome with `DP ` (e.g., `DP 6-4-3`).
*   **3 Outs**: Prefix outcome with `TP ` (e.g., `TP F8`).

### 3.3 Dropped 3rd Strike Codes
*   **Safe**: `D3` (Standard) or `FC`. Sequence is optional.
*   **Out**: `K`. Sequence is optional (e.g., `K` or `K 2-3`).

## 4. Runner Advancement Logic

When a BiP event is recorded, the system calculates default advancements for runners on base.

### 4.1 Non-Air Outs (Default +1 Base)
Includes: Safe hits, Errors, Fielder's Choice, Ground Outs, and Dropped 3rd Strikes.
*   **Forced Runners**: All runners forced to move by the batter becoming a runner default to advancing one base (e.g., 1st -> 2nd).
*   **Non-Forced Runners**: Typically stay at their current base by default unless the batter reaches safely on a multi-base hit (e.g., everyone moves +2 on a Double).

### 4.2 Air Outs (No Default Advancement)
Includes: Fly outs, Line outs, Pop outs, and Infield Fly.
*   **All Runners**: Default to "Stay" to prevent automatic "doubling off" errors. Users must manually record tag-ups or advances.

## 5. Hit Data and Trajectory

*   **Coordinates**: Relative (x, y) coordinates mapped to the field diagram.
*   **Default Trajectories**:
    *   Fly / Pop / IFF -> `Fly` (Arcing path)
    *   Line -> `Line` (Straight path)
    *   Ground / Safe -> `Ground` (Dashed path)
*   **Visuals**: Air outs are marked with an 'X' at the hit location on the diamond view.
