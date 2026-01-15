# Skorekeeper TODO List

## Performance Optimizations

- [ ] **Optimize `ListAllGameMetadata` in `backend/gamestore.go`**:
    The current implementation is inefficient because it calls `gs.LoadGame(gameId)`, which loads the entire game file from disk (including the potentially large `actionLog`) just to extract a few metadata fields. This causes significant I/O and performance degradation during registry rebuilds or for users with many games.
    
    **Suggested Approaches:**
    1. Store a separate, small metadata file (e.g., `gameId.meta.json`) alongside the main game file.
    2. Structure the game JSON so that metadata is at the top, and implement a way to read only the beginning of the file to parse the metadata without reading the entire `actionLog`.

## Security

- [ ] **Refactor Static Inline Styles**:
    Replace hardcoded `style` attributes in `frontend/index.html` (e.g., `z-index: 70`) with equivalent Tailwind CSS classes (e.g., `z-[70]`) to reduce reliance on inline styles.

- [ ] **Tighten Content Security Policy (CSP)**:
    Update `securityMiddleware` in `backend/server.go` to distinguish between style blocks and attributes.
    **Proposed Policy:** `style-src 'self'; style-src-attr 'self' 'unsafe-inline';`
    This will block malicious `<style>` tag injection while still allowing dynamic positioning and user-defined colors.
