mod sequence;
use fs2::FileExt;
use futures_util::{SinkExt, StreamExt};
use sequence::Sequence;
use serde::Deserialize;
use serde_json::{json, Value};
use std::{
    collections::HashMap,
    fs,
    io::Read,
    os::unix::fs::PermissionsExt,
    path::PathBuf,
    sync::Arc,
    time::{Duration, Instant},
};
use tokio::{
    io::{AsyncBufReadExt, AsyncWriteExt, BufReader},
    net::{UnixListener, UnixStream},
    sync::{mpsc, oneshot, watch, Mutex},
    time,
};
use tokio_tungstenite::{connect_async, tungstenite::Message};

const MAX_FRAME: usize = 256 * 1024;
#[derive(Deserialize)]
struct Request {
    version: u32,
    id: String,
    method: String,
    #[serde(default)]
    params: Value,
}
struct Command {
    value: Value,
    reply: oneshot::Sender<Result<(), String>>,
}
struct Pending {
    command: Command,
    started: Instant,
    accepted: bool,
    expected_status: Option<&'static str>,
    expected_uri: Option<String>,
    confirmed: bool,
    sequence: Option<Option<Sequence>>,
}
impl Pending {
    fn observe(&mut self, state: &Value) {
        self.confirmed = self
            .expected_status
            .is_none_or(|status| state["status"] == status)
            && self
                .expected_uri
                .as_ref()
                .is_none_or(|uri| state["uri"] == *uri);
    }
    fn complete(&self) -> bool {
        self.accepted && self.confirmed
    }
}
struct Shared {
    state: Mutex<Value>,
    commands: mpsc::Sender<Command>,
    stop: watch::Sender<bool>,
}

fn initial_state() -> Value {
    json!({"revision":0,"instance":format!("{}-{}",std::process::id(),now_ms()),"connected":false,"logged_in":false,"status":"stopped","title":"Nothing playing","artist":"","uri":"","position_ms":0,"duration_ms":0,"volume":50,"shuffle":false,"repeat":"off","error":"Soloist is not connected"})
}

fn clear_endpoint_files(data: &std::path::Path) -> std::io::Result<()> {
    for name in ["ws.addr", "ws.port"] {
        match fs::remove_file(data.join(name)) {
            Ok(()) => {}
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => {}
            Err(e) => return Err(e),
        }
    }
    Ok(())
}

async fn stop_provider(child: &mut tokio::process::Child) {
    if let Some(pid) = child.id() {
        unsafe {
            libc::kill(pid as libc::pid_t, libc::SIGTERM);
        }
    }
    if time::timeout(Duration::from_secs(3), child.wait())
        .await
        .is_err()
    {
        let _ = child.kill().await;
        let _ = child.wait().await;
    }
}

fn command(method: &str, params: &Value) -> Result<Value, String> {
    let mut v = json!({"type":"command"});
    let name = match method {
        "play_sequence" => return Ok(Sequence::parse(params)?.play()),
        "play_track" | "play_collection" => {
            let uri = params["uri"].as_str().ok_or("Missing Spotify URI")?;
            if !["spotify:track:", "spotify:playlist:", "spotify:album:"]
                .iter()
                .any(|p| uri.starts_with(p))
                || uri.len() > 256
                || uri.chars().any(char::is_control)
            {
                return Err("Invalid playable Spotify URI".into());
            }
            v["uri"] = json!(uri);
            "play"
        }
        "toggle_pause" => "_toggle_pause",
        "resume" => "play",
        "pause" => "pause",
        "next" => "skip_next",
        "previous" => "skip_prev",
        "activate" => "activate",
        "seek" => {
            let p = params["position_ms"]
                .as_u64()
                .ok_or("Invalid seek position")?;
            if p > 86_400_000 {
                return Err("Seek position is too large".into());
            }
            v["position_ms"] = json!(p);
            "seek"
        }
        "set_volume" => {
            let n = params["volume"]
                .as_u64()
                .filter(|n| *n <= 100)
                .ok_or("Volume must be 0 to 100")?;
            v["volume"] = json!(n);
            "set_volume"
        }
        "set_shuffle" => {
            v["enabled"] = json!(params["enabled"]
                .as_bool()
                .ok_or("Shuffle needs a boolean")?);
            "set_shuffle"
        }
        "add_to_queue" => {
            let uri = params["uri"]
                .as_str()
                .filter(|s| {
                    s.starts_with("spotify:track:")
                        && s.len() < 256
                        && !s.chars().any(char::is_control)
                })
                .ok_or("Queue needs a track URI")?;
            v["uri"] = json!(uri);
            "add_to_queue"
        }
        "repeat_track" | "repeat_context" => {
            v["enabled"] = json!(params["enabled"]
                .as_bool()
                .ok_or("Repeat needs a boolean")?);
            if method == "repeat_track" {
                "set_repeat_track"
            } else {
                "set_repeat_context"
            }
        }
        m if m.starts_with("download") => {
            return Err(
                "UnsupportedCapability: This operation is not available through this provider"
                    .into(),
            )
        }
        _ => return Err("Unknown method".into()),
    };
    v["command"] = json!(name);
    Ok(v)
}

fn apply_event(state: &mut Value, event: &Value) {
    let revision = state["revision"].as_u64().unwrap_or(0);
    state["revision"] = json!(revision + 1);
    match event["type"].as_str().unwrap_or("") {
        "auth_state" => {
            state["logged_in"] = event["logged_in"].clone();
            state["error"] = if event["logged_in"] == true {
                json!("")
            } else {
                json!("Pair Soloist once using an official Spotify client on the same network")
            };
        }
        "playback_state" => {
            state["status"] = event["status"].clone();
            update_track(state, &event["item"]);
            update_position(state, &event["position"]);
            if !event["volume"].is_null() {
                state["volume"] = event["volume"].clone()
            };
            update_options(state, &event["options"]);
        }
        "track_changed" => update_track(state, &event["item"]),
        "playback_changed" => {
            if state["status"] == "playing" {
                let elapsed = now_ms()
                    .saturating_sub(state["position_updated_at_ms"].as_u64().unwrap_or(now_ms()));
                state["position_ms"] = json!(state["position_ms"].as_u64().unwrap_or(0) + elapsed);
            }
            state["status"] = event["status"].clone();
            state["position_updated_at_ms"] = json!(now_ms());
        }
        "position_sync" => update_position(state, &event["position"]),
        "volume_changed" => state["volume"] = event["volume"].clone(),
        "options_changed" => update_options(state, &event["options"]),
        _ => {}
    }
}
fn position_ms(state: &Value) -> u64 {
    let position = state["position_ms"].as_u64().unwrap_or(0);
    if state["status"] == "playing" {
        position.saturating_add(
            now_ms().saturating_sub(
                state["position_updated_at_ms"]
                    .as_u64()
                    .unwrap_or_else(now_ms),
            ),
        )
    } else {
        position
    }
}
fn now_ms() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis() as u64
}
fn update_track(state: &mut Value, item: &Value) {
    let uri = item["uri"].as_str().unwrap_or_default();
    let same = state["uri"] == uri && !uri.is_empty();
    if !same {
        for key in ["title", "artist", "album", "art_url"] {
            state[key] = json!("");
        }
        state["duration_ms"] = json!(0);
    }
    state["uri"] = json!(uri);
    if let Some(name) = item["decorations"]["identity"]["name"].as_str() {
        state["title"] = json!(name);
    }
    if let Some(duration) = item["decorations"]["playback"]["duration_ms"].as_u64() {
        state["duration_ms"] = json!(duration);
    }
    if let Some(album) =
        item["decorations"]["parent"]["entity"]["decorations"]["identity"]["name"].as_str()
    {
        state["album"] = json!(album);
    }
    if let Some(images) = item["decorations"]["visual_identity"]["cover"].as_array() {
        let art = images
            .iter()
            .filter_map(|image| {
                image["url"]
                    .as_str()
                    .filter(|value| valid_art_url(value))
                    .map(|url| {
                        let rank = match image["size"].as_str() {
                            Some("xlarge") => 4,
                            Some("large") => 3,
                            Some("default") => 2,
                            _ => 1,
                        };
                        (rank, url)
                    })
            })
            .max_by_key(|(rank, _)| *rank)
            .map(|(_, url)| url)
            .unwrap_or_default();
        state["art_url"] = json!(art);
    }
    if let Some(creators) = item["decorations"]["creators"].as_array() {
        state["artist"] = json!(creators
            .iter()
            .filter_map(|e| e["entity"]["decorations"]["identity"]["name"].as_str())
            .collect::<Vec<_>>()
            .join(", "));
    }
}
fn valid_art_url(value: &str) -> bool {
    url::Url::parse(value).is_ok_and(|url| {
        url.scheme() == "https"
            && matches!(
                url.host_str(),
                Some("i.scdn.co" | "image-cdn-ak.spotifycdn.com" | "image-cdn-fa.spotifycdn.com")
            )
            && !value
                .chars()
                .any(|c| c.is_control() || matches!(c, '\'' | '"' | '`' | '\\'))
            && url.username().is_empty()
            && url.password().is_none()
    })
}
fn update_position(state: &mut Value, pos: &Value) {
    if !pos["position_ms"].is_null() {
        state["position_ms"] = pos["position_ms"].clone();
        state["position_updated_at_ms"] = json!(now_ms());
    }
}
fn update_options(state: &mut Value, options: &Value) {
    if let Some(shuffle) = options["shuffle"].as_bool() {
        state["shuffle"] = json!(shuffle)
    };
    let repeat = options["repeat"]
        .as_str()
        .filter(|v| matches!(*v, "off" | "context" | "track"));
    state["repeat"] = json!(if let Some(repeat) = repeat {
        repeat
    } else if options["repeat_track"] == true {
        "track"
    } else if options["repeat_context"] == true {
        "context"
    } else {
        "off"
    });
}

fn local_url(value: &str) -> Result<(), String> {
    let parsed = url::Url::parse(value).map_err(|_| "Invalid provider URL")?;
    if parsed.scheme() != "ws"
        || !matches!(parsed.host_str(), Some("127.0.0.1" | "[::1]" | "::1"))
        || parsed.port().is_none()
        || !parsed.username().is_empty()
        || parsed.password().is_some()
    {
        return Err("Provider URL must be an explicit loopback ws:// address and port".into());
    };
    Ok(())
}

async fn bridge(
    address: String,
    shared: Arc<Shared>,
    mut rx: mpsc::Receiver<Command>,
    mut stop: watch::Receiver<bool>,
) {
    let mut sequence: Option<Sequence> = None;
    loop {
        let connection = time::timeout(Duration::from_secs(5), connect_async(&address)).await;
        if let Ok(Ok((mut ws, _))) = connection {
            {
                let mut s = shared.state.lock().await;
                s["connected"] = json!(true);
                s["error"] = json!("");
            }
            let mut pending: Option<Pending> = None;
            let mut automatic: Option<Command> = None;
            let mut ending: Option<Instant> = None;
            let mut tick = time::interval(Duration::from_millis(100));
            let mut last_metadata_query = Instant::now();
            loop {
                tokio::select! {
                    _=stop.changed()=>{let _=ws.close(None).await;return},
                    Some(mut cmd)=async { if let Ok(cmd)=rx.try_recv(){Some(cmd)}else if let Some(cmd)=automatic.take(){Some(cmd)}else{rx.recv().await} }, if pending.is_none()=>{
                        if shared.state.lock().await["logged_in"]!=true {let _=cmd.reply.send(Err("Soloist is not paired; select Spotify TUI Local once in the official Spotify device list".into()));continue}
                        if cmd.value["_automatic"]!=true {automatic=None;if matches!(cmd.value["command"].as_str(),Some("play"|"seek"|"pause"|"_toggle_pause"|"skip_next"|"skip_prev")){ending=None;}}
                        let mut next_sequence = None;
                        if !cmd.value["_sequence"].is_null() {
                            match Sequence::parse(&json!({"uris":cmd.value["_sequence"],"index":cmd.value["_index"]})) {
                                Ok(s)=>next_sequence=Some(Some(s)),
                                Err(e)=>{let _=cmd.reply.send(Err(e));continue}
                            }
                        } else if cmd.value["command"]=="play" && !cmd.value["uri"].is_null() {
                            next_sequence=Some(None);
                        } else if matches!(cmd.value["command"].as_str(),Some("skip_next"|"skip_prev")) {
                            if let Some(current)=sequence.as_ref() {
                                let state=shared.state.lock().await;
                                let forward=cmd.value["command"]=="skip_next";
                                let target=if !forward && position_ms(&state)>3000 {Some(current.clone())} else {current.step(forward,&state)};
                                if let Some(target)=target {cmd.value=target.play();next_sequence=Some(Some(target));} else {next_sequence=Some(None);}
                            }
                        } else if cmd.value["command"]=="add_to_queue" {
                            if let Some(current)=sequence.as_mut() {
                                if current.uris.len()>=5000 {let _=cmd.reply.send(Err("Sequence is full".into()));continue}
                                current.uris.push(cmd.value["uri"].as_str().unwrap_or_default().to_owned());
                                shared.state.lock().await["sequence"]=current.snapshot();
                                let _=cmd.reply.send(Ok(()));continue;
                            }
                        }
                        // Internal sequence fields never cross the provider boundary.
                        if let Some(obj)=cmd.value.as_object_mut(){obj.remove("_sequence");obj.remove("_index");obj.remove("_automatic");}
                        if cmd.value["command"]=="_toggle_pause" {
                            let state=shared.state.lock().await;
                            cmd.value["command"]=json!(if state["status"]=="playing" || state["status"]=="buffering" {"pause"}else{"play"});
                        }
                        let expected_status=match cmd.value["command"].as_str(){Some("pause")=>Some("paused"),Some("play")=>Some("playing"),_=>None};
                        let expected_uri=cmd.value["uri"].as_str().filter(|v|cmd.value["command"]=="play" && v.starts_with("spotify:track:")).map(String::from);
                        if ws.send(Message::Text(cmd.value.to_string().into())).await.is_err(){let _=cmd.reply.send(Err("Provider disconnected".into()));break}
                        pending=Some(Pending{command:cmd,started:Instant::now(),accepted:false,expected_status,expected_uri,confirmed:expected_status.is_none(),sequence:next_sequence});
                    },
                    item=ws.next()=>{
                        match item {
                            Some(Ok(Message::Text(text)))=>{
                                if text.len()>MAX_FRAME{break}
                                if let Ok(event)=serde_json::from_str::<Value>(&text){
                                    let mut state=shared.state.lock().await;
                                    let before=state.clone();
                                    apply_event(&mut state,&event);
                                    if pending.is_none() && automatic.is_none() {
                                        if let Some(current)=sequence.as_ref() {
                                            let duration=before["duration_ms"].as_u64().unwrap_or(0);
                                            if before["uri"]==current.uri() && before["status"]=="playing" && duration>0 && position_ms(&before)>=duration.saturating_sub(2000) {ending=Some(Instant::now());}
                                            let boundary=ending.is_some_and(|t|t.elapsed()<Duration::from_secs(3)) && before["repeat"]!="track" && (state["uri"]!=before["uri"] || state["status"]=="stopped");
                                            if current.naturally_finished(&before,&state) || boundary {
                                                if let Some(next)=current.step(true,&before) {
                                                    let(tx,_)=oneshot::channel();let mut value=next.play();value["_automatic"]=json!(true);automatic=Some(Command{value,reply:tx});
                                                } else {sequence=None;}
                                            } else if state["uri"]!=before["uri"] && state["uri"]!=current.uri() {
                                                // Respect playback changes from another Spotify controller.
                                                sequence=None;
                                            }
                                        }
                                    }
                                    if let Some(p)=pending.as_mut() {
                                        if matches!(event["type"].as_str(),Some("playback_state"|"playback_changed"|"track_changed")) {p.observe(&state);}
                                        if event["type"]=="command_result" && p.command.value["command"]==event["command"] {
                                            p.accepted=true;
                                            if p.expected_status.is_some() {let _=ws.send(Message::Text(json!({"type":"command","command":"get_state"}).to_string().into())).await;}
                                        }
                                    }
                                    drop(state);
                                    if event["type"]=="auth_state"&&event["logged_in"]==true {let _=ws.send(Message::Text(json!({"type":"command","command":"get_state"}).to_string().into())).await;}
                                    if event["type"]=="error" {
                                        if let Some(p)=pending.take(){let _=p.command.reply.send(Err("Provider rejected command; check pairing, available actions and track eligibility".into()));}
                                    }else if pending.as_ref().is_some_and(Pending::complete) {
                                        if let Some(p)=pending.take(){if let Some(next)=p.sequence {sequence=next;ending=None;}shared.state.lock().await["sequence"]=sequence.as_ref().map(Sequence::snapshot).unwrap_or(Value::Null);let _=p.command.reply.send(Ok(()));}
                                    }
                                }
                            },
                            Some(Ok(Message::Ping(data)))=>{let _=ws.send(Message::Pong(data)).await;},
                            Some(Ok(Message::Close(_)))|None|Some(Err(_))=>break,
                            _=>{},
                        }
                    },
                    _=tick.tick()=>{
                        shared.state.lock().await["sequence"]=sequence.as_ref().map(Sequence::snapshot).unwrap_or(Value::Null);
                        if pending.is_none() && last_metadata_query.elapsed()>Duration::from_secs(2) {
                            let state=shared.state.lock().await;
                            let incomplete=state["logged_in"]==true && state["uri"].as_str().is_some_and(|uri|!uri.is_empty()) && (state["duration_ms"].as_u64().unwrap_or(0)==0 || state["title"].as_str().unwrap_or_default().is_empty());
                            drop(state);
                            if incomplete {let _=ws.send(Message::Text(json!({"type":"command","command":"get_state"}).to_string().into())).await;}
                            last_metadata_query=Instant::now();
                        }
                        if pending.as_ref().is_some_and(|p|p.started.elapsed()>Duration::from_secs(15)){if let Some(p)=pending.take(){let _=p.command.reply.send(Err("Provider command timed out; outcome unknown".into()));}break}},
                }
            }
            if let Some(p) = pending.take() {
                let _ = p.command.reply.send(Err(
                    "Provider connection lost; command outcome unknown".into(),
                ));
            }
        }
        {
            let mut s = shared.state.lock().await;
            s["connected"] = json!(false);
            s["logged_in"] = json!(false);
            s["status"] = json!("unavailable");
            s["error"] = json!("Soloist unavailable; check provider setup or pair the device");
        }
        while let Ok(cmd) = rx.try_recv() {
            let _ = cmd.reply.send(Err("Provider unavailable".into()));
        }
        tokio::select! {_=time::sleep(Duration::from_secs(2))=>{},_=stop.changed()=>return}
    }
}

async fn client(
    stream: UnixStream,
    shared: Arc<Shared>,
) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    if stream.peer_cred()?.uid() != unsafe { libc::getuid() } {
        return Ok(());
    }
    let (read, mut write) = stream.into_split();
    let mut reader = BufReader::new(read);
    let mut bytes = Vec::new();
    loop {
        let available = time::timeout(Duration::from_secs(5), reader.fill_buf()).await??;
        if available.is_empty() {
            return Ok(());
        };
        let count = available
            .iter()
            .position(|b| *b == b'\n')
            .map(|n| n + 1)
            .unwrap_or(available.len());
        if bytes.len() + count > MAX_FRAME {
            return Ok(());
        };
        bytes.extend_from_slice(&available[..count]);
        reader.consume(count);
        if bytes.last() == Some(&b'\n') {
            break;
        }
    }
    let req: Request = match serde_json::from_slice(&bytes) {
        Ok(r) => r,
        Err(_) => {
            write
                .write_all(b"{\"version\":1,\"id\":\"\",\"error\":\"Invalid JSON request\"}\n")
                .await?;
            return Ok(());
        }
    };
    let result: Result<Value, String> = if req.version != 1 {
        Err("Incompatible protocol version".into())
    } else {
        match req.method.as_str() {
            "hello" | "capabilities" => Ok(
                json!({"provider":"soloist","downloads":false,"offline_start":false,"local_playback":true,"version":env!("CARGO_PKG_VERSION")}),
            ),
            "state" => Ok(shared.state.lock().await.clone()),
            "shutdown" => Ok(json!({"stopping":true})),
            _ => match command(&req.method, &req.params) {
                Err(e) => Err(e),
                Ok(value) => {
                    let (tx, rx) = oneshot::channel();
                    match shared.commands.try_send(Command { value, reply: tx }) {
                        Err(_) => Err("Playback request queue is full".into()),
                        Ok(()) => match time::timeout(Duration::from_secs(17), rx).await {
                            Ok(Ok(Ok(()))) => Ok(json!({"accepted":true})),
                            Ok(Ok(Err(e))) => Err(e),
                            _ => Err("Provider unavailable or command outcome unknown".into()),
                        },
                    }
                }
            },
        }
    };
    let response = match result {
        Ok(v) => json!({"version":1,"id":req.id,"result":v}),
        Err(e) => json!({"version":1,"id":req.id,"error":e}),
    };
    write.write_all(format!("{response}\n").as_bytes()).await?;
    if req.version == 1 && req.method == "shutdown" {
        let _ = shared.stop.send(true);
    }
    Ok(())
}

#[tokio::main(worker_threads = 2)]
async fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let args: Vec<String> = std::env::args().skip(1).collect();
    if args.iter().any(|a| a == "--version") {
        println!("spotify-tui-playback {}", env!("CARGO_PKG_VERSION"));
        return Ok(());
    }
    if args.first().map(String::as_str) != Some("serve") {
        println!("Usage: spotify-tui-playback serve --socket PATH [--provider-url ws://127.0.0.1:PORT] [--soloist-path PATH --provider-data PATH --provider-cache PATH]\nProvider API key may be supplied on stdin.");
        return Ok(());
    }
    let options: HashMap<String, String> = args[1..]
        .chunks(2)
        .filter(|c| c.len() == 2)
        .map(|c| (c[0].clone(), c[1].clone()))
        .collect();
    let socket = PathBuf::from(options.get("--socket").ok_or("--socket required")?);
    let parent = socket.parent().ok_or("Invalid socket path")?;
    fs::create_dir_all(parent)?;
    if fs::symlink_metadata(parent)?.file_type().is_symlink() {
        return Err("Runtime directory must not be a symlink".into());
    };
    fs::set_permissions(parent, fs::Permissions::from_mode(0o700))?;
    let lock = fs::OpenOptions::new()
        .create(true)
        .truncate(false)
        .read(true)
        .write(true)
        .open(parent.join("helper.lock"))?;
    lock.try_lock_exclusive()
        .map_err(|_| "Playback helper already running")?;
    if socket.exists() {
        if fs::symlink_metadata(&socket)?.file_type().is_symlink() {
            return Err("Socket path is a symlink".into());
        };
        fs::remove_file(&socket)?
    }
    let mut provider = None;
    let address = if let Some(address) = options.get("--provider-url") {
        local_url(address)?;
        address.clone()
    } else {
        let data = PathBuf::from(
            options
                .get("--provider-data")
                .ok_or("--provider-data required")?,
        );
        let cache = options
            .get("--provider-cache")
            .ok_or("--provider-cache required")?;
        fs::create_dir_all(&data)?;
        fs::set_permissions(&data, fs::Permissions::from_mode(0o700))?;
        let mut key = String::new();
        std::io::stdin().take(4096).read_to_string(&mut key)?;
        let key = key.trim();
        if key.is_empty() {
            return Err("No Soloist API key; run spotify-tui provider-key".into());
        }
        clear_endpoint_files(&data)?;
        let mut child = tokio::process::Command::new(
            options
                .get("--soloist-path")
                .map(String::as_str)
                .unwrap_or("soloist"),
        )
        .args([
            "--device-name",
            "Spotify TUI Local",
            "--api-key",
            key,
            "--ws",
            "127.0.0.1:0",
            "--data-dir",
            data.to_str().ok_or("Invalid data path")?,
            "--cache-dir",
            cache,
            "--cache-size",
            "256",
        ])
        .stdin(std::process::Stdio::null())
        .stdout(std::process::Stdio::null())
        .stderr(std::process::Stdio::null())
        .kill_on_drop(true)
        .spawn()?;
        let started = Instant::now();
        let address = loop {
            if let Some(status) = child.try_wait()? {
                return Err(format!("Soloist exited before readiness ({status}); check key/build/audio configuration").into());
            }
            if let Ok(port) = fs::read_to_string(data.join("ws.port")) {
                if let Ok(port) = port.trim().parse::<u16>() {
                    if port > 0 {
                        break format!("ws://127.0.0.1:{port}");
                    }
                }
            }
            if started.elapsed() > Duration::from_secs(12) {
                stop_provider(&mut child).await;
                return Err("Soloist did not publish a control port".into());
            };
            time::sleep(Duration::from_millis(100)).await;
        };
        provider = Some(child);
        address
    };
    let listener = UnixListener::bind(&socket)?;
    fs::set_permissions(&socket, fs::Permissions::from_mode(0o600))?;
    let (tx, rx) = mpsc::channel(16);
    let (stop_tx, mut stop_rx) = watch::channel(false);
    let shared = Arc::new(Shared {
        state: Mutex::new(initial_state()),
        commands: tx,
        stop: stop_tx,
    });
    let bridge_handle = tokio::spawn(bridge(address, shared.clone(), rx, stop_rx.clone()));
    let permits = Arc::new(tokio::sync::Semaphore::new(16));
    let mut terminate = tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())?;
    loop {
        tokio::select! {
            connection=listener.accept()=>{let(stream,_)=connection?;if let Ok(permit)=permits.clone().try_acquire_owned(){let state=shared.clone();tokio::spawn(async move{let _permit=permit;let _=client(stream,state).await;});}},
            _=stop_rx.changed()=>break,
        _=tokio::signal::ctrl_c()=>{let _=shared.stop.send(true);break},
        _=terminate.recv()=>{let _=shared.stop.send(true);break},
        }
    }
    bridge_handle.abort();
    if let Some(mut child) = provider {
        stop_provider(&mut child).await;
    }
    let _ = fs::remove_file(socket);
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn refuses_unsupported_commands_and_invalid_volume() {
        assert!(command("download_start", &json!({}))
            .unwrap_err()
            .contains("UnsupportedCapability"));
        assert!(command("set_volume", &json!({"volume":101})).is_err());
    }
    #[test]
    fn refuses_remote_provider() {
        assert!(local_url("ws://example.com:9090").is_err());
        assert!(local_url("ws://127.0.0.1:9090").is_ok());
    }
    #[test]
    fn state_follows_provider_confirmation() {
        let mut s = initial_state();
        assert_eq!(s["status"], "stopped");
        apply_event(
            &mut s,
            &json!({"type":"playback_changed","status":"paused"}),
        );
        assert_eq!(s["status"], "paused");
        apply_event(
            &mut s,
            &json!({"type":"options_changed","options":{"repeat":"context","shuffle":true}}),
        );
        assert_eq!(s["repeat"], "context");
        assert_eq!(s["shuffle"], true);
        apply_event(
            &mut s,
            &json!({"type":"options_changed","options":{"repeat":"track"}}),
        );
        assert_eq!(s["repeat"], "track");
    }
    #[test]
    fn artwork_and_album_follow_tracks_and_partial_snapshots() {
        let mut state = initial_state();
        update_track(
            &mut state,
            &json!({"uri":"spotify:track:a","decorations":{"identity":{"name":"Song A"},"playback":{"duration_ms":123000},"parent":{"entity":{"decorations":{"identity":{"name":"Album A"}}}},"visual_identity":{"cover":[{"url":"https://i.scdn.co/image/small","size":"small"},{"url":"https://i.scdn.co/image/large","size":"large"}]}}}),
        );
        assert_eq!(state["art_url"], "https://i.scdn.co/image/large");
        assert_eq!(state["album"], "Album A");
        update_track(
            &mut state,
            &json!({"uri":"spotify:track:a","decorations":{}}),
        );
        assert_eq!(state["title"], "Song A");
        assert_eq!(state["duration_ms"], 123000);
        assert_eq!(state["art_url"], "https://i.scdn.co/image/large");
        update_track(
            &mut state,
            &json!({"uri":"spotify:track:b","decorations":{"identity":{"name":"Song B"}}}),
        );
        assert_eq!(state["art_url"], "");
        assert_eq!(state["album"], "");
        assert!(!valid_art_url("file:///tmp/cover.png"));
        assert!(!valid_art_url("https://example.com/cover.png"));
        assert!(!valid_art_url("https://i.scdn.co/image/quote'"));
    }

    #[test]
    fn sanitizes_command_boundary() {
        assert!(command("play_track", &json!({"uri":"file:///etc/passwd"})).is_err());
        assert!(command("seek", &json!({"position_ms":-1})).is_err());
    }

    #[test]
    fn stale_endpoint_cleanup_preserves_session_files() {
        let path =
            std::env::temp_dir().join(format!("spotify-tui-endpoints-{}", std::process::id()));
        fs::create_dir_all(&path).unwrap();
        fs::write(path.join("ws.port"), "12345").unwrap();
        fs::write(path.join("ws.addr"), "127.0.0.1").unwrap();
        fs::write(path.join("session-fixture"), "keep").unwrap();
        clear_endpoint_files(&path).unwrap();
        assert!(!path.join("ws.port").exists());
        assert_eq!(
            fs::read_to_string(path.join("session-fixture")).unwrap(),
            "keep"
        );
        fs::remove_dir_all(path).unwrap();
    }

    #[tokio::test]
    async fn pause_waits_for_provider_confirmation() {
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let address = format!("ws://{}", listener.local_addr().unwrap());
        let (release_tx, release_rx) = oneshot::channel();
        let server = tokio::spawn(async move {
            let (socket, _) = listener.accept().await.unwrap();
            let mut ws = tokio_tungstenite::accept_async(socket).await.unwrap();
            ws.send(Message::Text(
                json!({"type":"auth_state","logged_in":true})
                    .to_string()
                    .into(),
            ))
            .await
            .unwrap();
            let query = ws.next().await.unwrap().unwrap().into_text().unwrap();
            assert_eq!(
                serde_json::from_str::<Value>(&query).unwrap()["command"],
                "get_state"
            );
            ws.send(Message::Text(json!({"type":"playback_state","status":"playing","item":{"uri":"spotify:track:fixture","decorations":{"identity":{"name":"Fixture track"},"playback":{"duration_ms":120000}}},"position":{"position_ms":1000}}).to_string().into())).await.unwrap();
            let pause = ws.next().await.unwrap().unwrap().into_text().unwrap();
            assert_eq!(
                serde_json::from_str::<Value>(&pause).unwrap()["command"],
                "pause"
            );
            ws.send(Message::Text(
                json!({"type":"command_result","command":"pause"})
                    .to_string()
                    .into(),
            ))
            .await
            .unwrap();
            let _ = release_rx.await;
            ws.send(Message::Text(
                json!({"type":"playback_changed","status":"paused"})
                    .to_string()
                    .into(),
            ))
            .await
            .unwrap();
            time::sleep(Duration::from_millis(100)).await;
        });
        let (tx, rx) = mpsc::channel(4);
        let (stop, stop_rx) = watch::channel(false);
        let shared = Arc::new(Shared {
            state: Mutex::new(initial_state()),
            commands: tx,
            stop,
        });
        let task = tokio::spawn(bridge(address, shared.clone(), rx, stop_rx));
        for _ in 0..100 {
            if shared.state.lock().await["status"] == "playing" {
                break;
            }
            time::sleep(Duration::from_millis(10)).await;
        }
        assert_eq!(shared.state.lock().await["title"], "Fixture track");
        let (reply, mut receive) = oneshot::channel();
        shared
            .commands
            .send(Command {
                value: command("pause", &json!({})).unwrap(),
                reply,
            })
            .await
            .unwrap();
        assert!(time::timeout(Duration::from_millis(100), &mut receive)
            .await
            .is_err());
        assert_eq!(shared.state.lock().await["status"], "playing");
        release_tx.send(()).unwrap();
        assert!(time::timeout(Duration::from_secs(2), receive)
            .await
            .unwrap()
            .unwrap()
            .is_ok());
        for _ in 0..100 {
            if shared.state.lock().await["status"] == "paused" {
                break;
            }
            time::sleep(Duration::from_millis(5)).await;
        }
        assert_eq!(shared.state.lock().await["status"], "paused");
        let _ = shared.stop.send(true);
        task.await.unwrap();
        server.await.unwrap();
    }

    #[tokio::test]
    async fn queued_toggles_follow_confirmed_state() {
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let address = format!("ws://{}", listener.local_addr().unwrap());
        let server = tokio::spawn(async move {
            let (socket, _) = listener.accept().await.unwrap();
            let mut ws = tokio_tungstenite::accept_async(socket).await.unwrap();
            ws.send(Message::Text(
                json!({"type":"auth_state","logged_in":true})
                    .to_string()
                    .into(),
            ))
            .await
            .unwrap();
            let mut status = "playing";
            let mut actions = Vec::new();
            while let Some(Ok(Message::Text(text))) = ws.next().await {
                let value: Value = serde_json::from_str(&text).unwrap();
                let name = value["command"].as_str().unwrap();
                if name == "get_state" {
                    ws.send(Message::Text(json!({"type":"playback_state","status":status,"item":{"uri":"spotify:track:fixture"}}).to_string().into())).await.unwrap();
                } else {
                    actions.push(name.to_string());
                    ws.send(Message::Text(
                        json!({"type":"command_result","command":name})
                            .to_string()
                            .into(),
                    ))
                    .await
                    .unwrap();
                    time::sleep(Duration::from_millis(30)).await;
                    status = if name == "pause" { "paused" } else { "playing" };
                    ws.send(Message::Text(
                        json!({"type":"playback_changed","status":status})
                            .to_string()
                            .into(),
                    ))
                    .await
                    .unwrap();
                    if actions.len() == 2 {
                        return actions;
                    }
                }
            }
            actions
        });
        let (tx, rx) = mpsc::channel(4);
        let (stop, stop_rx) = watch::channel(false);
        let shared = Arc::new(Shared {
            state: Mutex::new(initial_state()),
            commands: tx,
            stop,
        });
        let task = tokio::spawn(bridge(address, shared.clone(), rx, stop_rx));
        for _ in 0..100 {
            if shared.state.lock().await["status"] == "playing" {
                break;
            };
            time::sleep(Duration::from_millis(10)).await;
        }
        let mut replies = Vec::new();
        for _ in 0..2 {
            let (reply, receive) = oneshot::channel();
            shared
                .commands
                .send(Command {
                    value: command("toggle_pause", &json!({})).unwrap(),
                    reply,
                })
                .await
                .unwrap();
            replies.push(receive);
        }
        for reply in replies {
            assert!(time::timeout(Duration::from_secs(2), reply)
                .await
                .unwrap()
                .unwrap()
                .is_ok());
        }
        assert_eq!(server.await.unwrap(), vec!["pause", "play"]);
        let _ = shared.stop.send(true);
        task.await.unwrap();
    }

    #[tokio::test]
    async fn sequence_continues_after_buffering_and_position_reset() {
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let address = format!("ws://{}", listener.local_addr().unwrap());
        let server = tokio::spawn(async move {
            let (socket, _) = listener.accept().await.unwrap();
            let mut ws = tokio_tungstenite::accept_async(socket).await.unwrap();
            ws.send(Message::Text(
                json!({"type":"auth_state","logged_in":true})
                    .to_string()
                    .into(),
            ))
            .await
            .unwrap();
            let mut uri = String::from("spotify:track:a");
            let mut playing = false;
            let mut ended = false;
            while let Some(Ok(Message::Text(text))) = ws.next().await {
                let v: Value = serde_json::from_str(&text).unwrap();
                assert!(v.get("_sequence").is_none());
                let name = v["command"].as_str().unwrap();
                if name == "play" {
                    uri = v["uri"].as_str().unwrap().to_owned();
                    playing = true;
                    ws.send(Message::Text(
                        json!({"type":"command_result","command":"play"})
                            .to_string()
                            .into(),
                    ))
                    .await
                    .unwrap();
                }
                ws.send(Message::Text(json!({"type":"playback_state","status":if playing {"playing"} else {"paused"},"item":{"uri":uri,"decorations":{"identity":{"name":"Fixture"},"playback":{"duration_ms":10000}}},"position":{"position_ms":1000},"options":{"shuffle":false}}).to_string().into())).await.unwrap();
                if name == "get_state" && playing && uri == "spotify:track:a" && !ended {
                    ended = true;
                    time::sleep(Duration::from_millis(30)).await;
                    for event in [
                        json!({"type":"position_sync","position":{"position_ms":9900}}),
                        json!({"type":"playback_changed","status":"buffering"}),
                        json!({"type":"position_sync","position":{"position_ms":0}}),
                        json!({"type":"track_changed","item":{"uri":"spotify:track:autoplay","decorations":{"playback":{"duration_ms":10000}}}}),
                    ] {
                        ws.send(Message::Text(event.to_string().into()))
                            .await
                            .unwrap();
                    }
                }
            }
        });
        let (tx, rx) = mpsc::channel(16);
        let (stop, _) = watch::channel(false);
        let shared = Arc::new(Shared {
            state: Mutex::new(initial_state()),
            commands: tx,
            stop,
        });
        let task = tokio::spawn(bridge(address, shared.clone(), rx, shared.stop.subscribe()));
        time::timeout(Duration::from_secs(2), async {
            while shared.state.lock().await["logged_in"] != true {
                time::sleep(Duration::from_millis(5)).await
            }
        })
        .await
        .unwrap();
        let (reply, done) = oneshot::channel();
        shared
            .commands
            .send(Command {
                value: command(
                    "play_sequence",
                    &json!({"uris":["spotify:track:a","spotify:track:b"],"index":0}),
                )
                .unwrap(),
                reply,
            })
            .await
            .unwrap();
        assert!(done.await.unwrap().is_ok());
        time::timeout(Duration::from_secs(2), async {
            while shared.state.lock().await["uri"] != "spotify:track:b" {
                time::sleep(Duration::from_millis(5)).await
            }
        })
        .await
        .unwrap();
        task.abort();
        server.abort();
    }

    #[test]
    fn shared_protocol_fixture_is_accepted() {
        let req: Request =
            serde_json::from_str(include_str!("../../protocol/fixtures/hello-request.json"))
                .unwrap();
        assert_eq!(req.version, 1);
        assert_eq!(req.method, "hello");
        assert_eq!(req.id, "1");
    }
}
