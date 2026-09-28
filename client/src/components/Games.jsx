import React, { useEffect, useMemo, useRef, useState } from "react";
import { Frame } from "./Frame.jsx";
import { ScoreBoardPanel } from "./ScoreBoardPanel.jsx";
import { Controls } from "./Controls.jsx";
import { RoundSelector } from "./RoundSelector.jsx";
import { DemoMenu } from "./DemoMenu.jsx";
import { AnalysisPanel } from "./AnalysisPanel.jsx";
import { PostPlantPanel } from "./PostPlantPanel.jsx";
import { SettingsMenu } from "./SettingsMenu.jsx";
import { CoachPanel } from "./CoachPanel.jsx";
import { getSetting, getLLMConfig, llmConfigured } from "../utils/settings";
import { FilterMenu } from "./FilterMenu.jsx";
import { mapDisplayName } from "../maps/config";
import { getMySteamId, setMySteamId as persistMySteamId } from "../utils/me";
import { toggleFullscreen } from "../utils/fullscreen";

const API = "";
const TICK_MS = 16; // ~60fps playback at 1x
const BANNER = process.env.PUBLIC_URL + "/logo.png";
const ICON = process.env.PUBLIC_URL + "/statetrak.png";

export const Games = () => {
  const [demos, setDemos] = useState([]);
  const [demoId, setDemoId] = useState(null);
  const [output, setOutput] = useState({});
  const [index, setIndex] = useState(0);
  const [round, setRound] = useState(0);
  const [loading, setLoading] = useState(false);
  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState(1);
  const [metaData, setMetaData] = useState({});
  const [rounds, setRounds] = useState([]);
  const [focusPlayer, setFocusPlayer] = useState(null);
  const [uploading, setUploading] = useState(false);
  const [analysis, setAnalysis] = useState(null); // { loading } | { data }
  const [postplant, setPostplant] = useState(null); // { loading } | { data }
  const [coach, setCoach] = useState(null); // { loading } | { text, model }
  const [uploadProgress, setUploadProgress] = useState(null);
  const [toast, setToast] = useState(null);
  const [filters, setFilters] = useState({
    health: true,
    names: true,
    trails: true,
    theater: false,
  });
  const [mySteamId, setMySteamIdState] = useState(() => getMySteamId());
  const toastTimer = useRef(null);
  const roundFetchId = useRef(0);
  // frame index to jump to once the next round finishes loading
  // (PostPlantPanel "watch from plant" jumps)
  const pendingIndexRef = useRef(null);

  const fetchDemos = async () => {
    try {
      const res = await fetch(`${API}/demos`);
      const json = await res.json();
      setDemos(Array.isArray(json) ? json : []);
    } catch (err) {
      console.error(err);
    }
  };

  const fetchMeta = async (id) => {
    try {
      const meta = await fetch(`${API}/demos/${id}/output`);
      const json = await meta.json();
      setMetaData(json);
    } catch (err) {
      console.error(err);
    }
  };

  const fetchRounds = async (id) => {
    try {
      const res = await fetch(`${API}/demos/${id}/rounds`);
      setRounds(await res.json());
    } catch (err) {
      console.error(err);
    }
  };

  const fetchRound = async (id, r) => {
    const reqId = ++roundFetchId.current;
    // consume any pending jump target (PostPlantPanel "watch from plant")
    // at request time — a superseded request must not apply it to a newer
    // round load
    const pending = pendingIndexRef.current;
    pendingIndexRef.current = null;
    try {
      const res = await fetch(`${API}/demos/${id}/${r}`);
      const data = await res.json();
      if (reqId !== roundFetchId.current) return; // a newer round was requested
      setOutput(data);
      setIndex(pending ?? 0);
    } catch (e) {
      console.log(e);
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchDemos();
  }, []);

  // On demo switch: reset view state, fetch meta + round list.
  useEffect(() => {
    if (!demoId) return;
    setPlaying(false);
    setFocusPlayer(null);
    setLoading(true);
    setAnalysis(null);
    setPostplant(null);
    setCoach(null);
    pendingIndexRef.current = null;
    fetchMeta(demoId);
    fetchRounds(demoId);
    setRound(0);
  }, [demoId]);

  // Clear focus when switching demos (stale steamId otherwise).
  useEffect(() => {
    if (!demoId) setFocusPlayer(null);
  }, [demoId]);

  // Load round data when the selected round changes.
  useEffect(() => {
    if (!demoId) return;
    fetchRound(demoId, round);
  }, [round, demoId]);

  const tick = () => {
    setIndex((i) => {
      const max = (output.frames?.length ?? 1) - 1;
      return i < max ? i + 1 : i;
    });
  };

  useEffect(() => {
    if (!playing) return undefined;
    const inter = setInterval(tick, TICK_MS / speed);
    return () => clearInterval(inter);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- tick only closes over output, which is a dep
  }, [playing, speed, output]);

  // Auto-advance to the next round (or stop after the last one).
  useEffect(() => {
    if (!playing) return;
    const max = (output.frames?.length ?? 1) - 1;
    if (index < max) return;
    if (output.round != null && output.round !== round) return; // next round still loading
    if (round < rounds.length - 1) {
      setRound((r) => r + 1);
    } else {
      setPlaying(false);
    }
  }, [index, playing, round, rounds.length, output]);

  const tooglePlay = (play) => {
    setPlaying(play);
  };
  const selectPlayer = (steamId) => {
    setFocusPlayer((prev) => (prev === steamId ? null : steamId));
  };
  const toggleFilter = (key) => {
    setFilters((prev) => ({ ...prev, [key]: !prev[key] }));
  };
  const changeMySteamId = (id) => {
    setMySteamIdState(id);
    persistMySteamId(id);
  };

  const showToast = (msg) => {
    setToast(msg);
    if (toastTimer.current) clearTimeout(toastTimer.current);
    toastTimer.current = setTimeout(() => setToast(null), 5000);
  };

  // JEV round analysis: first run takes a few seconds (cached server-side
  // afterwards, so everyone shares one analysis per demo). Uses the user's
  // own TypeSafe key (🔑 settings) when set, else the server's env key.
  const runAnalysis = async () => {
    if (!demoId) return;
    setAnalysis({ loading: true });
    try {
      const headers = {};
      const jevKey = getSetting("jevKey");
      if (jevKey) headers["X-TypeSafe-Key"] = jevKey;
      const res = await fetch(`${API}/demos/${demoId}/analysis`, { headers });
      const json = await res.json();
      if (!res.ok) throw new Error(json.error || `HTTP ${res.status}`);
      setAnalysis({ loading: false, data: json });
    } catch (err) {
      setAnalysis(null);
      showToast("JEV analysis failed: " + err.message);
    }
  };

  // AI coach: sends the user's own LLM config (🔑 settings) plus the demo
  // id; the server assembles the context and calls the provider.
  const runCoach = async () => {
    if (!demoId) return;
    if (!llmConfigured()) {
      showToast("Add your LLM provider in the 🔑 settings first");
      return;
    }
    setCoach({ loading: true });
    try {
      const res = await fetch(`${API}/demos/${demoId}/coach`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          llm: getLLMConfig(),
          steamId: mySteamId,
        }),
      });
      const json = await res.json();
      if (!res.ok) throw new Error(json.error || `HTTP ${res.status}`);
      setCoach({ loading: false, text: json.text, model: json.model });
    } catch (err) {
      setCoach(null);
      showToast("AI coach failed: " + err.message);
    }
  };

  // Post-plant positioning analysis: deterministic, computed server-side on
  // the round files and cached as postplant.json (subsequent calls are fast).
  const runPostPlant = async () => {
    if (!demoId) return;
    setPostplant({ loading: true });
    try {
      const res = await fetch(`${API}/demos/${demoId}/postplant`);
      const json = await res.json();
      if (!res.ok) throw new Error(json.error || `HTTP ${res.status}`);
      setPostplant({ loading: false, data: json });
    } catch (err) {
      setPostplant(null);
      showToast("Post-plant analysis failed: " + err.message);
    }
  };

  // Jump the playback to the plant moment of a round (from the
  // PostPlantPanel). If the round is already loaded, seek directly;
  // otherwise remember the frame for when the fetch completes.
  const watchFromPlant = (r, frameIndex) => {
    setPlaying(false);
    setPostplant(null);
    if (r === round && output.frames?.length) {
      setIndex(Math.min(frameIndex, output.frames.length - 1));
    } else {
      pendingIndexRef.current = frameIndex;
      setRound(r);
    }
  };

  const deleteDemo = async (id) => {
    try {
      await fetch(`${API}/demos/${id}`, { method: "DELETE" });
      if (demoId === id) {
        setDemoId(null);
        setOutput({});
        setRounds([]);
        setMetaData({});
      }
      await fetchDemos();
    } catch (err) {
      console.error(err);
    }
  };

  // Upload via XHR so we get real upload progress, then poll parse progress.
  const onUpload = (e) => {
    const file = e.target.files?.[0];
    if (!file) return;
    e.target.value = ""; // allow re-selecting the same file
    setUploading(true);
    setUploadProgress({ phase: "uploading", pct: 0 });
    const form = new FormData();
    form.append("file", file);
    const xhr = new XMLHttpRequest();
    xhr.open("POST", `${API}/upload`);
    xhr.upload.onprogress = (ev) => {
      if (ev.lengthComputable) {
        setUploadProgress({
          phase: "uploading",
          pct: Math.round((ev.loaded / ev.total) * 100),
        });
      }
    };
    xhr.onload = () => {
      try {
        const { id } = JSON.parse(xhr.responseText);
        setUploadProgress({ phase: "parsing", pct: 0 });
        pollStatus(id);
      } catch (err) {
        console.error(err);
        setUploading(false);
        setUploadProgress(null);
        showToast("Upload failed: " + (xhr.responseText || "unknown error"));
      }
    };
    xhr.onerror = () => {
      setUploading(false);
      setUploadProgress(null);
      showToast("Upload failed");
    };
    xhr.send(form);
  };

  const pollStatus = (id) => {
    const poll = async () => {
      try {
        const res = await fetch(`${API}/demos/${id}/status`);
        const status = await res.json();
        if (status.status === "done") {
          setUploading(false);
          setUploadProgress(null);
          await fetchDemos();
          setDemoId(id);
        } else if (status.status === "error") {
          setUploading(false);
          setUploadProgress(null);
          showToast("Failed to parse demo: " + (status.error || "unknown error"));
        } else {
          if (typeof status.progress === "number" && status.progress > 0) {
            setUploadProgress({ phase: "parsing", pct: status.progress });
          }
          setTimeout(poll, 1000);
        }
      } catch (err) {
        console.error(err);
        setUploading(false);
        setUploadProgress(null);
      }
    };
    poll();
  };

  useEffect(() => {
    const onKeyDown = (e) => {
      if (e.target.tagName === "INPUT") return;
      if (e.code === "Space") {
        e.preventDefault();
        setPlaying((p) => !p);
      } else if (e.key === "ArrowRight") {
        e.preventDefault();
        setPlaying(false);
        setIndex((i) => Math.min(i + 1, (output.frames?.length ?? 1) - 1));
      } else if (e.key === "ArrowLeft") {
        e.preventDefault();
        setPlaying(false);
        setIndex((i) => Math.max(i - 1, 0));
      } else if (e.key === "F11") {
        e.preventDefault();
        toggleFullscreen();
      } else if (e.key === "Escape") {
        setFocusPlayer(null);
      } else if (e.key === "," || e.key === "[") {
        e.preventDefault();
        if (round > 0) {
          setPlaying(false);
          setRound((r) => r - 1);
        }
      } else if (e.key === "." || e.key === "]") {
        e.preventDefault();
        if (round < rounds.length - 1) {
          setPlaying(false);
          setRound((r) => r + 1);
        }
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [output.frames?.length, round, rounds.length]);

  // Throttle scoreboard updates to ~5Hz; the tables don't need 60fps.
  const sbIndex = output.frames
    ? Math.min(Math.floor(index / 12) * 12, output.frames.length - 1)
    : 0;
  const sbFrame = output.frames?.[sbIndex];

  // The team I'm on in the round currently loaded. Computed per frame from
  // the player states (teams swap at halftime, so a demo-wide answer is
  // wrong); dead players keep their team, so it stays correct all round.
  const currentFrame = output.frames
    ? output.frames[Math.min(index, output.frames.length - 1)]
    : null;
  const myTeam = useMemo(() => {
    const me = currentFrame?.playerStates?.find(
      (p) => p.steamId === mySteamId,
    );
    return me?.team || null;
  }, [currentFrame, mySteamId]);

  // Which team I was on in *each* round (from the roster steam ids shipped
  // with the round summaries), keyed by round number. Gaps (rounds without
  // an economy snapshot) inherit the nearest known round's side.
  const myTeamByRound = useMemo(() => {
    const sides = {};
    rounds.forEach((r) => {
      if (r.ctSteamIds?.includes(mySteamId)) sides[r.round] = "CT";
      else if (r.tSteamIds?.includes(mySteamId)) sides[r.round] = "T";
    });
    const nums = rounds.map((r) => r.round);
    // forward-fill gaps from the nearest earlier round with a known side
    let last = null;
    const resolved = {};
    nums.forEach((n) => {
      if (sides[n]) last = sides[n];
      resolved[n] = sides[n] || last;
    });
    // back-fill leading gaps from the first round with a known side
    const firstKnown = nums.find((n) => sides[n]);
    if (firstKnown != null) {
      nums.forEach((n) => {
        if (n < firstKnown) resolved[n] = sides[firstKnown];
      });
    }
    return resolved;
  }, [rounds, mySteamId]);

  return (
    <div>
      <DemoMenu
        demos={demos}
        demoId={demoId}
        uploading={uploading}
        uploadProgress={uploadProgress}
        analysisRunning={!!analysis?.loading}
        postplantRunning={!!postplant?.loading}
        coachRunning={!!coach?.loading}
        onSelectDemo={setDemoId}
        onUpload={onUpload}
        onDeleteDemo={deleteDemo}
        onRunAnalysis={runAnalysis}
        onRunPostPlant={runPostPlant}
        onRunCoach={runCoach}
      />

      {analysis?.data && (
        <AnalysisPanel
          analysis={analysis.data}
          onClose={() => setAnalysis(null)}
        />
      )}

      {postplant?.data && (
        <PostPlantPanel
          data={postplant.data}
          mapName={metaData.map}
          mySteamId={mySteamId}
          players={metaData.players}
          roundSummaries={rounds}
          onClose={() => setPostplant(null)}
          onWatch={watchFromPlant}
        />
      )}

      {coach?.text && (
        <CoachPanel
          text={coach.text}
          model={coach.model}
          onClose={() => setCoach(null)}
        />
      )}

      {toast && <div className="toast">{toast}</div>}

      {!demoId ? (
        <div className="empty-state">
          <img className="splash-logo" src={BANNER} alt="StateTrak" />
          <p>Upload or select a demo to begin</p>
          <SettingsMenu label="🔑 AI settings" />
        </div>
      ) : loading ? (
        <div className="empty-state">…loading</div>
      ) : (
        <>
          <h1 className="map-title">
            <img className="map-logo" src={ICON} alt="" />
            {mapDisplayName(metaData.map)}
          </h1>
          <div className={`main-layout ${filters.theater ? "theater" : ""}`}>
            <ScoreBoardPanel
              frame={sbFrame}
              onSelectPlayer={selectPlayer}
              focusPlayer={focusPlayer}
              economy={output.economy}
              mySteamId={mySteamId}
              myTeam={myTeam}
            />
            <Frame
              mapName={metaData.map}
              frame={currentFrame}
              frames={output.frames}
              kills={output.kills}
              index={index}
              focusPlayer={focusPlayer}
              filters={filters}
              onSelectPlayer={selectPlayer}
              mySteamId={mySteamId}
              myTeam={myTeam}
              backdrop={BANNER}
            />
          </div>
          <Controls
            playing={playing}
            onTogglePlay={tooglePlay}
            speed={speed}
            onSpeedChange={setSpeed}
          >
            <FilterMenu
              filters={filters}
              onToggle={toggleFilter}
              mySteamId={mySteamId}
              onMySteamIdChange={changeMySteamId}
              myTeamActive={!!myTeam}
            />
            <SettingsMenu />
          </Controls>

          <RoundSelector
            rounds={rounds}
            currentRound={round}
            onSelect={(i) => {
              setPlaying(false);
              setRound(i);
            }}
            output={output}
            index={index}
            onIndexChange={setIndex}
            mySteamId={mySteamId}
            myTeam={myTeam}
            myTeamByRound={myTeamByRound}
          />
        </>
      )}
    </div>
  );
};