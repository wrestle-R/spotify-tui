import Image from "next/image";
import Link from "next/link";
import { download, latest, repo } from "@/lib/releases";
import { MemoryStats } from "@/components/memory-stats";
export default function Home() {
  return (
    <main id="main">
      <section className="hero wrap">
        <div className="hero-copy" data-reveal>
          <div className="eyebrow">
            <span className="live-dot" /> {latest()} / LINUX + APPLE SILICON
          </div>
          <h1>
            All the music.
            <br />A little less
            <br />
            <em>everything else.</em>
          </h1>
          <p className="intro">
            Spotify was eating my RAM. I just wanted my playlists.
            <br className="desktop" /> So I made a terminal player.
          </p>
          <div className="actions">
            <a className="button primary" href={download()}>
              <span aria-hidden="true">↓</span> Download for Linux
            </a>
            <a className="button" href={download("darwin_arm64")}>
              <span aria-hidden="true">↓</span> Download for Mac
            </a>
            <Link href="/docs" className="text-link">
              Get it running <span aria-hidden="true">↗</span>
            </Link>
          </div>
          <p className="fine">Linux x86_64 / Apple Silicon Mac · Premium setup required</p>
        </div>
        <div className="hero-art" data-loop>
          <div className="orbit orbit-one" />
          <div className="orbit orbit-two" />
          <Image
            src="/hero-headphones.png"
            width={640}
            height={640}
            sizes="(max-width: 760px) 80vw, 45vw"
            alt="Floating headphones with emerald glass shells and charcoal cushions"
            preload
          />
        </div>
        <div className="hero-bottom">
          <span>GO + RUST</span>
          <span>KEYBOARD FIRST</span>
          <span>YOUR MUSIC, LOCALLY</span>
          <span>01 / A SMALLER PLAYER</span>
        </div>
      </section>
      <section className="showcase wrap" id="player">
        <div className="section-heading" data-reveal>
          <div>
            <span className="eyebrow">FAMILIAR MUSIC. DIFFERENT WINDOW.</span>
            <h2>
              Your library.
              <br />
              At your fingertips.
            </h2>
          </div>
          <p>
            Liked songs, playlists and a quick search.
            <br />
            Open a song. Hit Enter. Get back to whatever
            <br className="desktop" /> you were doing.
          </p>
        </div>
        <div className="terminal-shell" data-reveal>
          <div className="terminal-top">
            <div>
              <i />
              <i />
              <i />
            </div>
            <span>spotify-tui · demo</span>
            <span>⌘ &nbsp; ~/music</span>
          </div>
          <Image
            src="/tui-screenshot.png"
            alt="Actual Spotify TUI demo showing generic sample playlists and songs in a green and charcoal terminal"
            width={1280}
            height={760}
            sizes="(max-width: 1200px) 94vw, 1120px"
          />
        </div>
        <div className="caption">
          <span>
            Actual terminal capture. Generic demo music, no account data.
          </span>
          <Link href="/docs#keyboard">Meet the keyboard shortcuts ↗</Link>
        </div>
      </section>
      <MemoryStats />
      <section className="features wrap">
        <article data-reveal>
          <span className="feature-index">01 / THE POINT</span>
          <h3>
            Keep your RAM
            <br />
            for your work.
          </h3>
          <p>
            A terminal interface with a Rust playback helper. Built because the
            desktop app felt too heavy. The Linux comparison above includes the
            audio provider and terminal, because the whole setup is what matters.
          </p>
        </article>
        <article data-reveal>
          <span className="feature-index">02 / THE FEEL</span>
          <h3>
            Hands on keys.
            <br />
            Music in the tray.
          </h3>
          <p>
            Arrow keys move, Enter plays, Space pauses. Close the TUI and keep
            listening. The tray brings playback controls into your Linux panel.
          </p>
        </article>
        <article data-reveal>
          <span className="feature-index">03 / THE UPKEEP</span>
          <h3>
            A new release?
            <br />
            Ctrl-U. Done.
          </h3>
          <p>
            The player checks for stable updates when it opens. Ctrl-U
            downloads, verifies and installs a matching Go and Rust pair, then
            restarts the app.
          </p>
        </article>
      </section>
      <section className="setup-banner wrap" data-reveal>
        <span className="eyebrow">A LITTLE SETUP. THEN YOUR MUSIC.</span>
        <div>
          <h2>
            Bring your account.
            <br />
            <em>We’ll bring the terminal.</em>
          </h2>
          <Link href="/docs" className="button primary">
            Let’s set it up <span aria-hidden="true">↗</span>
          </Link>
        </div>
        <nav className="setup-mini" aria-label="Setup guide">
          {[["install", "Install"], ["sign-in", "Sign in"], ["audio", "Pair"], ["play", "Play"]].map(([id, label], index) =>
            <Link href={`/docs#${id}`} key={id}><span>{String(index + 1).padStart(2, "0")}</span> {label} <span aria-hidden="true">↗</span></Link>)}
        </nav>
        <p>
          You’ll need your own Spotify Developer app and personal Soloist key.
          Sign-in opens in your normal browser. No passwords on this website.
        </p>
      </section>
      <section className="honest wrap" data-reveal>
        <span>BEFORE YOU DOWNLOAD</span>
        <p>
          See the setup requirements, supported features and current limits.{" "}
          <Link href="/docs#limits">Read what works and what’s left ↗</Link>
        </p>
        <a href={`${repo}/releases/tag/${latest()}`}>Release notes ↗</a>
      </section>
    </main>
  );
}
