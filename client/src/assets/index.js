export const getGunFromName = name => {
  try {
    name = name.replace('weapon_', '')
    const gun = require(`./${name}.svg`)
    return gun
  } catch (_) {}
}
