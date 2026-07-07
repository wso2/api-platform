# Rule: JavaScript (Node.js/Express) File Access Security Standards

## Context & Scope

Apply this rule whenever writing, refactoring, or reviewing JavaScript (`.js`) code in `portals/developer-portal` that handles file uploads (multer), archive extraction (unzipper), file reads from disk, or byte-stream processing from user input. This is the JavaScript counterpart to `file-access.md` (Go). The project uses **multer memory storage** — uploaded bytes live in `req.file.buffer`; this rule reinforces that pattern and extends it to ZIP handling, path safety, and configurable stream limits sourced from the `DP_*` config system.

---

## Directives

### 1. Path Traversal Prevention (Clean Path Validation)

* **Enforce Containment:** Any file read or write derived from user input must resolve to an absolute path with `path.resolve()` and verify the result starts with the expected root directory (with a trailing separator to prevent partial prefix matches).
* **Reject Traversal Sequences:** Strip or reject input containing `..`, `\x00` (null byte), or URL-encoded variants (`%2e%2e`, `%2f`, `%00`) before any `fs.*` call.
* **Never Concatenate User Input into Paths:** Use `path.join(root, path.basename(userInput))` as the minimum safe form; prefer the `safeJoin` pattern below for full containment validation.

### 2. Database / Storage — Filename Only, No Paths

* **Store Only the Basename:** Before persisting any file reference to Sequelize models or the SQLite/PostgreSQL database, extract the bare name using `path.basename()`. Never store a full path, relative path, or directory component in any database column.
* **Re-derive Paths Server-Side:** When reading a stored file, construct the access path by joining a server-controlled root (from config) with the stored bare filename. The stored value must never be used as-is as an argument to `fs.readFile` or similar.

### 3. In-Memory File Processing (No Intermediate Filesystem Writes)

* **Prefer Buffers and Streams:** multer is already configured with `memoryStorage()` in this project — uploaded content arrives as `req.file.buffer`. Continue processing from that buffer via `Buffer`, `stream.Readable.from()`, or `unzipper.Open.buffer()`. Do not write the buffer to disk before processing.
* **`fs.writeFile` / `fs.mkdtemp` Are Last Resort:** Only write to disk if a third-party dependency strictly requires a file path. Immediately clean up with `fs.unlink` in a `finally` block and use `os.tmpdir()` with a process-generated name — never a user-supplied name.
* **No User-Controlled Temp Names:** Use `crypto.randomUUID()` (or the `uuid` package) to generate temporary filenames, never `req.body.filename` or similar.

### 4. ZIP / Archive File Handling — Specific-File Restriction

* **Validate Every Entry Name:** When iterating over a ZIP using `unzipper`, check each entry's `path` property against traversal patterns before extracting. Reject entries whose cleaned path escapes the destination root (zip slip).
* **Single-Entry Extraction:** When the API targets one specific file inside an archive, locate it by exact name and call `entry.autodrain()` on all other entries. Never extract the full archive speculatively.
* **Reject Absolute Paths and `..` in Entry Names:** Any entry whose `path` starts with `/`, contains `..`, or resolves outside the destination must trigger an error — not a silent skip — so callers can surface it.
* **Limit Entry Count and Decompression Size:** Apply a maximum entry count and a maximum uncompressed byte ceiling per entry (sourced from config) to mitigate zip-bomb attacks.

### 5. Configurable Input Stream Size Limits

* **Never Accept Unbounded Input:** multer's `limits.fileSize` and Express's `bodyParser` limits must be set from configuration values, not hardcoded.
* **Externalize to the `DP_*` Config System:** Read the byte ceiling from the YAML config or `DP_MAX_UPLOAD_BYTES` / `DP_MAX_ZIP_ENTRIES` / `DP_MAX_ZIP_RATIO` environment variables. Apply a safe default when the key is absent.
* **Return HTTP 413 on Overflow:** If multer or stream limits are exceeded, the error handler must respond with `HTTP 413 Request Entity Too Large` and a generic message. Do not echo the configured limit in the response body.

### 6. Uploaded Content Type Allowlisting and No Dynamic Code Execution on User Input

* **Content-Sniff Before Trusting `mimetype`:** `req.file.mimetype` is client-declared and trivially spoofable. Validate the actual buffer content (e.g. via `file-type`'s `fileTypeFromBuffer`) against an explicit allowlist before storing or serving an upload — never key any decision solely on `req.file.mimetype` or the filename extension.
* **SVG Is Not "Just an Image":** SVG is XML and can carry `<script>` and event-handler attributes. If SVG upload is supported at all, it must go through the sanitization and serving hardening in `js-output-encoding-xss.md` (allowlist-based sanitization, `Content-Disposition: attachment` or a cookie-less origin) — never store and serve an uploaded SVG unmodified.
* **Never `eval`/`vm`/Template-Render User-Supplied Code:** Do not pass user-supplied text to `eval()`, `new Function()`, `vm.runInThisContext()`, or a template engine's compile step (e.g. treating uploaded content as an EJS/Handlebars template) without an explicit sandbox — this is Server-Side Template Injection (SSTI) and is equivalent in impact to remote code execution.
* **Authenticated Does Not Mean Safe:** A script/template/expression feature reachable only by an authenticated publisher or admin is still processing untrusted input for the purposes of this directive — authorization controls *who* can reach the feature, not whether execution of that input is safe.

---

## Code Examples for Enforcement

### ❌ Anti-Pattern (What to Reject)

```js
// BAD: Path traversal — user controls the path completely
app.get('/files', (req, res) => {
  const filePath = './uploads/' + req.query.name;  // ../../etc/passwd bypasses root
  res.sendFile(path.resolve(filePath));
});

// BAD: Storing full path in DB
await FileModel.create({ filePath: req.file.originalname }); // may contain /path/evil

// BAD: Writing upload buffer to disk with a user-supplied name
fs.writeFile(`/tmp/${req.file.originalname}`, req.file.buffer, callback);

// BAD: ZIP extraction with no zip-slip protection, hardcoded limit
const zip = await unzipper.Open.buffer(req.file.buffer);
for (const entry of zip.files) {
  const outPath = path.join('/var/app/content', entry.path); // Zip slip
  entry.stream().pipe(fs.createWriteStream(outPath));         // Unbounded decompression
}

// BAD: multer with hardcoded 50 MB limit
const upload = multer({ storage: multer.memoryStorage(), limits: { fileSize: 50 * 1024 * 1024 } });

// BAD: Trusts the client-declared mimetype with no byte-level check, and stores/serves
// an SVG unmodified — the stored-XSS-via-SVG-upload pattern.
app.post('/apis/:id/icon', upload.single('icon'), async (req, res) => {
  await ApiIcon.create({ apiId: req.params.id, data: req.file.buffer, mimeType: req.file.mimetype });
  res.status(201).send();
});

// BAD: Renders user-uploaded content as a template — Server-Side Template Injection.
const ejs = require('ejs');
app.post('/docs/preview', upload.single('doc'), (req, res) => {
  const rendered = ejs.render(req.file.buffer.toString('utf8')); // Arbitrary code execution
  res.send(rendered);
});
```

### Best Practice (What to Generate)

```js
// config/configLoader.js — load file limits from DP_* env / YAML config
function getFileConfig(config) {
  return {
    uploadRootDir: config.uploads?.rootDir || path.join(process.cwd(), 'uploads'),
    maxUploadBytes: parseInt(process.env.DP_MAX_UPLOAD_BYTES || config.uploads?.maxBytes) || 10 * 1024 * 1024,
    maxZipEntries: parseInt(process.env.DP_MAX_ZIP_ENTRIES || config.uploads?.maxZipEntries) || 500,
    maxZipRatio: parseFloat(process.env.DP_MAX_ZIP_RATIO || config.uploads?.maxZipRatio) || 20.0,
  };
}

// utils/fileSafety.js — GOOD: path containment helper
const path = require('path');
const crypto = require('crypto');

function safeJoin(root, userInput) {
  // Strip null bytes and decode percent-encoding before resolving
  const sanitised = userInput.replace(/\0/g, '').replace(/%2e/gi, '.').replace(/%2f/gi, '/');
  const resolved = path.resolve(root, path.basename(sanitised)); // basename strips all dirs
  const rootWithSep = path.resolve(root) + path.sep;
  if (!resolved.startsWith(rootWithSep)) {
    throw new Error('Path escapes root directory');
  }
  return resolved;
}

// GOOD: Serve file with containment check
app.get('/files', (req, res, next) => {
  try {
    const safePath = safeJoin(fileConfig.uploadRootDir, req.query.name);
    res.sendFile(safePath);
  } catch {
    res.status(404).json({ error: 'not_found' });
  }
});

// GOOD: Store only the bare filename in the database
async function saveFileMeta(originalName, mimeType) {
  const bareName = path.basename(originalName); // Strips any directory component
  return FileModel.create({ fileName: bareName, mimeType });
}

// GOOD: Re-derive path server-side from stored bare filename
async function openStoredFile(fileId) {
  const record = await FileModel.findByPk(fileId);
  if (!record) throw new Error('file_not_found');
  const safePath = safeJoin(fileConfig.uploadRootDir, record.fileName);
  return fs.promises.readFile(safePath);
}

// GOOD: multer with configurable limits from config system
function createUploadMiddleware(fileConfig) {
  return multer({
    storage: multer.memoryStorage(), // Keep buffer in memory — no disk write
    limits: { fileSize: fileConfig.maxUploadBytes },
  });
}

// Handle multer size-limit error — return 413, not 500
app.use((err, req, res, next) => {
  if (err.code === 'LIMIT_FILE_SIZE') {
    return res.status(413).json({
      error: 'payload_too_large',
      message: 'Uploaded file exceeds the maximum allowed size.',
      // Do NOT include the actual limit value here
    });
  }
  next(err);
});

// GOOD: In-memory ZIP processing with zip-slip protection, entry count, and decompression guard
const unzipper = require('unzipper');

async function extractSingleEntry(zipBuffer, targetEntryName, fileConfig) {
  const zip = await unzipper.Open.buffer(zipBuffer);

  if (zip.files.length > fileConfig.maxZipEntries) {
    throw Object.assign(new Error('Archive exceeds maximum entry count'), { statusCode: 413 });
  }

  let found = null;

  for (const entry of zip.files) {
    const entryName = entry.path;

    // Reject absolute paths and traversal sequences in entry names
    if (
      path.isAbsolute(entryName) ||
      entryName.includes('..') ||
      entryName.includes('\0')
    ) {
      throw Object.assign(new Error('Archive contains unsafe entry path'), { statusCode: 422 });
    }

    // Match by full normalized path — basename comparison picks the wrong file when
    // multiple entries share a filename across different directories.
    const normalizedEntry = path.posix.normalize(entryName);
    const normalizedTarget = path.posix.normalize(targetEntryName);
    if (normalizedEntry !== normalizedTarget) {
      // Drain and discard non-target entries — do not extract them
      await entry.autodrain();
      continue;
    }

    if (found) {
      // Duplicate path in archive — reject to prevent ambiguous extraction
      throw Object.assign(new Error('Archive contains duplicate entry path'), { statusCode: 422 });
    }
    found = entry;
  }

  if (!found) {
    throw Object.assign(new Error('Requested entry not found in archive'), { statusCode: 404 });
  }

  // Collect bytes with a decompression-ratio guard.
  // Take the tighter of the ratio-based limit and maxUploadBytes (Math.min, not Math.max).
  // Fall back to maxUploadBytes alone when compressedSize is unknown (0).
  const compressedSize = found.compressedSize || 0;
  const maxDecompressed = compressedSize > 0
    ? Math.min(compressedSize * fileConfig.maxZipRatio, fileConfig.maxUploadBytes)
    : fileConfig.maxUploadBytes;

  const chunks = [];
  let totalBytes = 0;
  const stream = found.stream();

  await new Promise((resolve, reject) => {
    stream.on('data', (chunk) => {
      totalBytes += chunk.length;
      if (totalBytes > maxDecompressed) {
        stream.destroy();
        reject(Object.assign(new Error('Decompressed entry exceeds allowed size'), { statusCode: 413 }));
        return;
      }
      chunks.push(chunk);
    });
    stream.on('end', resolve);
    stream.on('error', reject);
  });

  return Buffer.concat(chunks);
}

// utils/uploadContentValidation.js — GOOD: content-sniffs the actual buffer
// against an explicit allowlist, never trusting req.file.mimetype or the extension.
const { fileTypeFromBuffer } = require('file-type');

const ALLOWED_UPLOAD_TYPES = new Set(['image/png', 'image/jpeg']);
// 'image/svg+xml' deliberately excluded here — see js-output-encoding-xss.md
// for the sanitization + serving hardening required if SVG upload is added.

async function validateUploadContent(buffer) {
  const detected = await fileTypeFromBuffer(buffer);
  if (!detected || !ALLOWED_UPLOAD_TYPES.has(detected.mime)) {
    throw Object.assign(new Error('Unsupported upload content type'), { statusCode: 422 });
  }
  return detected.mime;
}

app.post('/apis/:id/icon', upload.single('icon'), async (req, res, next) => {
  try {
    if (!req.file) {
      return res.status(400).json({ error: 'invalid_request', message: 'No file was uploaded.' });
    }
    const sniffedMime = await validateUploadContent(req.file.buffer); // Actual bytes, not the header
    await ApiIcon.create({ apiId: req.params.id, data: req.file.buffer, mimeType: sniffedMime });
    res.status(201).send();
  } catch (err) {
    next(err);
  }
});

// GOOD: User-uploaded content is only ever displayed, never compiled/executed
// as a template. If rendering is required, sanitize per js-output-encoding-xss.md
// and pass the result through an escaping interpolation, not a template compile step.
app.post('/docs/preview', upload.single('doc'), (req, res) => {
  const rawText = req.file.buffer.toString('utf8');
  res.render('docPreview', { content: sanitizeApiDocHtml(rawText) }); // <%= content %> in the view
});
```

---

> **Verification Checklist before outputting code:**
> * Is every file path constructed with `path.resolve()` and verified against the root with a trailing `path.sep` prefix check? (If no, use `safeJoin`).
> * Is only `path.basename()` of the filename stored in Sequelize models or database columns? (If the full path is stored, strip it).
> * Is file processing done from `req.file.buffer` in memory without any `fs.writeFile` / `fs.mkdtemp` intermediate step? (If disk writes exist, remove them or wrap in a `finally` cleanup).
> * Is every ZIP entry's `path` checked for `..`, absolute paths, and null bytes before any extraction? (If not, add checks and call `entry.autodrain()` on skipped entries).
> * Are multer `limits.fileSize`, `maxZipEntries`, and `maxZipRatio` sourced from the `DP_*` config system rather than hardcoded? (If hardcoded, move to config with a safe default).
> * Is an upload's actual content sniffed (`fileTypeFromBuffer`) against an allowlist, rather than trusting `req.file.mimetype` or the filename extension? (If trusted as-is, add content-sniffing.)
> * Is a user-uploaded SVG stored/served without the sanitization and serving hardening from `js-output-encoding-xss.md`? (If yes, apply both — sanitize and serve as `attachment`/from a cookie-less origin.)
> * Does any code path compile or execute user-supplied content as a template (`ejs.render`, `new Function`, `vm.runInThisContext`) rather than only ever displaying it through an escaping interpolation? (If yes, this is SSTI/RCE — remove the compile/execute step.)
