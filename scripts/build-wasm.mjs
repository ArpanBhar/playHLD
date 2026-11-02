import { execFileSync } from 'node:child_process'
import { copyFileSync, mkdirSync, writeFileSync } from 'node:fs'
import { join, resolve } from 'node:path'

const root = resolve(import.meta.dirname, '..')
const cache = join(root, '.cache')
const env = {
  ...process.env,
  APPDATA: join(cache, 'appdata'),
  LOCALAPPDATA: join(cache, 'localappdata'),
  XDG_CONFIG_HOME: join(cache, 'config'),
  GOCACHE: join(cache, 'go-build'),
  GOOS: 'js',
  GOARCH: 'wasm',
  GOTOOLCHAIN: 'local',
}
const publicDir = join(root, 'frontend', 'public', 'wasm')
const sourceDir = join(root, 'frontend', 'src', 'simulation')
mkdirSync(publicDir, { recursive: true })
mkdirSync(sourceDir, { recursive: true })
mkdirSync(env.APPDATA, { recursive: true })
mkdirSync(env.LOCALAPPDATA, { recursive: true })
mkdirSync(env.XDG_CONFIG_HOME, { recursive: true })
mkdirSync(env.GOCACHE, { recursive: true })

execFileSync('go', ['telemetry', 'off'], { cwd: root, env, stdio: 'inherit' })
const goRoot = execFileSync('go', ['env', 'GOROOT'], { cwd: root, env, encoding: 'utf8' }).trim()
const toolchain = execFileSync('go', ['version'], { cwd: root, env, encoding: 'utf8' }).trim()
execFileSync('go', ['build', '-o', join(publicDir, 'simulator.wasm'), './wasm/cmd/simulator'], { cwd: root, env, stdio: 'inherit' })
copyFileSync(join(goRoot, 'lib', 'wasm', 'wasm_exec.js'), join(sourceDir, 'wasm_exec.js'))
writeFileSync(join(publicDir, 'manifest.json'), JSON.stringify({ engineVersion: '0.6.0', schemaVersion: 3, toolchain }, null, 2) + '\n')
console.log(`Built Go WASM with ${toolchain}`)
