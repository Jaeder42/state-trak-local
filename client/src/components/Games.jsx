import React, { useState, useEffect } from "react";
import { Frame } from "./Frame";

export const Games = () => {
  const [output, setOutput] = useState({});
  const [index, setIndex] = useState(0);
  const [round, setRound] = useState(2);
  const [loading, setLoading] = useState(false);
  useEffect(() => {
    try {
    const roundData = require(`./data/output/${round}.json`);
    console.log({roundData});
    setOutput(roundData);
    setIndex(0);
    } catch (e) {
      console.log(e);
    }
  }, [round]);

  const onChange = (e) => {
    setIndex(e.target.value);
  };
  return (
    <div>
      <h1>{output.map}</h1>
      {loading ? (
        <div>...loading</div>
      ) : (
        <>
          <div style={{ backgroundColor: "transparent" }}>
            <Frame frame={output.frames ? output.frames[index] : null} />
          </div>
          <div className="controls">
            <input
              type="range"
              min="0"
              max={output.frames?.length - 1 ?? 0}
              value={index}
              onChange={onChange}
            />
            <div onClick={() => setRound(Math.max(0, round - 1))}>-</div>
            <h2>{round}</h2>
            <div onClick={() => setRound(round + 1)}>+</div>
          </div>
        </>
      )}
    </div>
  );
};
