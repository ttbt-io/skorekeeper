# Backup & Restore Design

This document outlines the design and implementation for the Backup and Restore feature in Skorekeeper.

## Objective
Provide a robust way to export all application data (Teams and Games) to a single file and restore it selectively. The system must handle large datasets efficiently.

## Technical Strategy: JSONL Streaming
To avoid memory issues with potentially thousands of games, we use **JSON Lines (JSONL)**. Each line in the backup file is a standalone JSON object.
- **Backup:** We use `ReadableStream` and `TextEncoder` to stream the file directly to the user's browser for download.
- **Restore:** We use the `File.stream()` API combined with a line-by-line parser. This allows us to "scan" a 100MB backup file in seconds to show a manifest without loading the whole file into RAM.

## Component Design

### 1. `BackupManager` (`frontend/services/backupManager.js`)
A standalone service to handle the heavy lifting.
- `getBackupStream(options)`: Generates the JSONL stream.
- `scanBackupFile(file)`: Returns a list of headers {id, type, summary} for UI selection.
- `restoreBackup(file, selectedIds)`: Processes the stream and saves selected items to IndexedDB.

### 2. File Format (JSONL)
```json
{"type": "header", "version": 1, "timestamp": 1700000000000}
{"type": "team", "id": "uuid-1", "data": {...}}
{"type": "game", "id": "uuid-2", "summary": {"away": "A", "home": "B", "date": "..."}, "data": {...}}
```

### 3. UI Flow
- **Sidebar:** New "Backup / Restore" entry.
- **Backup Modal:** Selection for Games vs Teams, and a toggle for "Include Remote Data" (fetches items from server that aren't local yet).
- **Restore Modal:** 
    - Step 1: Upload file.
    - Step 2: Display manifest with checkboxes.
    - Step 3: Stream and save selected items.

## E2E Testing

The feature is validated by an automated test using `chromedp` and Docker (`tests/e2e/backup_restore_test.go`).

### 1. Infrastructure (`docker-compose-browser-tests.yaml`)
*   **Shared Volume:** A `shared-downloads` volume mounted to `/downloads` in both the `chrome` (headless browser) and `devtest` (test runner) containers.
*   **Chrome Config:** The headless browser is configured to save downloads to this shared path automatically.

### 2. Test Scenario
1.  **Setup:** Start server, login, and create a game ("BackupAway" vs "BackupHome").
2.  **Backup:** 
    *   Navigate to Backup modal.
    *   Trigger download.
    *   Verify file exists in `/downloads`.
3.  **Wipe:**
    *   Clear IndexedDB.
    *   Reload page to confirm data loss.
4.  **Restore:**
    *   Open Restore modal.
    *   Upload the file from `/downloads`.
    *   Select the game from the parsed manifest.
    *   Execute restore.
5.  **Verification:**
    *   Check Dashboard for the restored game.
    *   Verify metadata matches original.