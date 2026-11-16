// Native-only desktop packaging. Uses the pinned desktop/go.mod; no global Wails CLI.
import { spawnSync } from 'node:child_process'
import { cpSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const desktop = join(root, 'desktop')
const mode = process.argv[2] ?? 'build'
if (!['build', 'stage', 'test'].includes(mode)) throw new Error('Usage: node scripts/desktop-build.mjs [build|stage|test]')
const platform = process.platform
const arch = process.arch === 'x64' ? 'amd64' : process.arch
const goos = { linux: 'linux', darwin: 'darwin', win32: 'windows' }[platform]
function run(command, args, cwd = root, env = {}) {
  const result = spawnSync(command, args, {
    cwd, stdio: 'inherit', env: { ...process.env, ...env },
    // Windows batch launchers require cmd.exe; only the fixed npm build command uses it.
    shell: command === 'npm.cmd',
  })
  if (result.error) throw result.error
  if (result.status !== 0) throw new Error(`${command} failed (${result.status})`)
}
if (mode === 'test') {
  // Native entrypoint has the desktop tag; core tests need no display or GTK.
  run('go', ['test', './...'], desktop)
  process.exit(0)
}
if (!goos || !['amd64', 'arm64'].includes(arch)) throw new Error('Unsupported native platform/architecture')
if ((process.env.GOOS && process.env.GOOS !== goos) || (process.env.GOARCH && process.env.GOARCH !== arch)) {
  throw new Error('Cross-compilation is not supported by this packaging script; use a native runner')
}
run(platform === 'win32' ? 'npm.cmd' : 'npm', ['run', 'build'], join(root, 'web'))
const assets = join(desktop, 'assets', 'web')
// Read before removing stale staging so failed frontend builds do not destroy it.
readFileSync(join(root, 'web', 'dist', 'index.html'))
rmSync(assets, { recursive: true, force: true })
cpSync(join(root, 'web', 'dist'), assets, { recursive: true })
if (mode === 'stage') process.exit(0)

const version = readFileSync(join(root, 'VERSION'), 'utf8').trim().replace(/^v/, '')
if (!/^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$/.test(version)) throw new Error('Invalid VERSION')
const name = `picofarm-desktop-${version}-${goos}-${arch}`
const output = join(root, 'bin', 'desktop')
const work = join(output, name)
rmSync(work, { recursive: true, force: true })
mkdirSync(work, { recursive: true })
const executable = platform === 'win32' ? 'PicoFarm.exe' : 'PicoFarm'
const app = join(work, 'PicoFarm.app')
const binaryDir = platform === 'darwin' ? join(app, 'Contents', 'MacOS') : work
mkdirSync(binaryDir, { recursive: true })
const ldflags = ['-s', '-w', `-X github.com/Brook-sys/picofarm/internal/version.Version=${version}`,
  '-X github.com/Brook-sys/picofarm/internal/version.BuiltBy=desktop']
if (platform === 'win32') ldflags.push('-H=windowsgui')
const env = { GOOS: goos, GOARCH: arch, CGO_ENABLED: platform === 'win32' ? '0' : '1' }
if (platform === 'darwin') Object.assign(env, {
  MACOSX_DEPLOYMENT_TARGET: '12.0', CGO_CFLAGS: '-mmacosx-version-min=12.0', CGO_LDFLAGS: '-mmacosx-version-min=12.0',
})
run('go', ['build', '-mod=readonly', '-trimpath', '-buildvcs=false', '-tags', 'desktop,production',
  '-ldflags', ldflags.join(' '), '-o', join(binaryDir, executable), '.'], desktop, env)
let notes = 'PicoFarm desktop preview. No publisher certificate, notarization, installer, or automatic updates.\n'
if (platform === 'darwin') {
  const resources = join(app, 'Contents', 'Resources')
  const iconset = join(work, 'PicoFarm.iconset')
  mkdirSync(resources, { recursive: true })
  mkdirSync(iconset, { recursive: true })
  for (const size of [16, 32, 128, 256, 512]) {
    for (const scale of [1, 2]) {
      run('sips', ['-z', String(size * scale), String(size * scale), join(root, 'web/public/picofarm_logo.png'),
        '--out', join(iconset, `icon_${size}x${size}${scale === 2 ? '@2x' : ''}.png`)])
    }
  }
  run('iconutil', ['-c', 'icns', iconset, '-o', join(resources, 'PicoFarm.icns')])
  rmSync(iconset, { recursive: true })
  const shortVersion = version.split(/[-+]/)[0]
  writeFileSync(join(app, 'Contents', 'Info.plist'), `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleName</key><string>PicoFarm</string>
<key>CFBundleDisplayName</key><string>PicoFarm</string>
<key>CFBundleIdentifier</key><string>com.brooksys.picofarm</string>
<key>CFBundleExecutable</key><string>PicoFarm</string>
<key>CFBundlePackageType</key><string>APPL</string>
<key>CFBundleIconFile</key><string>PicoFarm.icns</string>
<key>CFBundleShortVersionString</key><string>${shortVersion}</string>
<key>CFBundleVersion</key><string>${shortVersion}</string>
<key>LSMinimumSystemVersion</key><string>12.0</string>
<key>NSHighResolutionCapable</key><true/>
<key>NSLocalNetworkUsageDescription</key><string>Connect to printers and print services on your local network.</string>
</dict></plist>
`)
  run('plutil', ['-lint', join(app, 'Contents', 'Info.plist')])
  // Ad-hoc signing is needed for native arm64; this is NOT a trusted publisher signature.
  run('codesign', ['--force', '--deep', '--sign', '-', app])
  run('codesign', ['--verify', '--deep', '--strict', app])
  notes += 'macOS: ad-hoc signed only; Gatekeeper may block it. Requires macOS 12+; not notarized.\n'
} else if (platform === 'linux') {
  notes += 'Linux: built on Ubuntu 24.04 amd64, dynamically linked; NOT a portable/static AppImage.\nRequires compatible glibc, GTK4, WebKitGTK 6.0, libsoup3 and X11 libraries.\nUbuntu 24.04 runtime packages: libgtk-4-1 libwebkitgtk-6.0-4 libsoup-3.0-0 libx11-6.\nA graphical desktop is required; older distributions are not supported by this artifact.\n'
} else {
  notes += 'Windows: requires WebView2 Evergreen Runtime. Unsigned executable; SmartScreen may warn.\n'
}
writeFileSync(join(work, 'README.txt'), notes)
if (platform === 'linux') {
  run('tar', ['-czf', join(output, `${name}.tar.gz`), '-C', output, name])
} else if (platform === 'darwin') {
  run('ditto', ['-c', '-k', '--sequesterRsrc', '--keepParent', work, join(output, `${name}.zip`)])
} else {
  const quote = value => `'${value.replaceAll("'", "''")}'`
  run('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command',
    `Compress-Archive -LiteralPath ${quote(work)} -DestinationPath ${quote(join(output, `${name}.zip`))} -Force`])
}
console.log(`Desktop artifact: ${join(output, name)}.${platform === 'linux' ? 'tar.gz' : 'zip'}`)
