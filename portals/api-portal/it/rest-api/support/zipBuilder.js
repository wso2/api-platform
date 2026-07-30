// --------------------------------------------------------------------
// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
// --------------------------------------------------------------------

// Minimal ZIP writer for artifact-upload tests (POST/PUT /apis with an `artifact`
// field) — no zip-writing library is a dependency of it/rest-api, and the server
// side only needs to unzip (via `unzipper`), so this only ever needs to produce
// valid input, not full ZIP feature coverage. Uses STORED (uncompressed) entries
// only, so no deflate implementation is needed — just each entry's bytes verbatim
// plus a CRC-32 (Node's built-in zlib.crc32, added in Node 21).

const zlib = require('zlib');

function dosDateTime() {
    // Fixed MS-DOS date/time (1980-01-01 00:00:00) — entry timestamps aren't
    // inspected by anything on the read side here.
    return { time: 0, date: 0x21 };
}

function writeUint16LE(buf, offset, value) {
    buf.writeUInt16LE(value, offset);
}

function writeUint32LE(buf, offset, value) {
    buf.writeUInt32LE(value, offset);
}

/**
 * @param {Array<{ name: string, content: string | Buffer }>} entries
 * @returns {Buffer}
 */
function createZip(entries) {
    const { time, date } = dosDateTime();
    const localChunks = [];
    const centralChunks = [];
    let offset = 0;

    for (const entry of entries) {
        const nameBuf = Buffer.from(entry.name, 'utf8');
        const contentBuf = Buffer.isBuffer(entry.content) ? entry.content : Buffer.from(entry.content, 'utf8');
        const crc = zlib.crc32(contentBuf);

        const localHeader = Buffer.alloc(30);
        writeUint32LE(localHeader, 0, 0x04034b50);
        writeUint16LE(localHeader, 4, 20); // version needed
        writeUint16LE(localHeader, 6, 0); // flags
        writeUint16LE(localHeader, 8, 0); // compression: stored
        writeUint16LE(localHeader, 10, time);
        writeUint16LE(localHeader, 12, date);
        writeUint32LE(localHeader, 14, crc);
        writeUint32LE(localHeader, 18, contentBuf.length); // compressed size
        writeUint32LE(localHeader, 22, contentBuf.length); // uncompressed size
        writeUint16LE(localHeader, 26, nameBuf.length);
        writeUint16LE(localHeader, 28, 0); // extra field length

        localChunks.push(localHeader, nameBuf, contentBuf);

        const centralHeader = Buffer.alloc(46);
        writeUint32LE(centralHeader, 0, 0x02014b50);
        writeUint16LE(centralHeader, 4, 20); // version made by
        writeUint16LE(centralHeader, 6, 20); // version needed
        writeUint16LE(centralHeader, 8, 0); // flags
        writeUint16LE(centralHeader, 10, 0); // compression: stored
        writeUint16LE(centralHeader, 12, time);
        writeUint16LE(centralHeader, 14, date);
        writeUint32LE(centralHeader, 16, crc);
        writeUint32LE(centralHeader, 20, contentBuf.length);
        writeUint32LE(centralHeader, 24, contentBuf.length);
        writeUint16LE(centralHeader, 28, nameBuf.length);
        writeUint16LE(centralHeader, 30, 0); // extra field length
        writeUint16LE(centralHeader, 32, 0); // comment length
        writeUint16LE(centralHeader, 34, 0); // disk number start
        writeUint16LE(centralHeader, 36, 0); // internal attributes
        writeUint32LE(centralHeader, 38, 0); // external attributes
        writeUint32LE(centralHeader, 42, offset); // relative offset of local header

        centralChunks.push(centralHeader, nameBuf);

        offset += localHeader.length + nameBuf.length + contentBuf.length;
    }

    const centralDirectoryStart = offset;
    const centralDirectory = Buffer.concat(centralChunks);

    const eocd = Buffer.alloc(22);
    writeUint32LE(eocd, 0, 0x06054b50);
    writeUint16LE(eocd, 4, 0); // disk number
    writeUint16LE(eocd, 6, 0); // disk with central directory
    writeUint16LE(eocd, 8, entries.length); // records on this disk
    writeUint16LE(eocd, 10, entries.length); // total records
    writeUint32LE(eocd, 12, centralDirectory.length);
    writeUint32LE(eocd, 16, centralDirectoryStart);
    writeUint16LE(eocd, 20, 0); // comment length

    return Buffer.concat([...localChunks, centralDirectory, eocd]);
}

module.exports = { createZip };
