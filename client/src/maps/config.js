import ancient from "./de_ancient_radar.png";
import anubis from "./de_anubis_radar.png";
import dust2 from "./de_dust2_radar.png";
import inferno from "./de_inferno_radar.png";
import mirage from "./de_mirage_radar.png";
import nuke from "./de_nuke_radar.png";
import overpass from "./de_overpass_radar.png";
import vertigo from "./de_vertigo_radar.png";
import train from "./de_train_radar.png";

export const RADAR_NATIVE_SIZE = 1024;

export const MAPS = {
  de_ancient: { image: ancient, posX: -2953, posY: 2164, scale: 5, name: "Ancient" },
  de_anubis: { image: anubis, posX: -2796, posY: 3328, scale: 5.22, name: "Anubis" },
  de_dust2: { image: dust2, posX: -2476, posY: 3239, scale: 4.4, name: "Dust II" },
  de_inferno: { image: inferno, posX: -2087, posY: 3870, scale: 4.9, name: "Inferno" },
  de_mirage: { image: mirage, posX: -3230, posY: 1713, scale: 5, name: "Mirage" },
  de_nuke: { image: nuke, posX: -3453, posY: 2887, scale: 7, name: "Nuke" },
  de_overpass: { image: overpass, posX: -4831, posY: 1781, scale: 5.2, name: "Overpass" },
  de_vertigo: { image: vertigo, posX: -3168, posY: 1762, scale: 4, name: "Vertigo" },
  de_train: { image: train, posX: -2308, posY: 2078, scale: 4.082077, name: "Train" },
};

export const mapDisplayName = (mapName) => {
  const m = MAPS[mapName];
  if (m) return m.name;
  if (!mapName) return "";
  return mapName
    .replace(/^de_/, "")
    .split("_")
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(" ");
};
