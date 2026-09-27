import React, { useRef, useEffect, useState, useMemo } from "react";
import { MAPS, RADAR_NATIVE_SIZE } from "../maps/config";
import { KillFeed } from "./KillFeed.jsx";
import { FocusHud } from "./FocusHud.jsx";

const clamp = (v, min, max) => Math.min(max, Math.max(min, v));

const TRAIL_FRAMES = 120; // ~2s of movement history
const KILL_MARKER_FRAMES = 300; // kill markers fade over ~5s
const DEAD_FADE_FRAMES = 240; // dead dots fade over ~4s

export const Frame = ({
  frame,
  frames,
  kills,
  index,
  mapName,
  focusPlayer,
  filters,
  onSelectPlayer,
  mySteamId,
  myTeam,
}) => {
  const canvasRef = useRef(null);
  const containerRef = useRef(null);
  const wrapperRef = useRef(null);
  const [imgSize, setImgSize] = useState({ width: 1024, height: 1024 });
  const [displaySize, setDisplaySize] = useState({ width: 500, height: 500 });
  const [view, setView] = useState({ zoom: 1, offsetX: 0, offsetY: 0 });
  const [focusZoom, setFocusZoom] = useState(2.5);
  const imgRef = useRef(null);
  const pinchRef = useRef(null);
  const dragRef = useRef(null);
  const dragMovedRef = useRef(false);

  const map = MAPS[mapName] || MAPS.de_ancient;

  useEffect(() => {
    const img = new Image();
    img.onload = () => {
      imgRef.current = img;
      setImgSize({ width: img.naturalWidth, height: img.naturalHeight });
    };
    img.src = map.image;
  }, [map.image]);

  useEffect(() => {
    const el = wrapperRef.current;
    if (!el) return;
    const update = () => {
      const width = el.clientWidth || window.innerWidth - 40;
      const height = window.innerHeight - 200;
      setDisplaySize({ width, height });
    };
    update();
    const ro = new ResizeObserver(update);
    ro.observe(el);
    window.addEventListener("resize", update);
    return () => {
      ro.disconnect();
      window.removeEventListener("resize", update);
    };
  }, []);

  const cover = () => {
    const scale = Math.min(
      displaySize.width / imgSize.width,
      displaySize.height / imgSize.height,
    );
    return {
      scale,
      offsetX: (displaySize.width - imgSize.width * scale) / 2,
      offsetY: (displaySize.height - imgSize.height * scale) / 2,
    };
  };

  const transformPos = (posX, posY) => {
    const nativeScale = RADAR_NATIVE_SIZE / imgSize.width;
    const ix = (posX - map.posX) / map.scale / nativeScale;
    const iy = (map.posY - posY) / map.scale / nativeScale;
    const { scale, offsetX, offsetY } = cover();
    return {
      x: ix * scale + offsetX,
      y: iy * scale + offsetY,
    };
  };

  const worldRadiusToPixels = (worldRadius) => {
    const nativeScale = RADAR_NATIVE_SIZE / imgSize.width;
    const { scale } = cover();
    return (worldRadius / map.scale / nativeScale) * scale;
  };

  // Per-player position history across the round, for trails.
  const trails = useMemo(() => {
    const m = new Map();
    if (!frames) return m;
    frames.forEach((f, i) => {
      f.playerStates?.forEach((ps) => {
        let arr = m.get(ps.steamId);
        if (!arr) {
          arr = new Array(frames.length);
          m.set(ps.steamId, arr);
        }
        arr[i] = { x: ps.position.x, y: ps.position.y, alive: ps.alive };
      });
    });
    return m;
  }, [frames]);

  // Frame index at which each player died, for fading dead dots.
  const deathIndex = useMemo(() => {
    const m = new Map();
    if (!kills?.length || !frames?.length) return m;
    const first = frames[0].frame;
    kills.forEach((k) => {
      if (k.victimSteamId && !m.has(k.victimSteamId)) {
        m.set(k.victimSteamId, k.frame - first);
      }
    });
    return m;
  }, [kills, frames]);

  const draw = (ctx, player) => {
    const { position, alive, team, firing, yaw, name, health } = player;
    const isMe = mySteamId && player.steamId === mySteamId;

    let alpha = 1;
    if (!alive) {
      const di = deathIndex.get(player.steamId);
      if (di != null) {
        const age = index - di;
        if (age > DEAD_FADE_FRAMES) return; // long dead: don't clutter the map
        alpha = Math.max(0.2, 1 - age / DEAD_FADE_FRAMES);
      }
    }
    // "Focus my team": keep my team at full strength, fade out the enemy.
    if (filters.teamFocus && myTeam && team && team !== myTeam && alive) {
      alpha *= 0.35;
    }

    ctx.save();
    ctx.globalAlpha = alpha;
    ctx.fillStyle = team === "CT" ? "#68a3e5" : "#e6f13d";
    if (!alive) {
      ctx.fillStyle = "#515151";
    }
    var rad = (90 - yaw) * (Math.PI / 180);
    var { x, y } = transformPos(position.x, position.y);

    if (filters.names) {
      ctx.font = "11px sans-serif";
      ctx.lineWidth = 3;
      ctx.strokeStyle = "rgba(0, 0, 0, 0.85)";
      ctx.strokeText(name, x + 6, y - 6);
      ctx.fillStyle = alive ? "#ffffff" : "#bbbbbb";
      ctx.fillText(name, x + 6, y - 6);
      // restore the dot color for the body below
      ctx.fillStyle = team === "CT" ? "#68a3e5" : "#e6f13d";
      if (!alive) {
        ctx.fillStyle = "#515151";
      }
    }
    ctx.strokeStyle = "#ffffff";
    if (!alive) {
      ctx.strokeStyle = "#939393";
    }

    ctx.beginPath();
    ctx.arc(x, y, 4, 0, 2 * Math.PI);
    ctx.fill();
    if (alive) {
      ctx.fillStyle = "#00FF00";
      ctx.beginPath();
      ctx.translate(x, y);
      ctx.rotate(rad);
      ctx.rect(0, -15, 1, 15);
      if (firing) {
        ctx.fillStyle = "#FFFFFF";
        ctx.beginPath();
        ctx.rect(0, -30, 1, 30);
        ctx.fill();
      }
      ctx.fill();
    }
    ctx.restore();

    // My own dot: white halo ring so I can always find myself, drawn on top
    // (see the draw order below).
    if (isMe) {
      ctx.save();
      ctx.globalAlpha = 1;
      ctx.strokeStyle = "#ffffff";
      ctx.lineWidth = 1.5;
      ctx.beginPath();
      ctx.arc(x, y, 7, 0, 2 * Math.PI);
      ctx.stroke();
      ctx.restore();
    }

    if (alive && health != null && filters.health) {
      const barWidth = 20;
      const barHeight = 3;
      const bx = x - barWidth / 2;
      const by = y - 14;
      ctx.fillStyle = "rgba(0, 0, 0, 0.6)";
      ctx.fillRect(bx, by, barWidth, barHeight);
      const ratio = Math.max(0, Math.min(1, health / 100));
      ctx.fillStyle =
        ratio > 0.5 ? "#00cc00" : ratio > 0.25 ? "#ffaa00" : "#ff0000";
      ctx.fillRect(bx, by, barWidth * ratio, barHeight);
    }
  };

  const drawTrails = (ctx) => {
    if (!filters.trails || !frames?.length) return;
    frame?.playerStates?.forEach((p) => {
      const hist = trails.get(p.steamId);
      if (!hist) return;
      const start = Math.max(0, index - TRAIL_FRAMES);
      const color = p.team === "CT" ? "#68a3e5" : "#e6f13d";
      // dim enemy trails to match the "focus my team" dot fading
      const dim =
        filters.teamFocus && myTeam && p.team && p.team !== myTeam ? 0.35 : 1;
      ctx.strokeStyle = color;
      ctx.lineWidth = 2;
      for (let i = start + 1; i <= index; i++) {
        const a = hist[i - 1];
        const b = hist[i];
        if (!a || !b || !b.alive) continue;
        ctx.globalAlpha = ((i - start) / TRAIL_FRAMES) * 0.4 * dim;
        const pa = transformPos(a.x, a.y);
        const pb = transformPos(b.x, b.y);
        ctx.beginPath();
        ctx.moveTo(pa.x, pa.y);
        ctx.lineTo(pb.x, pb.y);
        ctx.stroke();
      }
    });
    ctx.globalAlpha = 1;
  };

  const drawKillMarkers = (ctx) => {
    if (!kills?.length || !frames?.length) return;
    const first = frames[0].frame;
    kills.forEach((k) => {
      const ki = k.frame - first;
      if (ki < 0) return;
      const age = index - ki;
      if (age < 0 || age > KILL_MARKER_FRAMES) return;
      const { x, y } = transformPos(k.position.x, k.position.y);
      ctx.globalAlpha = 1 - age / KILL_MARKER_FRAMES;
      // With a known team, markers read from my team's perspective: green =
      // an enemy died, red = a teammate died. Otherwise fall back to red.
      let color = "#ff3b30";
      if (myTeam && k.victimTeam) {
        color = k.victimTeam === myTeam ? "#ff3b30" : "#4caf50";
      }
      ctx.strokeStyle = color;
      ctx.lineWidth = 2.5;
      const r = 5;
      ctx.beginPath();
      ctx.moveTo(x - r, y - r);
      ctx.lineTo(x + r, y + r);
      ctx.moveTo(x + r, y - r);
      ctx.lineTo(x - r, y + r);
      ctx.stroke();
    });
    ctx.globalAlpha = 1;
  };

  const drawBomb = (ctx, bomb) => {
    if (!bomb) return;
    if (bomb.planted) {
      const { x, y } = transformPos(bomb.position.x, bomb.position.y);
      const s = 7;
      ctx.fillStyle = "#ff2222";
      ctx.fillRect(x - s / 2, y - s / 2, s, s);
      // expanding pulse ring
      const ph = (frame ? frame.time % 1.2 : 0) / 1.2;
      ctx.strokeStyle = `rgba(255, 90, 0, ${(0.7 * (1 - ph)).toFixed(3)})`;
      ctx.lineWidth = 1.5;
      ctx.beginPath();
      ctx.arc(x, y, 5 + ph * 18, 0, 2 * Math.PI);
      ctx.stroke();
      return;
    }
    if (bomb.carrier) {
      // orange ring + C4 badge on whoever is carrying the bomb
      const { x, y } = transformPos(bomb.position.x, bomb.position.y);
      ctx.save();
      ctx.strokeStyle = "#ff9500";
      ctx.lineWidth = 1.5;
      ctx.beginPath();
      ctx.arc(x, y, 8, 0, 2 * Math.PI);
      ctx.stroke();
      ctx.font = "bold 9px sans-serif";
      ctx.textAlign = "center";
      ctx.fillStyle = "#ff9500";
      ctx.fillText("C4", x, y + 19);
      ctx.restore();
      return;
    }
    // dropped on the ground: blinking orange square (skip if position unknown)
    if (!bomb.position || (bomb.position.x === 0 && bomb.position.y === 0)) {
      return;
    }
    const { x, y } = transformPos(bomb.position.x, bomb.position.y);
    const blink = 0.55 + 0.35 * Math.sin((frame ? frame.time : 0) * 4);
    const s = 7;
    ctx.fillStyle = `rgba(255, 149, 0, ${blink.toFixed(3)})`;
    ctx.fillRect(x - s / 2, y - s / 2, s, s);
    ctx.strokeStyle = "rgba(255, 255, 255, 0.6)";
    ctx.lineWidth = 1;
    ctx.strokeRect(x - s / 2, y - s / 2, s, s);
  };
  const drawFlash = (ctx, flash) => {
    const { x, y } = transformPos(flash.position.x, flash.position.y);

    ctx.fillStyle = `rgba(255, 255, 255, ${flash.power / 100})`;
    ctx.beginPath();
    ctx.arc(x, y, 15, 0, 2 * Math.PI);
    ctx.fill();
  };
  const drawHe = (ctx, he) => {
    const { x, y } = transformPos(he.position.x, he.position.y);

    ctx.fillStyle = `rgba(255, 0, 0, ${he.power / 10})`;
    ctx.beginPath();
    ctx.arc(x, y, 15, 0, 2 * Math.PI);
    ctx.fill();
  };

  const drawSmoke = (ctx, smoke) => {
    const { x, y } = transformPos(smoke.position.x, smoke.position.y);
    const radius = worldRadiusToPixels(180);

    const gradient = ctx.createRadialGradient(x, y, radius * 0.2, x, y, radius);
    gradient.addColorStop(0, "rgba(180, 180, 180, 0.55)");
    gradient.addColorStop(1, "rgba(120, 120, 120, 0.15)");
    ctx.fillStyle = gradient;
    ctx.beginPath();
    ctx.arc(x, y, radius, 0, 2 * Math.PI);
    ctx.fill();
  };

  const drawProjectile = (ctx, projectile) => {
    const { x, y } = transformPos(projectile.position.x, projectile.position.y);
    ctx.fillStyle = "#ffffff";
    ctx.beginPath();
    ctx.arc(x, y, 2, 0, 2 * Math.PI);
    ctx.fill();
  };

  const drawFire = (ctx, fire) => {
    const { x, y } = transformPos(fire.position.x, fire.position.y);
    ctx.fillStyle = "#ff9500ff";
    ctx.beginPath();
    ctx.arc(x, y, 2, 0, 2 * Math.PI);
    ctx.fill();
  };

  const onMouseDown = (e) => {
    dragRef.current = {
      x: e.clientX,
      y: e.clientY,
      offsetX: view.offsetX,
      offsetY: view.offsetY,
    };
    dragMovedRef.current = false;
  };

  const onMouseMove = (e) => {
    const d = dragRef.current;
    if (!d) return;
    if (
      Math.abs(e.clientX - d.x) > 4 ||
      Math.abs(e.clientY - d.y) > 4
    ) {
      dragMovedRef.current = true;
    }
    setView((prev) => ({
      ...prev,
      offsetX: d.offsetX + (e.clientX - d.x),
      offsetY: d.offsetY + (e.clientY - d.y),
    }));
  };

  const onMouseUp = () => {
    dragRef.current = null;
  };

  // Map a world position to canvas pixels, matching the current view
  // transform (focus-zoom or pan/zoom). Used for click-to-focus.
  const toScreen = (worldX, worldY) => {
    const c = transformPos(worldX, worldY);
    if (focusPlayer) {
      const f = frame?.playerStates?.find((p) => p.steamId === focusPlayer);
      if (f) {
        const fp = transformPos(f.position.x, f.position.y);
        return {
          x: displaySize.width / 2 + (c.x - fp.x) * focusZoom,
          y: displaySize.height / 2 + (c.y - fp.y) * focusZoom,
        };
      }
    }
    return {
      x: c.x * view.zoom + view.offsetX,
      y: c.y * view.zoom + view.offsetY,
    };
  };

  const onClick = (e) => {
    if (dragMovedRef.current) return; // that was a pan, not a click
    if (!onSelectPlayer || !frame?.playerStates) return;
    const rect = containerRef.current.getBoundingClientRect();
    const mx = e.clientX - rect.left;
    const my = e.clientY - rect.top;
    let best = null;
    let bestDist = 18 * 18;
    frame.playerStates.forEach((p) => {
      const { x, y } = toScreen(p.position.x, p.position.y);
      const d = (x - mx) * (x - mx) + (y - my) * (y - my);
      if (d < bestDist) {
        bestDist = d;
        best = p;
      }
    });
    if (!best) return;
    onSelectPlayer(best.steamId === focusPlayer ? null : best.steamId);
  };

  const onWheel = (e) => {
    e.preventDefault();
    const factor = e.deltaY < 0 ? 1.1 : 1 / 1.1;
    if (focusPlayer) {
      setFocusZoom((z) => clamp(z * factor, 1, 8));
      return;
    }
    const rect = containerRef.current.getBoundingClientRect();
    const mx = e.clientX - rect.left;
    const my = e.clientY - rect.top;
    setView((prev) => {
      const zoom = clamp(prev.zoom * factor, 1, 8);
      return {
        zoom,
        offsetX: prev.offsetX + (prev.zoom - zoom) * mx,
        offsetY: prev.offsetY + (prev.zoom - zoom) * my,
      };
    });
  };

  const touchDist = (a, b) =>
    Math.hypot(a.clientX - b.clientX, a.clientY - b.clientY);

  const touchMid = (a, b) => ({
    x: (a.clientX + b.clientX) / 2,
    y: (a.clientY + b.clientY) / 2,
  });

  const onTouchStart = (e) => {
    if (e.touches.length === 2) {
      pinchRef.current = {
        mode: "pinch",
        dist: touchDist(e.touches[0], e.touches[1]),
        mid: touchMid(e.touches[0], e.touches[1]),
        zoom: focusPlayer ? focusZoom : view.zoom,
        offsetX: view.offsetX,
        offsetY: view.offsetY,
      };
    } else if (e.touches.length === 1) {
      pinchRef.current = {
        mode: "pan",
        x: e.touches[0].clientX,
        y: e.touches[0].clientY,
        offsetX: view.offsetX,
        offsetY: view.offsetY,
      };
    }
  };

  const onTouchMove = (e) => {
    const g = pinchRef.current;
    if (!g) return;
    const rect = containerRef.current.getBoundingClientRect();
    if (g.mode === "pinch" && e.touches.length === 2) {
      const dist = touchDist(e.touches[0], e.touches[1]);
      const mid = touchMid(e.touches[0], e.touches[1]);
      const mx = mid.x - rect.left;
      const my = mid.y - rect.top;
      const zoom = clamp((g.zoom * dist) / g.dist, 1, 8);
      if (focusPlayer) {
        setFocusZoom(zoom);
      } else {
        setView({
          zoom,
          offsetX: g.offsetX + (g.zoom - zoom) * mx,
          offsetY: g.offsetY + (g.zoom - zoom) * my,
        });
      }
    } else if (g.mode === "pan" && e.touches.length === 1) {
      const dx = e.touches[0].clientX - g.x;
      const dy = e.touches[0].clientY - g.y;
      setView({
        zoom: view.zoom,
        offsetX: g.offsetX + dx,
        offsetY: g.offsetY + dy,
      });
    }
  };

  const onTouchEnd = () => {
    pinchRef.current = null;
  };

  useEffect(() => {
    const canvas = canvasRef.current;
    const context = canvas.getContext("2d");
    context.clearRect(0, 0, canvas.width, canvas.height);

    const focused = focusPlayer
      ? frame?.playerStates?.find((p) => p.steamId === focusPlayer)
      : null;

    context.save();
    if (focused) {
      const { x, y } = transformPos(focused.position.x, focused.position.y);
      context.translate(canvas.width / 2, canvas.height / 2);
      context.scale(focusZoom, focusZoom);
      context.translate(-x, -y);
    } else {
      context.translate(view.offsetX, view.offsetY);
      context.scale(view.zoom, view.zoom);
    }

    if (imgRef.current) {
      const { scale, offsetX, offsetY } = cover();
      context.drawImage(
        imgRef.current,
        offsetX,
        offsetY,
        imgSize.width * scale,
        imgSize.height * scale,
      );
    }
    drawTrails(context);
    // Draw my own dot last so it (and its halo) is never covered by others.
    const players = [...(frame?.playerStates || [])].sort(
      (a, b) =>
        (a.steamId === mySteamId ? 1 : 0) - (b.steamId === mySteamId ? 1 : 0),
    );
    players.forEach((player) => draw(context, player));
    frame?.smokes?.forEach((smoke) => drawSmoke(context, smoke));
    frame?.flashes?.forEach((flash) => drawFlash(context, flash));
    frame?.grenades?.forEach((grenade) => drawProjectile(context, grenade));
    frame?.fires?.forEach((fire) => drawFire(context, fire));
    frame?.hes?.forEach((he) => drawHe(context, he));
    drawKillMarkers(context);
    drawBomb(context, frame?.bombState);

    context.restore();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- draw fns close over state that's already in deps
  }, [
    frame,
    frames,
    kills,
    index,
    map.image,
    imgSize,
    focusPlayer,
    view,
    focusZoom,
    displaySize,
    filters,
    trails,
    deathIndex,
    mySteamId,
    myTeam,
  ]);

  useEffect(() => {
    if (focusPlayer) {
      setView({ zoom: 1, offsetX: 0, offsetY: 0 });
      setFocusZoom(2.5);
    }
  }, [focusPlayer]);

  return (
    <div
      ref={wrapperRef}
      style={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        flex: "1 1 auto",
        minWidth: 0,
        height: "100%",
      }}
    >
      <div
        className="stack"
        style={{ height: displaySize.height, width: displaySize.width }}
        ref={containerRef}
        onWheel={onWheel}
        onMouseDown={onMouseDown}
        onMouseMove={onMouseMove}
        onMouseUp={onMouseUp}
        onMouseLeave={onMouseUp}
        onClick={onClick}
        onTouchStart={onTouchStart}
        onTouchMove={onTouchMove}
        onTouchEnd={onTouchEnd}
      >
        <canvas
          height={displaySize.height}
          width={displaySize.width}
          ref={canvasRef}
          style={{ cursor: "grab" }}
        />
        <KillFeed
          kills={kills}
          frames={frames}
          index={index}
          mySteamId={mySteamId}
          myTeam={myTeam}
        />
        <FocusHud frame={frame} focusPlayer={focusPlayer} mySteamId={mySteamId} />
      </div>
    </div>
  );
};