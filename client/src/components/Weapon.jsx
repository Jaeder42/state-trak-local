import { gun } from "../utils/weapons";

export const Weapon = ({ weapon }) => {
  const weaponImage = gun[weapon];
  if (!weaponImage) {
    return <span className="weapon-fallback">{weapon}</span>;
  }
  return <img className="weapon-icon" src={weaponImage} alt={weapon} />;
};
