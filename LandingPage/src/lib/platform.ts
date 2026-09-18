import type { PlatformId } from './release'

/**
 * Best guess at the visitor's OS, used only to put their download first.
 * Every other build stays one click away, so a wrong guess costs nothing.
 *
 * Apple stopped shipping the CPU in the user agent, so a Mac is assumed
 * to be Apple Silicon — true for everything sold since late 2020 — and
 * the Intel build sits right next to it.
 */
export function detectPlatform(): PlatformId | null {
  if (typeof navigator === 'undefined') return null
  const ua = navigator.userAgent
  if (/Android/i.test(ua)) return null
  if (/Windows|Win32|Win64/i.test(ua)) return 'windows'
  if (/Mac OS X|Macintosh/i.test(ua)) {
    return navigator.maxTouchPoints > 1 ? null : 'macos-arm64'
  }
  if (/Linux|X11/i.test(ua)) return 'linux'
  return null
}
