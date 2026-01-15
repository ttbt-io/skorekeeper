# Skorekeeper Formal Schema

This document defines the formal schema for all data structures used in the Skorekeeper application, including the Game state, Team data, and the Action Log events.

> **Note:** The application strictly enforces **Schema Version 3**. Legacy data (Version 2 and older) is no longer supported and must be migrated.

## 1. Game Object (Snapshot/State)

The `Game` object represents the full state of a baseball/softball game at a specific point in time.

| Field | Type | Description |
| :--- | :--- | :--- |
| `id` | `string (UUID)` | Unique identifier for the game. |
| `schemaVersion` | `number` | The version of the data schema (currently 3). |
| `date` | `string (ISO 8601)` | The date and time the game started. |
| `location` | `string` | The venue where the game is played. |
| `event` | `string` | The name of the event or tournament. |
| `away` | `string` | Name of the away team. |
| `home` | `string` | Name of the home team. |
| `status` | `string` | Game status: `'ongoing'`, `'final'`. |
| `pitchers` | `object` | `{ away: string, home: string }` current pitchers. |
| `overrides` | `object` | `{ away: { [inning]: score }, home: { [inning]: score } }` manual score overrides. |
| `events` | `object` | Map of game events: `${team}-${batterIdx}-${colId}` -> `EventObject`. |
| `columns` | `array` | List of `ColumnObject`. |
| `roster` | `object` | `{ away: SlotObject[], home: SlotObject[] }`. |
| `subs` | `object` | `{ away: PlayerObject[], home: PlayerObject[] }` available substitutes. |
| `pitchLog` | `array` | Chronological list of `PitchLogEntry`. |
| `actionLog` | `array` | The authoritative list of `Action` objects. |
| `ownerId` | `string` | User ID of the game owner. |
| `permissions` | `object` | `Permissions` object. |
| `awayTeamId` | `string` | ID of the away team in the TeamStore. |
| `homeTeamId` | `string` | ID of the home team in the TeamStore. |

### 1.1 ColumnObject
| Field | Type | Description |
| :--- | :--- | :--- |
| `inning` | `number` | The inning number (1-based). |
| `id` | `string` | Unique ID for the column (e.g., `col-1-0`). |
| `team` | `string` | (Optional) Team this column belongs to if it's split. |
| `leadRow` | `object` | (Optional) `{ [team]: rowIdx }` row that starts this column. |

### 1.2 EventObject
| Field | Type | Description |
| :--- | :--- | :--- |
| `outcome` | `string` | The PA result (e.g., `'1B'`, `'K'`, `'F8'`). |
| `balls` | `number` | Final ball count for the PA. |
| `strikes` | `number` | Final strike count for the PA. |
| `outNum` | `number` | The out number when this event concluded (1, 2, or 3). |
| `paths` | `array(4)` | Runner status on [1st, 2nd, 3rd, Home]. (0: None, 1: Safe, 2: Out). |
| `pathInfo` | `array(4)` | Action associated with each path (e.g., `'SB'`, `'Adv'`, `'WP'`). |
| `pitchSequence` | `array` | List of `{ type, code, pitcher }`. |
| `pId` | `string` | Batter ID. |
| `hitData` | `object` | (Optional) `{ x, y, type }` hit location data. |
| `bipState` | `object` | (Optional) Details of the Ball in Play. |
| `scoreInfo` | `object` | (Optional) `{ rbiCreditedTo: string }`. |
| `outPos` | `array(4)` | (Optional) Visual position for out markers on paths. |

## 2. Team Object

| Field | Type | Description |
| :--- | :--- | :--- |
| `id` | `string (UUID)` | Unique identifier for the team. |
| `schemaVersion` | `number` | The version of the data schema (currently 3). |
| `name` | `string` | Full team name. |
| `shortName` | `string` | Abbreviated team name. |
| `color` | `string` | Hex color code for team branding. |
| `roster` | `array` | List of `Player` objects. |
| `ownerId` | `string` | User ID of the team owner. |
| `roles` | `object` | `TeamRoles` object. |
| `updatedAt` | `number` | Timestamp of last update. |

### 2.1 Player (TeamStore)
| Field | Type | Description |
| :--- | :--- | :--- |
| `id` | `string` | Unique player ID. |
| `name` | `string` | Full name. |
| `number` | `string` | Jersey number. |
| `pos` | `string` | Primary position. |

## 3. Action Log Event

Every change to a game is recorded as an `Action`.

| Field | Type | Description |
| :--- | :--- | :--- |
| `id` | `string (UUID)` | Unique action identifier. |
| `type` | `string` | One of the `ActionTypes` (see below). |
| `schemaVersion` | `number` | The version of the data schema (currently 3). |
| `payload` | `object` | Data specific to the action type. |
| `timestamp` | `number` | When the action occurred (Unix ms). |
| `userId` | `string` | User ID who performed the action. |

### 3.1 Action Types & Payloads

Each action in the log has a `type` and a corresponding `payload` structure.

#### `GAME_START`
Initializes the game state.
```json
{
  "id": "string (UUID)",
  "date": "string (ISO 8601)",
  "away": "string",
  "home": "string",
  "event": "string",
  "location": "string",
  "ownerId": "string",
  "permissions": "PermissionsObject",
  "initialRosters": {
    "away": "PlayerObject[]",
    "home": "PlayerObject[]"
  }
}
```

#### `PITCH`
Records a single pitch or an out result from a pitch.
```json
{
  "activeCtx": "Context",
  "activeTeam": "string ('away'|'home')",
  "batterId": "string",
  "type": "string ('ball'|'strike'|'foul'|'out')",
  "code": "string (e.g., 'Called', 'Swinging', 'Dropped')"
}
```

#### `PLAY_RESULT`
Records the outcome of a Ball in Play (BIP).
```json
{
  "activeCtx": "Context",
  "activeTeam": "string",
  "batterId": "string",
  "bipState": {
    "res": "string (e.g., 'Fly', 'Ground', 'Safe')",
    "base": "string (e.g., '1B', '2B', '3B', 'Home')",
    "type": "string (e.g., 'SF', 'SH', 'FC', 'ERR')",
    "seq": "string (e.g., '6-3')"
  },
  "hitData": {
    "x": "number",
    "y": "number",
    "type": "string"
  },
  "runnerAdvancements": "RunnerAdvancement[]"
}
```

#### `RUNNER_ADVANCE`
Records runner movement.
```json
{
  "activeCtx": "Context",
  "activeTeam": "string",
  "batterId": "string",
  "runners": [
    {
      "key": "string (EventKey)",
      "base": "number (0-2)",
      "outcome": "string ('To 2nd'|'To 3rd'|'Score'|'Out'|'Stay')"
    }
  ],
  "rbiEligible": "boolean",
  "outSequencing": "string ('BatterFirst'|'RunnersFirst')"
}
```

#### `SUBSTITUTION`
Replaces a player in the lineup.
```json
{
  "team": "string",
  "rosterIndex": "number (0-99)",
  "subParams": {
    "id": "string",
    "name": "string",
    "number": "string",
    "pos": "string"
  }
}
```

#### `LINEUP_UPDATE`
Updates the entire team lineup.
```json
{
  "team": "string",
  "teamName": "string",
  "roster": "SlotObject[] (standardized V3)",
  "subs": "PlayerObject[] (standardized V3)"
}
```

#### `SCORE_OVERRIDE`
Manual adjustment of inning scores.
```json
{
  "team": "string",
  "inning": "number",
  "score": "string"
}
```

#### `GAME_IMPORT`
Imports a full game state (legacy or backup).
```json
{
  "id": "string (UUID)",
  "date": "string",
  "away": "string",
  "home": "string",
  "...": "Full Game Object fields"
}
```

#### `PITCHER_UPDATE`
Changes the current pitcher.
```json
{
  "team": "string",
  "pitcher": "string"
}
```

#### `MOVE_PLAY`
Moves a PA result to a different column.
```json
{
  "sourceKey": "string (EventKey)",
  "targetKey": "string (EventKey)"
}
```

#### `CLEAR_DATA`
Resets a PA event.
```json
{
  "activeCtx": "Context",
  "activeTeam": "string"
}
```

#### `RUNNER_BATCH_UPDATE`
Updates multiple runner positions simultaneously.
```json
{
  "activeCtx": "Context",
  "activeTeam": "string",
  "batterId": "string",
  "updates": [
    {
      "key": "string (EventKey)",
      "action": "string",
      "base": "number"
    }
  ]
}
```

#### `ADD_INNING`
Appends a new inning to the game.
```json
{}
```

#### `ADD_COLUMN`
Adds a new column to an inning.
```json
{
  "targetInning": "number",
  "team": "string (optional)"
}
```

#### `REMOVE_COLUMN`
Removes an empty column.
```json
{
  "colId": "string",
  "team": "string (optional)"
}
```

#### `SET_INNING_LEAD`
Sets which batter starts an inning/column.
```json
{
  "team": "string",
  "colId": "string",
  "rowId": "number"
}
```

#### `GAME_FINALIZE`
Marks the game as complete.
```json
{
  "finalScore": {
    "away": "number",
    "home": "number"
  },
  "stats": "Object (Aggregated Stats)",
  "timestamp": "number"
}
```

#### `RBI_EDIT`
Manually adjusts RBI credit.
```json
{
  "key": "string (EventKey)",
  "rbiCreditedTo": "string (PlayerID)"
}
```

#### `OUT_NUM_UPDATE`
Manually adjusts the out count for an event.
```json
{
  "key": "string (EventKey)",
  "outNum": "number"
}
```

#### `MANUAL_PATH_OVERRIDE`
Overrides the computed base paths for an event.
```json
{
  "key": "string (EventKey)",
  "data": {
    "paths": "array(4)",
    "pathInfo": "array(4)",
    "pId": "string (Optional Batter ID)"
  }
}
```

#### `UNDO`
Neutralizes a previous action.
```json
{
  "refId": "string (UUID of target action)"
}
```

#### `Context` Object
Defines the cursor position in the scoring grid.
```json
{
  "b": "number (Batter Index 0+)",
  "i": "number (Inning 1+)",
  "col": "string (Column ID)"
}
```

## 4. Shared Objects

### 4.1 Permissions
| Field | Type | Description |
| :--- | :--- | :--- |
| `public` | `string` | `'none'`, `'read'`. |
| `users` | `object` | `{ [email]: 'read' \| 'write' }`. |

### 4.2 TeamRoles
| Field | Type | Description |
| :--- | :--- | :--- |
| `admins` | `array<string>` | List of user IDs with full control. |
| `scorekeepers` | `array<string>` | List of user IDs with write access. |
| `spectators` | `array<string>` | List of user IDs with read access. |

### 4.3 PlayerObject (Lineup)
| Field | Type | Description |
| :--- | :--- | :--- |
| `id` | `string` | Unique player ID. |
| `name` | `string` | Name. |
| `number` | `string` | Uniform number. |
| `pos` | `string` | Current position. |

## 5. Backup File Format (JSONL)

Backups are stored in JSON Lines (JSONL) format to allow for incremental processing of large datasets. Each line is a standalone JSON object.

### 5.1 Header Record
The first line of the file.
```json
{"type": "header", "version": 1, "timestamp": 1700000000000}
```

### 5.2 Team Record
```json
{"type": "team", "id": "uuid", "data": "TeamObject"}
```

### 5.3 Game Record
```json
{
  "type": "game",
  "id": "uuid",
  "summary": {
    "away": "string",
    "home": "string",
    "date": "string",
    "event": "string",
    "location": "string",
    "status": "string"
  },
  "data": "GameObject"
}
```

