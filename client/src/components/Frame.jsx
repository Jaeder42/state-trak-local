import React, { useRef, useEffect, useState } from "react";
import { MAPS, RADAR_NATIVE_SIZE } from "../maps/config";

const clamp = (v, min, max) => Math.min(max, Math.max(min, v));

export const Frame = ({ frame, mapName, focusPlayer, filters }) => {
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

  const draw = (ctx, player) => {
    const { position, alive, team, firing, yaw, name, health } = player;

    ctx.fillStyle = team === "CT" ? "#68a3e5" : "#e6f13d";
    if (!alive) {
      ctx.fillStyle = "#515151";
    }
    var rad = (90 - yaw) * (Math.PI / 180);
    var { x, y } = transformPos(position.x, position.y);

    ctx.save();
    ctx.strokeStyle = "#ffffff";
    if (!alive) {
      ctx.strokeStyle = "#939393";
    }

    if (filters.names) {
      ctx.strokeText(name, x + 3, y - 3);
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

  const drawBomb = (ctx, bomb) => {
    if (bomb && bomb.planted) {
      ctx.fillStyle = "#FF0000";
      const { x, y } = transformPos(bomb.position.x, bomb.position.y);
      ctx.beginPath();
      ctx.rect(x, y, 7, 7);
      ctx.fill();
    }
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
  };

  const onMouseMove = (e) => {
    const d = dragRef.current;
    if (!d) return;
    setView((prev) => ({
      ...prev,
      offsetX: d.offsetX + (e.clientX - d.x),
      offsetY: d.offsetY + (e.clientY - d.y),
    }));
  };

  const onMouseUp = () => {
    dragRef.current = null;
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
    frame?.playerStates?.forEach((player) => draw(context, player));
    frame?.smokes?.forEach((smoke) => drawSmoke(context, smoke));
    frame?.flashes?.forEach((flash) => drawFlash(context, flash));
    frame?.grenades?.forEach((grenade) => drawProjectile(context, grenade));
    frame?.fires?.forEach((fire) => drawFire(context, fire));
    frame?.hes?.forEach((he) => drawHe(context, he));
    drawBomb(context, frame?.bombState);

    context.restore();
  }, [frame, map.image, imgSize, focusPlayer, view, focusZoom, displaySize, filters]);

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
      </div>
    </div>
  );
};
