import React, { useRef, useEffect, useState } from "react";
const anubis = require("../maps/De_anubis_radar.webp");
const ancient = require("../maps/Ancient_Radar.webp");

export const Frame = ({ frame }) => {
  const canvasRef = useRef(null);
  const drawingCanvasRef = useRef(null);
  const [isDrawing, setIsDrawing] = useState(false);
  const [lastPos, setLastPos] = useState({ x: 0, y: 0 });
  const [drawColor, setDrawColor] = useState("#FF0000"); // Set initial color to red
  const height = 500;
  const width = 500;
  const transformPos = (posX, posY) => {
    return {
      x: posX / 10 + width / 2 + 40,
      y: -posY / 10 + height / 2 - 40,
    };
  };
  const draw = (ctx, player) => {
    const { position, alive, team, firing, yaw, name } = player;

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

    ctx.strokeText(name, x + 3, y - 3);
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

  const drawSmoke = (ctx, smoke) => {
    // TODO DRAW THE SMOKE ON THE MAP
    const { x, y } = transformPos(smoke.position.x, smoke.position.y);

    ctx.fillStyle = "#4a4a4a";
    ctx.beginPath();
    ctx.arc(x, y, 15, 0, 2 * Math.PI);
    ctx.fill();
  };

  const drawProjectile = (ctx, projectile) => {
      const { x, y } = transformPos(projectile.position.x, projectile.position.y);
      ctx.fillStyle = "#ffffff";
      ctx.beginPath();
      ctx.arc(x, y, 2, 0, 2 * Math.PI);
      ctx.fill();
  };


  const startDrawing = (e) => {
    setIsDrawing(true);
    setLastPos({ x: e.nativeEvent.offsetX, y: e.nativeEvent.offsetY });
  };

  const drawOnCanvas = (e) => {
    if (!isDrawing) return;
    const ctx = drawingCanvasRef.current.getContext("2d");
    ctx.strokeStyle = drawColor;
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(lastPos.x, lastPos.y);
    ctx.lineTo(e.nativeEvent.offsetX, e.nativeEvent.offsetY);
    ctx.stroke();
    setLastPos({ x: e.nativeEvent.offsetX, y: e.nativeEvent.offsetY });
  };

  const stopDrawing = () => {
    setIsDrawing(false);
  };

  const clearDrawing = () => {
    const ctx = drawingCanvasRef.current.getContext("2d");
    ctx.clearRect(
      0,
      0,
      drawingCanvasRef.current.width,
      drawingCanvasRef.current.height
    );
  };

  useEffect(() => {
    const canvas = canvasRef.current;
    const context = canvas.getContext("2d");
    context.clearRect(0, 0, context.canvas.width, context.canvas.height);
    //Our draw come here
    frame?.playerStates?.map((player) => {
      draw(context, player);
    });
    frame?.smokes?.map((smoke) => {
      drawSmoke(context, smoke);
    });
    frame?.flashes?.map((flash) => {
      drawFlash(context, flash);
    });
    frame?.grenades?.map((grenade) => {
      drawProjectile(context, grenade);
    });
    
    drawBomb(context, frame?.bombState);
  }, [draw]);
  return (
    <div
      style={{ display: "flex", flexDirection: "column", alignItems: "center" }}
    >
      <div>{frame?.frame || "-"}</div>
      <div>{frame?.round || "-"}</div>
      <div>{frame?.ctScore || 0} - {frame?.tScore || 0}</div>
      <div className="stack">
        <div className="map">
          <img height={500} width={500} src={ancient} />
        </div>
        <div className="canvas">
          <canvas height={500} width={500} ref={canvasRef} />
          <canvas
            height={500}
            width={500}
            ref={drawingCanvasRef}
            onMouseDown={startDrawing}
            onMouseMove={drawOnCanvas}
            onMouseUp={stopDrawing}
            onMouseLeave={stopDrawing}
            style={{ position: "absolute", top: 0, left: 0 }}
          />
        </div>
      </div>
      <button className="controls" onClick={clearDrawing}>
        Clear Drawing
      </button>
    </div>
  );
};
