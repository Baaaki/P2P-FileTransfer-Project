import { useEffect, useState } from 'react'

export const REPO = 'Baaaki/P2P-FileTransfer-Project'
export const REPO_URL = `https://github.com/${REPO}`
export const RELEASES_URL = `${REPO_URL}/releases`

export type Asset = { name: string; url: string; size: number }
export type Release = {
  tag: string
  publishedAt: string | null
  assets: Asset[]
  checksums: string | null
}

/**
 * The archive names come from .goreleaser.yaml:
 *   filetransferilla_<version>_<os>_<arch>[.tar.gz|.zip]
 * They carry the version, so a link cannot be hardcoded — we look the
 * current release up instead, and fall back to the releases page when
 * there is no published build yet (or GitHub rate-limits the request).
 */
export type PlatformId = 'windows' | 'macos-arm64' | 'macos-x86_64' | 'linux'

const SUFFIX: Record<PlatformId, RegExp> = {
  windows: /_windows_x86_64\.zip$/i,
  'macos-arm64': /_macOS_arm64\.tar\.gz$/i,
  'macos-x86_64': /_macOS_x86_64\.tar\.gz$/i,
  linux: /_linux_x86_64\.tar\.gz$/i,
}

export function assetFor(release: Release | null, id: PlatformId): Asset | null {
  return release?.assets.find((a) => SUFFIX[id].test(a.name)) ?? null
}

/** What the file will be called once a release exists. */
export function exampleName(id: PlatformId, version = '<sürüm>'): string {
  const tail: Record<PlatformId, string> = {
    windows: `windows_x86_64.zip`,
    'macos-arm64': `macOS_arm64.tar.gz`,
    'macos-x86_64': `macOS_x86_64.tar.gz`,
    linux: `linux_x86_64.tar.gz`,
  }
  return `filetransferilla_${version}_${tail[id]}`
}

export function useLatestRelease(): { release: Release | null; loading: boolean } {
  const [release, setRelease] = useState<Release | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const ac = new AbortController()
    fetch(`https://api.github.com/repos/${REPO}/releases/latest`, {
      signal: ac.signal,
      headers: { Accept: 'application/vnd.github+json' },
    })
      .then((r) => (r.ok ? r.json() : null))
      .then((json) => {
        if (!json?.tag_name) return
        const assets: Asset[] = (json.assets ?? []).map(
          (a: { name: string; browser_download_url: string; size: number }) => ({
            name: a.name,
            url: a.browser_download_url,
            size: a.size,
          }),
        )
        setRelease({
          tag: json.tag_name,
          publishedAt: json.published_at ?? null,
          assets,
          checksums: assets.find((a) => a.name === 'checksums.txt')?.url ?? null,
        })
      })
      .catch(() => {
        /* offline, rate-limited or no release yet — the fallback covers it */
      })
      .finally(() => setLoading(false))
    return () => ac.abort()
  }, [])

  return { release, loading }
}
