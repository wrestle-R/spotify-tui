# Source ledger

Reviewed 28 October 2026. Sources describe upstream capabilities; this project has not run account or playback tests. Recheck live documentation before implementation because Spotify access rules and binaries can change.

## Spotify playback

3. [Soloist authentication](https://developer.spotify.com/documentation/soloist/concepts/authentication): personal API key and stored Connect session are separate.
4. [Soloist CLI](https://developer.spotify.com/documentation/soloist/reference/command-line): audio routing, cache, pairing, diagnostics, and exit codes. Cache settings are not a download API.
5. [Soloist WebSocket API](https://developer.spotify.com/documentation/soloist/reference/websocket-api): local integration protocol and missing built-in authentication/browser security protections.
6. [Soloist downloads and updates](https://developer.spotify.com/documentation/soloist/reference/downloads-and-updates): obtaining the player executable, not music downloads.

## Community playback

These projects are references only. No upstream code has been copied or dependencies installed in this planning session. Review licenses before reuse.

## Account, library, and API

13. [Authorization Code with PKCE](https://developer.spotify.com/documentation/web-api/tutorials/code-pkce-flow): authorization route for a desktop client without a bundled client secret.
14. [Redirect URI rules](https://developer.spotify.com/documentation/web-api/concepts/redirect_uri): explicit loopback address and callback configuration.
15. [Get current user's profile](https://developer.spotify.com/documentation/web-api/reference/get-current-users-profile): account identity endpoint.
16. [Get saved tracks](https://developer.spotify.com/documentation/web-api/reference/get-users-saved-tracks): liked-song collection.
17. [Get current user's playlists](https://developer.spotify.com/documentation/web-api/reference/get-list-users-playlists): playlist listing is separate from access to its contents.
18. [Current playlist-items endpoint](https://developer.spotify.com/documentation/web-api/reference/get-playlists-items): current `/items` route; ownership/collaboration restriction. The old [playlist-tracks route](https://developer.spotify.com/documentation/web-api/reference/get-playlists-tracks) is marked deprecated.
19. [Search](https://developer.spotify.com/documentation/web-api/reference/search): catalog search reference. Verify mode-specific limits during implementation.
20. [February 2026 changes](https://developer.spotify.com/documentation/web-api/references/changes/february-2026): endpoint/search changes; must be read with later updates.
21. [March 2026 changes](https://developer.spotify.com/documentation/web-api/references/changes/march-2026): reversals of selected earlier field changes.
22. [May 2026 changes](https://developer.spotify.com/documentation/web-api/references/changes/may-2026): account identifier update relevant to durable account namespacing.
23. [July 2026 changes](https://developer.spotify.com/documentation/web-api/references/changes/july-2026): higher app limit and account-level development quota accounting. Supersedes older one-client-ID guidance.
24. [Rate limits](https://developer.spotify.com/documentation/web-api/concepts/rate-limits): backoff and request-budget behavior.
25. [Quota modes](https://developer.spotify.com/documentation/web-api/concepts/quota-modes): personal development access requirements. Confirm actual dashboard status rather than inferring it from old tutorials.

## Go and Rust choices

26. [Bubble Tea](https://github.com/charmbracelet/bubbletea): Go event-driven terminal framework.
27. [Bubbles](https://github.com/charmbracelet/bubbles): optional terminal input/list components.
28. [Lip Gloss](https://github.com/charmbracelet/lipgloss): terminal styling/layout utilities.
29. [Tokio](https://docs.rs/tokio/latest/tokio/): asynchronous Rust runtime for IPC/process/provider work.

Exact package versions are intentionally not frozen before a real build and compatibility review. The implementation must record and lock its chosen versions.

## Interpretation limits

No account login, private API access, encrypted-store inspection, license bypass, or audio downloading occurred during research.
