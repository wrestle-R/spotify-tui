use serde_json::{json, Value};

#[derive(Clone, Debug)]
pub struct Sequence {
    pub uris: Vec<String>,
    pub index: usize,
}
impl Sequence {
    pub fn parse(params: &Value) -> Result<Self, String> {
        let items = params["uris"]
            .as_array()
            .filter(|a| !a.is_empty() && a.len() <= 5000)
            .ok_or("Sequence needs 1 to 5000 tracks")?;
        let uris: Vec<String> = items
            .iter()
            .map(|v| {
                v.as_str()
                    .filter(|s| valid_track(s))
                    .map(String::from)
                    .ok_or("Invalid sequence track".into())
            })
            .collect::<Result<_, String>>()?;
        let index = params["index"]
            .as_u64()
            .filter(|i| *i < uris.len() as u64)
            .ok_or("Invalid sequence offset")? as usize;
        Ok(Self { uris, index })
    }
    pub fn uri(&self) -> &str {
        &self.uris[self.index]
    }
    pub fn step(&self, forward: bool, state: &Value) -> Option<Self> {
        let mut next = self.clone();
        if forward {
            if state["shuffle"] == true && self.uris.len() > 1 {
                next.index = (self.index + 1 + (super::now_ms() as usize % (self.uris.len() - 1)))
                    % self.uris.len();
            } else if self.index + 1 < self.uris.len() {
                next.index += 1;
            } else if state["repeat"] == "context" {
                next.index = 0;
            } else {
                return None;
            }
        } else if self.index > 0 {
            next.index -= 1;
        }
        Some(next)
    }
    pub fn naturally_finished(&self, before: &Value, after: &Value) -> bool {
        if before["uri"] != self.uri()
            || before["status"] != "playing"
            || before["repeat"] == "track"
        {
            return false;
        }
        let duration = before["duration_ms"].as_u64().unwrap_or(0);
        let position = super::position_ms(before);
        duration > 0
            && position >= duration.saturating_sub(2000)
            && (after["uri"] != before["uri"] || after["status"] == "stopped")
    }
    pub fn play(&self) -> Value {
        json!({"type":"command","command":"play","uri":self.uri(),"_sequence":self.uris,"_index":self.index})
    }
}
pub fn valid_track(uri: &str) -> bool {
    uri.strip_prefix("spotify:track:").is_some_and(|id| {
        !id.is_empty() && id.len() <= 128 && id.chars().all(|c| c.is_ascii_alphanumeric())
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn sequence_offsets_and_boundaries() {
        let s = Sequence::parse(
            &json!({"uris":["spotify:track:a","spotify:track:b","spotify:track:c"],"index":1}),
        )
        .unwrap();
        assert_eq!(s.uri(), "spotify:track:b");
        assert_eq!(s.step(false, &json!({})).unwrap().uri(), "spotify:track:a");
        let last = s.step(true, &json!({})).unwrap();
        assert!(last.step(true, &json!({})).is_none());
        assert_eq!(
            last.step(true, &json!({"repeat":"context"})).unwrap().uri(),
            "spotify:track:a"
        );
        assert!(Sequence::parse(&json!({"uris":["file:///tmp/a"],"index":0})).is_err());
        assert!(Sequence::parse(&json!({"uris":["spotify:track:a"],"index":1})).is_err());
    }
    #[test]
    fn natural_end_does_not_override_pause_or_remote_transfer() {
        let s = Sequence::parse(&json!({"uris":["spotify:track:a","spotify:track:b"],"index":0}))
            .unwrap();
        let mut before =
            json!({"uri":s.uri(),"status":"playing","position_ms":99000,"duration_ms":100000});
        assert!(s.naturally_finished(
            &before,
            &json!({"uri":"spotify:track:autoplay","status":"playing"})
        ));
        assert!(!s.naturally_finished(&before, &json!({"uri":s.uri(),"status":"paused"})));
        before["position_ms"] = json!(1000);
        assert!(!s.naturally_finished(
            &before,
            &json!({"uri":"spotify:track:remote","status":"playing"})
        ));
    }
}
