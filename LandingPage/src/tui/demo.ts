/** The made-up transfer the whole page walks through. */

export const ROOM_CODE = 'kiraz-liman-42'

/** Two files, so the transfer has something to count through. */
export const DEMO_FILES = [
  { name: 'tatil-fotograflari.zip', size: 486_539_264 },
  { name: 'dugun-videosu.mp4', size: 1_395_864_371 },
]

export const DEMO_TOTAL = DEMO_FILES.reduce((a, f) => a + f.size, 0)

export const OUT_DIR = '/home/mehmet/İndirilenler/FileTransferilla'
