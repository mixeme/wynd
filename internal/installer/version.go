package installer

// WorkshopVersion — версия самой Мастерской: окно, заголовок, свойства exe.
// Своя линия номеров, не версия Wynd (план 45, решение владельца 2026-10-24):
// какой Wynd ставить, Мастерская узнаёт у хранилища (RegistryImage.Latest).
// Поднимать, когда меняется сама Мастерская, вместе с productVersion в
// cmd/wynd-installer/wails.json (их сверяет TestWorkshopVersionMatchesWails).
const WorkshopVersion = "0.1.0"
