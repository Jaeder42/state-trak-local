import React, { useState, useEffect } from "react";
import { Frame } from "./Frame.jsx";

export const Games = () => {
  const [output, setOutput] = useState({});
  const [index, setIndex] = useState(0);
  const [round, setRound] = useState(0);
  const [loading, setLoading] = useState(false);
  const [playing, setPlaying] = useState(false);
  const [intervalID, setIntervalID] = useState(null);
  const [metaData, setMetaData] = useState({});
  const fetchMeta = async () => {
    try {
      const meta = await fetch("http://localhost:3001/output");
      const json = await meta.json();
      setMetaData(json);
      console.log({ json });
    } catch (err) {
      console.error(err);
    }
  };
  const fetchRound = async () => {
    try {
      const roundData = await fetch(`http://localhost:3001/${round}`);
      setOutput(await roundData.json());
      setIndex(0);
      setPlaying(false);
    } catch (e) {
      console.log(e);
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchMeta();
  }, []);

  useEffect(() => {
    fetchRound();
  }, [round]);

  useEffect(() => {
    if (playing) {
      const inter = setInterval(() => tick(), 16);
      setIntervalID(inter);
    } else {
      clearInterval(intervalID);
    }
  }, [playing]);

  const tick = () => {
    if (index < output.frames?.length - 1) {
      setIndex((index) => +index + 1);
    }
  };

  const tooglePlay = (play) => {
    setPlaying(play);
  };
  const onChange = (e) => {
    setIndex(e.target.value);
  };
  return (
    <div>
      <h1>{metaData.map}</h1>
      {loading ? (
        <div>...loading</div>
      ) : (
        <>
          <div
            style={{
              backgroundColor: "transparent",
            }}
          >
            <Frame
              frame={
                output.frames
                  ? output.frames[Math.min(index, output.frames?.length - 1)]
                  : null
              }
            />
          </div>
          <div className="controls ">
            <input
              type="range"
              min="0"
              max={`${output.frames?.length - 1 ?? 0}`}
              value={index}
              onChange={onChange}
            />
            <div className="container">
              <div className="row">
                <button onClick={() => setRound(Math.max(0, round - 1))}>
                  -
                </button>
                <h2>{round}</h2>
                <button onClick={() => setRound(round + 1)}>+</button>
                <button onClick={() => tooglePlay(!playing)}>play</button>
              </div>
            </div>
          </div>
          {/* <p>{
            output.frames
                  ? JSON.stringify(output.frames[Math.min(index, output.frames?.length - 1)]?.playerStates[0])
                  : 'null'
          }</p> */}
          <p>{JSON.stringify(metaData)}</p>
        </>
      )}
    </div>
  );
};
