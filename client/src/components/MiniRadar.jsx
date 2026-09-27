import React, { useEffect, useRef, useState } from "react";
import { MAPS, RADAR_NATIVE_SIZE } from "../maps/config";

// Small static radar renderer for analysis panels — same world→pixel
// transform as the playback canvas in Frame.jsx, but fitted to a fixed-size
// square canvas.
//
// dots:  [{ x, y, color, r?, hollow?, ring? }]  hollow = outline only (dead),
//        ring draws a white halo (used for "my" player)
// bombs: [{ x, y }] red squares
export const MiniRadar = ({ mapName, dots = [], bombs = [], size = 170 }) => {
  const canvasRef = useRef(null);
  const [img, setImg] = useState(null);
  const map = MAPS[mapName];

  useEffect(() => {
    if (!map) return undefined;
    const image = new Image();
    image.onload = () => setImg(image);
    image.src = map.image;
    return undefined;
  }, [map]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || !img || !map) return;

    const ctx = canvas.getContext("2d");
    ctx.clearRect(0, 0, size, size);

    const iw = img.naturalWidth;
    const ih = img.naturalHeight;
    const scale = Math.min(size / iw, size / ih);
    const offX = (size - iw * scale) / 2;
    const offY = (size - ih * scale) / 2;
    const nativeScale = RADAR_NATIVE_SIZE / iw;
    const transformPos = (posX, posY) => ({
      x: ((posX - map.posX) / map.scale / nativeScale) * scale + offX,
      y: ((map.posY - posY) / map.scale / nativeScale) * scale + offY,
    });

    ctx.drawImage(img, offX, offY, iw * scale, ih * scale);

    bombs.forEach((b) => {
      const { x, y } = transformPos(b.x, b.y);
      const s = 5;
      ctx.fillStyle = "#ff3b30";
      ctx.fillRect(x - s / 2, y - s / 2, s, s);
    });

    dots.forEach((d) => {
      const { x, y } = transformPos(d.x, d.y);
      const r = d.r ?? 3.5;
      ctx.beginPath();
      ctx.arc(x, y, r, 0, 2 * Math.PI);
      if (d.hollow) {
        ctx.strokeStyle = d.color;
        ctx.lineWidth = 1.5;
        ctx.stroke();
      } else {
        ctx.fillStyle = d.color;
        ctx.fill();
      }
      if (d.ring) {
        ctx.strokeStyle = "#ffffff";
        ctx.lineWidth = 1;
        ctx.beginPath();
        ctx.arc(x, y, r + 2, 0, 2 * Math.PI);
        ctx.stroke();
      }
    });
  }, [img, map, dots, bombs, size]);

  if (!map) {
    return (
      <div className="mini-radar-missing" style={{ width: size, height: size }}>
        no radar for {mapName}
      </div>
    );
  }
  return (
    <canvas ref={canvasRef} width={size} height={size} className="mini-radar" />
  );
};