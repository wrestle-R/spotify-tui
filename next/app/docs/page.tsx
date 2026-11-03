import type { Metadata } from "next";
import { Code } from "@/components/code";
import { DocNav } from "@/components/doc-nav";
import { InstallPlayer, InstallSoloist, SetupPlatform } from "@/components/setup-platform";
import { download, latest, repo } from "@/lib/releases";
export const metadata: Metadata = { title: "Install, sign in, pair, play" };
const sections = [
  ["before", "Before you start"],
  ["install", "01  Install"],
  ["sign-in", "02  Sign in"],
  ["audio", "03  Pair"],
  ["play", "04  Play"],
  ["keyboard", "Keyboard & tray"],
  ["updates", "Updates"],
  ["troubleshooting", "Need a hand?"],
  ["limits", "Current limits"],
];
export default function Docs() {
  return (<SetupPlatform>
    <main id="main" className="doc-layout wrap">
      <DocNav sections={sections} />
      <article className="doc-content">
        <div className="eyebrow">GET STARTED / {latest()}</div>
        <h1>A few steps.<br /><em>Then hit play.</em></h1>
        <p className="lede">Install the player, connect your library, pair the audio engine, then play your first song. Here’s the whole route.</p>
        <nav className="setup-route" aria-label="Four setup steps">
          {[["install", "01", "Install", "Get the player"], ["sign-in", "02", "Sign in", "Load your library"],
            ["audio", "03", "Pair", "Connect the audio"], ["play", "04", "Play", "Your first song"]].map(([id, number, title, detail]) =>
            <a href={`#${id}`} key={id}><span>{number}</span><strong>{title}</strong><small>{detail}</small><i aria-hidden="true">↗</i></a>)}
        </nav>
        <section id="before">
          <span className="step">BEFORE YOU START</span>
          <h2>Two connections. One account.</h2>
          <p>You’ll need <strong>Spotify Premium</strong>, a normal browser, and either Linux x86_64 (glibc 2.39+) or an Apple Silicon Mac (macOS 15+). Python 3.9+ is needed for installation.</p>
          <dl className="setup-connections">
            <div><dt>Your library</dt><dd>A Spotify Developer app’s public client ID lets browser sign-in load playlists, liked songs and search.</dd></div>
            <div><dt>Your audio</dt><dd>Spotify’s separate Soloist engine and your private personal Soloist key play songs on this computer. It needs one-time device pairing.</dd></div>
          </dl>
          <p>Use the same Spotify account for both. Credentials stay in your desktop keyring (Linux) or Keychain (Mac). <strong>This website has no sign-in form and never asks for your password or keys.</strong></p>
          <p className="setup-note"><a href="#limits">Current limits →</a></p>
        </section>
        <section id="install">
          <span className="step">01 / INSTALL</span>
          <h2>Start with the player.</h2>
          <InstallPlayer version={latest()} linux={download()} mac={download("darwin_arm64")} />
          <div className="setup-checkpoint"><strong>Ready for sign-in?</strong><p>Both binaries report the same version, and the demo opens. Close the demo before continuing.</p><a href="#sign-in">Next: sign in <span aria-hidden="true">→</span></a></div>
        </section>
        <section id="sign-in">
          <span className="step">02 / SIGN IN</span>
          <h2>Let the player see your library.</h2>
          <h3>1. Create or select your Developer app</h3>
          <p>Open the <a href="https://developer.spotify.com/dashboard">Spotify Developer dashboard ↗</a> in your usual browser. Create or select your app, enable Web API access, and register this exact Redirect URI:</p>
          <Code label="Redirect URI">http://127.0.0.1:43821/callback</Code>
          <p>Save the dashboard changes. If its User Management page requires an allowlist, add the Spotify account you’ll sign in with. Copy the public <strong>client ID</strong> from the app’s settings. You don’t need its client secret.</p>
          <h3>2. Save the client ID, then sign in</h3>
          <p>Replace <code>YOUR_PUBLIC_CLIENT_ID</code> below with that ID. Don’t use the app name or client secret.</p>
          <Code label="Connect your library">{`~/.local/bin/spotify-tui setup --client-id YOUR_PUBLIC_CLIENT_ID &&
~/.local/bin/spotify-tui login`}</Code>
          <p>Keep the terminal command running. Your normal browser opens Spotify’s consent page. Approve access and let it return to the local callback on <strong>this computer</strong>. Login should finish within five minutes.</p>
          <p className="setup-note">You’re done when the terminal says: <samp>Signed in for library/search. Soloist pairing is separate.</samp> Audio comes next.</p>
          <details><summary>Browser didn’t open, or verification failed?</summary>
            <p>Use a normal browser outside an embedded app. Run the command below, then open the printed URL on this same computer. If five minutes have passed, start a fresh login command for a new URL.</p>
            <Code label="Open sign-in manually">~/.local/bin/spotify-tui login --no-browser</Code>
            <p>If Spotify rejects the callback, check <code>http://127.0.0.1:43821/callback</code> matches the saved dashboard URI exactly, including the port and path. Don’t replace <code>127.0.0.1</code> with <code>localhost</code>.</p>
          </details>
          <div className="setup-checkpoint"><strong>Library connected.</strong><p>Browser login has finished successfully in your terminal. Keep that app’s client ID; the next step adds audio to the same configuration.</p><a href="#audio">Next: pair the audio <span aria-hidden="true">→</span></a></div>
        </section>
        <section id="audio">
          <span className="step">03 / PAIR</span>
          <h2>Give your music somewhere to play.</h2>
          <h3>1. Download the official audio engine</h3>
          <InstallSoloist />
          <h3>2. Connect Soloist to this player</h3>
          <p>This adds the Soloist path to the configuration you saved in step 2. It keeps your client ID.</p>
          <Code label="Set audio engine path">~/.local/bin/spotify-tui setup --soloist-path "$HOME/.local/lib/spotify-soloist/soloist"</Code>
          <h3>3. Store your personal key</h3>
          <p>Generate your <a href="https://developer.spotify.com/dashboard/soloist">personal Soloist key ↗</a> while signed in to your Premium account. Run the next command and paste the key into its <strong>hidden terminal prompt</strong>, then press Enter. Nothing is shown while you type.</p>
          <Code label="Store audio key">~/.local/bin/spotify-tui provider-key</Code>
          <p>Keep the key private. Don’t put it in a command argument, screenshot, chat or this website. The public client ID from step 2 and this private key are different things.</p>
          <h3>4. Start the engine and pair once</h3>
          <Code label="Start local audio">~/.local/bin/spotify-tui connect</Code>
          <ol>
            <li>Open the official Spotify app on this computer or another device on the same local network, signed in to the same account.</li>
            <li>Open its device picker and select <strong>Spotify TUI Local</strong>. Keep the helper running while you do this.</li>
            <li>Back in the terminal, check the connection:</li>
          </ol>
          <Code label="Check pairing">~/.local/bin/spotify-tui doctor</Code>
          <p>Look for <code>Provider connected: true paired: true</code>. A fresh engine may take a moment to connect; run doctor again after it settles.</p>
          <details><summary>Spotify TUI Local isn’t in the picker?</summary>
            <p>Confirm Soloist printed its version, the key was stored, and <code>connect</code> is still running its helper. Use the same account and a shared Wi-Fi/local network. Phone USB tethering and mobile data can make discovery unreliable; try the official client on this computer for initial pairing. See <a href="#troubleshooting">troubleshooting</a> before deleting any credentials.</p>
          </details>
          <div className="setup-checkpoint"><strong>Ready to play?</strong><p>Doctor shows connected and paired. Keep the app’s data directory so this pairing can be restored after a restart.</p><a href="#play">Next: your first song <span aria-hidden="true">→</span></a></div>
        </section>
        <section id="play">
          <span className="step">04 / PLAY</span>
          <h2>Pick a song. Hit Enter.</h2>
          <p>You can now close the official Spotify app. Open your installed terminal player:</p>
          <Code label="Open your player">~/.local/bin/spotify-tui</Code>
          <ol>
            <li>Use <kbd>↑</kbd> / <kbd>↓</kbd> to choose Liked Songs or a playlist.</li>
            <li>Press <kbd>Enter</kbd> to open it. Choose a song, then press <kbd>Enter</kbd> again to play.</li>
            <li>Press <kbd>Space</kbd> to pause or resume. Press <kbd>/</kbd> to search and <kbd>Esc</kbd> to return.</li>
          </ol>
          <p><kbd>q</kbd> closes the TUI and keeps music playing. <kbd>Q</kbd> stops audio and quits. On Linux, the tray appears if your panel supports it. Mac runs without a tray.</p>
          <div className="setup-checkpoint"><strong>You’re set when you can hear it.</strong><p>The TUI’s Playing/Paused label should match the actual audio with the official player closed. If the saved session stalls later, press <kbd>c</kbd> to reconnect.</p><a href="#keyboard">Meet the shortcuts <span aria-hidden="true">→</span></a></div>
        </section>
        <section id="keyboard">
          <span className="step">REFERENCE / MAKE YOURSELF AT HOME</span>
          <h2>Keys you’ll actually remember</h2>
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>Key</th>
                  <th>Action</th>
                </tr>
              </thead>
              <tbody>
                {[
                  ["↑ ↓ / j k", "Move in the focused pane"],
                  ["Enter / →", "Open collection; Enter on a song plays"],
                  ["Esc / ←", "Back to library"],
                  ["Tab / Shift-Tab", "Switch panes"],
                  ["/", "Search; Enter submits, Esc cancels"],
                  ["Space", "Pause / resume"],
                  ["n / p", "Next / previous song"],
                  ["[ / ]", "Seek five seconds"],
                  ["+ / −", "Change volume"],
                  ["c / L", "Reconnect / browser sign-in"],
                  ["Ctrl-U", "Check, install stable update and restart"],
                  ["q / Q", "Quit, keep music / stop music and quit"],
                  ["?", "Full keyboard help"],
                ].map(([key, action]) => (
                  <tr key={key}>
                    <td>
                      <kbd>{key}</kbd>
                    </td>
                    <td>{action}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <h3>The tray</h3>
          <p>
            The player starts a small tray companion when your panel has a
            StatusNotifier host. Left-click opens the TUI; the menu offers
            pause/resume, next, previous, and stop. <kbd>q</kbd> leaves music
            and tray running; <kbd>Q</kbd> stops both.
          </p>
          <p>
            On Hyprland, enable the tray component in Quickshell or your panel.
            GNOME may need an AppIndicator extension. The app doesn’t install or
            modify your panel configuration. Run <code>spotify-tui doctor</code>{" "}
            to check the host, or <code>spotify-tui --no-tray</code> to skip the
            tray. Opening from the tray requires Kitty, Foot, Alacritty,
            Konsole, GNOME Terminal or xterm.
          </p>
        </section>
        <section id="updates">
          <span className="step">UPDATES / KEEP IT CURRENT</span>
          <h2>Stay in the terminal</h2>
          <p>
            The app checks the latest stable release in the background when
            opened. When an update is available, press <kbd>Ctrl-U</kbd>. It
            downloads the archive, verifies SHA-256 and both executable
            versions, switches the installed pair together, then restarts.
            Existing build directories stay available for rollback.
          </p>
          <Code>{`~/.local/bin/spotify-tui check-updates
~/.local/bin/spotify-tui update`}</Code>
          <p>
            In-app updates need the managed installation from{" "}
            <code>install.sh</code>. They don’t overwrite source builds. A
            failed download or verification leaves the installed version intact.
            No automatic installation happens without Ctrl-U or the update
            command.
          </p>
        </section>
        <section id="troubleshooting">
          <h2>If something gets stuck</h2>
          <details>
            <summary>The device isn’t in Spotify’s picker</summary>
            <p>
              Run <code>spotify-tui connect</code>, then{" "}
              <code>spotify-tui doctor</code>. Both devices need local-network
              reachability. Discovery across phone USB tethering can be
              unreliable; try Spotify on this computer for the initial pairing.
              Don’t delete credentials or provider data first.
            </p>
          </details>
          <details>
            <summary>No library or search results</summary>
            <p>
              Press L or run <code>spotify-tui login</code> again. Check your
              client ID, redirect URI and allowed users in the developer
              dashboard. Spotify can restrict reading followed or curated
              playlists.
            </p>
          </details>
          <details>
            <summary>No keyring available</summary>
            <p>
              On Linux, start a desktop secret service such as GNOME Keyring or KWallet
              and unlock it. On Mac, resolve the normal Keychain consent prompt. There is an explicit plaintext fallback described
              in the repository setup guide; it is not enabled by default.
            </p>
          </details>
          <details>
            <summary>Need diagnostic commands?</summary>
            <Code>{`~/.local/bin/spotify-tui doctor
~/.local/bin/spotify-tui stop
~/.local/bin/spotify-tui tray-stop`}</Code>
            <p>
              Doctor reports status without printing your keys. Read{" "}
              <a href={`${repo}/blob/main/docs/SETUP.md`}>
                the full setup guide ↗
              </a>{" "}
              for build instructions and integration caveats.
            </p>
          </details>
        </section>
        <section id="limits">
          <h2>What’s still missing</h2>

          <p>
            Search currently returns ten results, and long-session memory
            testing is still pending. Explore, search history, mouse controls,
            collection sequencing and Linux MPRIS media controls are available
            from v0.5.0. Hold i in the terminal for shortcuts.
            Connectivity and Soloist session restoration can occasionally
            stall. Press c to reconnect.
          </p>
          <p>
            This project does not promise ad-free playback for Free accounts.
            Each installation needs its own valid provider setup. See{" "}
            <a href={`${repo}/blob/main/docs/IMPLEMENTATION.md`}>
              verification details ↗
            </a>
            .
          </p>
        </section>
      </article>
    </main>
    </SetupPlatform>
  );
}
