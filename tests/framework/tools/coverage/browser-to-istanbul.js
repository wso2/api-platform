#!/usr/bin/env node

const fs = require('node:fs')
const path = require('node:path')
const v8toIstanbul = require('v8-to-istanbul')
const libCoverage = require('istanbul-lib-coverage')
const libReport = require('istanbul-lib-report')
const reports = require('istanbul-reports')
const { minimatch } = require('minimatch')

function usage() {
  console.error('usage: browser-to-istanbul.js <input-root> <output-root> <repo-root> [--source-root path] [--inventory-root path] [--include pattern] [--exclude pattern]')
  process.exit(2)
}

const args = process.argv.slice(2)
if (args.length < 3) usage()
const inputRoot = path.resolve(args.shift())
const outputRoot = path.resolve(args.shift())
const repoRoot = path.resolve(args.shift())
const sourceRoots = []
const inventoryRoots = []
const includes = []
const excludes = []
for (let i = 0; i < args.length; i += 2) {
  if (!args[i + 1] || !['--source-root', '--inventory-root', '--include', '--exclude'].includes(args[i])) usage()
  const values = args[i] === '--source-root' ? sourceRoots : args[i] === '--inventory-root' ? inventoryRoots : args[i] === '--include' ? includes : excludes
  values.push(args[i + 1])
}
const absoluteSourceRoots = sourceRoots.map((root) => path.resolve(repoRoot, root))
const absoluteInventoryRoots = inventoryRoots.map((root) => path.resolve(repoRoot, root))
function filesUnder(root, fileName) {
  const result = []
  for (const entry of fs.readdirSync(root, { withFileTypes: true })) {
    const full = path.join(root, entry.name)
    if (entry.isDirectory()) result.push(...filesUnder(full, fileName))
    else if (entry.isFile() && entry.name === fileName) result.push(full)
  }
  return result.sort()
}

function sourceFilesUnder(root) {
  const result = []
  if (!fs.existsSync(root)) return result
  for (const entry of fs.readdirSync(root, { withFileTypes: true })) {
    const full = path.join(root, entry.name)
    if (entry.isDirectory()) result.push(...sourceFilesUnder(full))
    else if (entry.isFile() && ['.js', '.jsx', '.ts', '.tsx'].includes(path.extname(entry.name))) result.push(full)
  }
  return result.sort()
}

function relativeSource(file) {
  return path.relative(repoRoot, file).split(path.sep).join('/')
}

function resolveSource(file) {
  const absolute = path.resolve(file)
  const normalized = absolute.split(path.sep).join('/')
  const sourceIndex = normalized.lastIndexOf('/src/')
  if (sourceIndex >= 0) {
    const suffix = normalized.slice(sourceIndex + '/src/'.length)
    for (const root of absoluteSourceRoots) {
      const candidate = path.join(root, suffix)
      if (fs.existsSync(candidate)) return candidate
    }
  }
  if (absolute === repoRoot || absolute.startsWith(`${repoRoot}${path.sep}`)) return absolute
  return absolute
}

function resolveIstanbulSource(file) {
  const normalized = String(file).replaceAll('\\', '/')
  for (const root of absoluteSourceRoots) {
    const marker = `/${path.basename(root)}/`
    const index = normalized.lastIndexOf(marker)
    if (index >= 0) {
      const candidate = path.join(root, normalized.slice(index + marker.length))
      if (fs.existsSync(candidate)) return candidate
    }
    const sourceMarker = '/src/'
    const sourceIndex = normalized.lastIndexOf(sourceMarker)
    if (sourceIndex >= 0) {
      const candidate = path.join(root, normalized.slice(sourceIndex + sourceMarker.length))
      if (fs.existsSync(candidate)) return candidate
    }
  }
  return path.isAbsolute(file) ? file : path.resolve(repoRoot, file)
}

function selected(file) {
  const rel = relativeSource(file)
  if (includes.length > 0 && !includes.some((pattern) => minimatch(rel, pattern, { dot: true }))) return false
  return !excludes.some((pattern) => minimatch(rel, pattern, { dot: true }))
}

async function main() {
  const coverageMap = libCoverage.createCoverageMap()
  const istanbulReports = filesUnder(inputRoot, 'raw-istanbul.json')
  for (const reportPath of istanbulReports) {
    const report = JSON.parse(fs.readFileSync(reportPath, 'utf8'))
    for (const [file, fileCoverage] of Object.entries(report)) {
      const absolute = resolveIstanbulSource(file)
      if (!selected(absolute)) continue
      coverageMap.addFileCoverage({ ...fileCoverage, path: absolute })
    }
  }
  if (istanbulReports.length === 0) {
    throw new Error('no source-level browser coverage reports found (raw-istanbul.json); CDP/V8 fallback is disabled')
  }
  const coveredFiles = new Set(coverageMap.files())
  for (const sourceRoot of absoluteInventoryRoots) {
    for (const file of sourceFilesUnder(sourceRoot)) {
      if (!selected(file) || coveredFiles.has(file)) continue
      const source = fs.readFileSync(file, 'utf8')
      const converter = v8toIstanbul(file, 0, { source })
      await converter.load()
      converter.applyCoverage([{
        functionName: '(uncovered-source)',
        ranges: [{ startOffset: 0, endOffset: source.length, count: 0 }],
        isBlockCoverage: true,
      }])
      for (const [convertedFile, fileCoverage] of Object.entries(converter.toIstanbul())) {
        coverageMap.addFileCoverage({ ...fileCoverage, path: convertedFile })
      }
    }
  }
  if (coverageMap.files().length === 0) {
    throw new Error('source-level browser coverage reports contained no selected source files')
  }

  fs.mkdirSync(outputRoot, { recursive: true })
  fs.writeFileSync(path.join(outputRoot, 'coverage-final.json'), JSON.stringify(coverageMap.toJSON(), null, 2))
  fs.writeFileSync(path.join(outputRoot, 'summary.json'), JSON.stringify(coverageMap.getCoverageSummary().toJSON(), null, 2))
  const context = libReport.createContext({ dir: outputRoot, coverageMap })
  reports.create('lcovonly', { projectRoot: repoRoot }).execute(context)
  reports.create('html', { projectRoot: repoRoot }).execute(context)
  console.log(`Browser JavaScript coverage: ${coverageMap.files().length} source files from ${istanbulReports.length} Istanbul reports`)
}

main().catch((error) => {
  console.error(`browser coverage: ${error.message}`)
  process.exit(1)
})
