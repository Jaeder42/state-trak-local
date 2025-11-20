import { gun } from "../utils/weapons"

export const Weapon = ({weapon}) => {
    const weaponImage = gun[weapon];
    return (
        <img src={weaponImage} />
    )
}
