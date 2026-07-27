# Rule: JavaScript (Node.js/Express) File Access Security Standards

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing JavaScript (`.js`) code in `portals/developer-portal` that handles file uploads (multer), archive extraction (unzipper), file reads from disk, or byte-stream processing from user input. JS counterpart to `file-access.md` (Go). The project uses **multer memory storage** — uploaded bytes live in `req.file.buffer`; this rule reinforces that pattern and extends it to ZIP handling, path safety, and configurable stream limits sourced from the `DP_*` config system.

## Directives

1. **Path traversal prevention.** Any file read/write derived from user input must resolve to an absolute path via `path.resolve()` and be verified against the expected root with a trailing-separator prefix check (`safeJoin` pattern below) — reject `..`, null bytes, and URL-encoded traversal variants (`%2e%2e`, `%2f`, `%00`) before any `fs.*` call. Never build a path with raw concatenation; `path.join(root, path.basename(userInput))` is the bare minimum.
2. **Filename only in storage.** Persist only `path.basename()` of a filename to Sequelize models/DB columns — never a full or relative path. Re-derive the real path server-side at read time by joining the stored bare filename with a server-controlled root; never use the stored value as-is in an `fs.readFile`-style call.
3. **In-memory processing, no intermediate disk writes.** Keep working from `req.file.buffer` (multer `memoryStorage()`) via `Buffer`/`stream.Readable.from()`/`unzipper.Open.buffer()`. Only fall back to `fs.writeFile`/`fs.mkdtemp` when a third-party library strictly requires a path, using an OS-generated name (`crypto.randomUUID()`), never `req.body.filename`, and always clean up with `fs.unlink` in a `finally` block.
4. **ZIP/archive handling — zip-slip protection.** Validate every entry's `path` before extracting: reject absolute paths, `..`, and null bytes with a thrown error (never a silent skip). For single-file extraction, match the exact normalized entry path (not basename) and `entry.autodrain()` everything else. Cap total entry count and apply a decompression-ratio guard (compressed size × ratio, capped by `maxUploadBytes`) to stop zip bombs.
5. **Configurable stream size limits.** Source multer's `limits.fileSize`, `maxZipEntries`, and `maxZipRatio` from the `DP_*` config system (`DP_MAX_UPLOAD_BYTES`, `DP_MAX_ZIP_ENTRIES`, `DP_MAX_ZIP_RATIO`) with a safe default, never hardcoded. On overflow, respond `413` with a generic message — never echo the configured limit.
6. **Content-type allowlisting; no dynamic code execution on user input.** Content-sniff the actual buffer (`fileTypeFromBuffer`) against an explicit allowlist — never trust `req.file.mimetype` or the extension alone. SVG is XML, not "just an image": if supported, it must go through the sanitization + `Content-Disposition: attachment` hardening in `js-output-encoding-xss.md`, never stored/served unmodified. Never pass user-supplied content to `eval()`, `new Function()`, `vm.runInThisContext()`, or a template engine's compile step (SSTI ≈ RCE) — this applies even to admin/publisher-only upload features, since authorization gates *who* can reach a feature, not whether executing its input is safe.

## Example

```js
// utils/fileSafety.js — path containment + zip-slip-safe single-entry extraction
const path = require('path');

function safeJoin(root, userInput) {
  const sanitised = userInput.replace(/\0/g, '').replace(/%2e/gi, '.').replace(/%2f/gi, '/');
  const resolved = path.resolve(root, path.basename(sanitised)); // basename strips all dirs
  const rootWithSep = path.resolve(root) + path.sep;
  if (!resolved.startsWith(rootWithSep)) throw new Error('Path escapes root directory');
  return resolved;
}

async function extractSingleEntry(zipBuffer, targetEntryName, cfg) {
  const unzipper = require('unzipper');
  const zip = await unzipper.Open.buffer(zipBuffer);
  if (zip.files.length > cfg.maxZipEntries) {
    throw Object.assign(new Error('too many entries'), { statusCode: 413 });
  }

  let found = null;
  for (const entry of zip.files) {
    if (path.isAbsolute(entry.path) || entry.path.includes('..') || entry.path.includes('\0')) {
      throw Object.assign(new Error('unsafe entry path'), { statusCode: 422 });
    }
    if (path.posix.normalize(entry.path) !== path.posix.normalize(targetEntryName)) {
      await entry.autodrain(); // discard non-target entries, never extract them
      continue;
    }
    found = entry;
  }
  if (!found) throw Object.assign(new Error('entry not found'), { statusCode: 404 });

  const maxDecompressed = found.compressedSize
    ? Math.min(found.compressedSize * cfg.maxZipRatio, cfg.maxUploadBytes) // tighter of ratio vs absolute cap
    : cfg.maxUploadBytes;

  const chunks = [];
  let total = 0;
  await new Promise((resolve, reject) => {
    const s = found.stream();
    s.on('data', (c) => {
      total += c.length;
      if (total > maxDecompressed) { s.destroy(); return reject(Object.assign(new Error('too large'), { statusCode: 413 })); }
      chunks.push(c);
    });
    s.on('end', resolve);
    s.on('error', reject);
  });
  return Buffer.concat(chunks);
}

// utils/uploadContentValidation.js — content-sniff, never trust req.file.mimetype
const { fileTypeFromBuffer } = require('file-type');
const ALLOWED_UPLOAD_TYPES = new Set(['image/png', 'image/jpeg']); // svg excluded — see js-output-encoding-xss.md

async function validateUploadContent(buffer) {
  const detected = await fileTypeFromBuffer(buffer);
  if (!detected || !ALLOWED_UPLOAD_TYPES.has(detected.mime)) {
    throw Object.assign(new Error('unsupported content type'), { statusCode: 422 });
  }
  return detected.mime;
}
```

> **Verification Checklist before outputting code:**
> * Is every path built via `path.resolve()`/`safeJoin` and checked against the root with a trailing-separator prefix? (If no, fix it.)
> * Is only `path.basename()` stored in the DB, with the real path re-derived server-side at read time? (If a full path is stored, strip it.)
> * Is processing done from `req.file.buffer` with no intermediate `fs.writeFile`/`fs.mkdtemp`, or a cleaned-up last resort if unavoidable? (If disk writes linger, add `finally` cleanup.)
> * Is every ZIP entry validated for `..`/absolute paths/null bytes before extraction, with count and decompression-ratio limits enforced? (If not, add both.)
> * Are multer/zip size limits sourced from `DP_*` config with a safe default, and does overflow return a generic `413`? (If hardcoded or leaks the limit, fix it.)
> * Is upload content sniffed (`fileTypeFromBuffer`) against an allowlist rather than trusting `mimetype`/extension, and is SVG handled per `js-output-encoding-xss.md`? (If trusted as-is, add sniffing.)
> * Does any path compile/execute user-supplied content (`eval`, `new Function`, `vm`, template compile) instead of only ever displaying it via escaping interpolation? (If yes, this is SSTI/RCE — remove it.)
