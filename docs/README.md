# Skorekeeper Engineering Design Document

Welcome to the technical documentation for the Skorekeeper Progressive Web Application (PWA). This document provides a comprehensive overview of the system architecture, core logic, and design principles that ensure data integrity, real-time collaboration, and a robust user experience.

## Table of Contents

1.  **[High-Level Overview](./OVERVIEW.md)**
    An introduction to the application's core purpose, key technologies, and high-level architectural components.

2.  **[State Management & Event Sourcing](./STATE-MANAGEMENT.md)**
    Details on the authoritative Action Log, deterministic state derivation (the Reducer), and the append-only Undo mechanism.

3.  **[Real-Time Synchronization & Offline Strategy](./SYNC-OFFLINE.md)**
    A deep dive into the WebSocket protocol, local-first persistence via IndexedDB, conflict resolution, and system resilience (batching, jitter, load shedding).

4.  **[Ball-in-Play (BiP) Design Specification](./BIP-DESIGN.md)**
    The authoritative reference for batted-ball logic, defensive sequences, and automated runner advancement rules.

5.  **[Authorization & Security](./AUTH-SECURITY.md)**
    Documentation of the team-centric permissions model, server-side enforcement, and secure communication protocols.

6.  **[Narrative Engine & Statistics](./NARRATIVE-STATS.md)**
    How the system translates raw actions into human-readable play-by-play stories and real-time player/team metrics.

7.  **[UI & Interaction Design](./UI-INTERACTION.md)**
    Specifications for the scoresheet grid, the adaptive Contextual Scoring Overlay (CSO), and responsive design principles.

8.  **[Visual System & UI Design](./VISUAL-UI.md)**
    Details on the color palette, typography, visual layering, and specific UI component behaviors.

9.  **[Definitions & Statistics Reference](./DEFINITIONS.md)**
    The authoritative glossary for all baseball/softball terminology, acronyms, and statistical formulas used in the app.

10. **[PDF Generation & Game Export](./EXPORT-PDF.md)**
    The design and orchestration logic for producing high-fidelity client-side PDF reports.

11. **[Backup & Restore Design](./BACKUPS.md)**
    Details the JSONL streaming architecture for efficiently exporting and importing large datasets of games and teams.

12. **[User Access Policy Design](./USER-ACCESS.md)**
    Documentation of the access control system, including global policies, user quotas, and Raft-replicated permissions.

---

*This documentation is intended for developers and architects working on the Skorekeeper project. It focuses on the "what" and "why" of the design, remaining implementation-independent to serve as a long-term reference.*