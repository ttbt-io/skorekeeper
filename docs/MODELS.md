# Skorekeeper Frontend Models

This document describes the JavaScript classes used to enforce the formal schema defined in `docs/SCHEMA.md`. These models ensure data consistency, provide default values, and handle schema versioning at the application level.

## 1. Design Principles

*   **Schema Enforcement:** All persistent data (Games, Teams, Actions) must be instantiated through these classes before being saved or processed.
*   **Validation:** Constructors perform basic type checking and ensure required fields are present.
*   **Versioning:** Models automatically set the `schemaVersion` to the current supported version (3) and will eventually handle logic for in-memory migrations.
*   **Immutability:** Methods that modify state return new instances or sanitized objects suitable for the Redux-style reducer.

## 2. Model Specifications

### 2.1 Player
Represents a player in a team roster or game lineup.
*   `id`: string (UUID)
*   `name`: string
*   `number`: string
*   `pos`: string

### 2.3 Team
Represents a persistent team entity.
*   `id`: string (UUID)
*   `schemaVersion`: number (default: 3)
*   `name`: string
*   `shortName`: string
*   `color`: string (Hex)
*   `roster`: Player[]
*   `ownerId`: string
*   `roles`: TeamRoles object
*   `updatedAt`: number (Timestamp)

### 2.4 Game
Represents the full state of a game.
*   `id`: string (UUID)
*   `schemaVersion`: number (default: 3)
*   `date`: string (ISO 8601)
*   `away`: string (Team Name)
*   `home`: string (Team Name)
*   `status`: 'ongoing' | 'final'
*   `roster`: { away: RosterSlot[], home: RosterSlot[] }
*   `subs`: { away: Player[], home: Player[] }
*   `actionLog`: Action[]
*   *(Other fields as defined in SCHEMA.md)*

### 2.5 Action
Represents a discrete event in the game's history.
*   `id`: string (UUID)
*   `type`: string (ActionType)
*   `payload`: object
*   `schemaVersion`: number (default: 3)
*   `timestamp`: number
*   `userId`: string

## 3. Usage Pattern

```javascript
import { Team } from './models/Team.js';
import { Player } from './models/Player.js';

// Creating a new team from UI input
const newTeam = new Team({
    name: 'Rockets',
    roster: [
        new Player({ name: 'Alice', number: '10' })
    ]
});

// The model ensures schemaVersion is 3 and generates IDs if missing.
console.log(newTeam.schemaVersion); // 3
```
