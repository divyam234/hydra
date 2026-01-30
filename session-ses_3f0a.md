# Aria2c HTTP features port to Go

**Session ID:** ses_3f0a58818fferV2d69HI0FXD56
**Created:** 1/30/2026, 8:11:53 PM
**Updated:** 1/30/2026, 8:23:01 PM

---

## User

clone ari2c with depth=1 I wann convert whdle ari2 inot go wihtout magnet adn other thind only naiatve htpp downloads wirter with all ari2c fetured port 1on 1 wirte a whole plan

---

## Assistant (Plan · gemini-claude-opus-4-5-thinking · 12.8s)

I'll clone the aria2c repository first, then analyze it to create a comprehensive plan for porting to Go.

```
Tool: bash

**Input:**
```json
{
  "description": "Clone aria2 repo with depth=1",
  "timeout": 120000,
  "command": "git clone --depth=1 https://github.com/aria2/aria2.git"
}
```
**Output:**
```
Cloning into 'aria2'...

```
```

---

## Assistant (Plan · gemini-claude-opus-4-5-thinking · 11.1s)

```
Tool: bash

**Input:**
```json
{
  "command": "ls -la aria2/",
  "description": "List aria2 directory contents"
}
```
**Output:**
```
total 1044
drwxr-xr-x 1 bhunter bhunter    804 Jan 30 20:12 .
drwxr-xr-x 1 bhunter bhunter     10 Jan 30 20:12 ..
drwxr-xr-x 1 bhunter bhunter     78 Jan 30 20:12 android
-rwxr-xr-x 1 bhunter bhunter   2610 Jan 30 20:12 android-config
-rwxr-xr-x 1 bhunter bhunter   1944 Jan 30 20:12 android-release
-rw-r--r-- 1 bhunter bhunter   1231 Jan 30 20:12 AUTHORS
-rwxr-xr-x 1 bhunter bhunter   1735 Jan 30 20:12 build_test.sh
-rw-r--r-- 1 bhunter bhunter      0 Jan 30 20:12 ChangeLog
-rw-r--r-- 1 bhunter bhunter 869120 Jan 30 20:12 ChangeLog.old
-rw-r--r-- 1 bhunter bhunter   3443 Jan 30 20:12 .clang-format
-rw-r--r-- 1 bhunter bhunter  35433 Jan 30 20:12 configure.ac
-rw-r--r-- 1 bhunter bhunter  18092 Jan 30 20:12 COPYING
drwxr-xr-x 1 bhunter bhunter     32 Jan 30 20:12 deps
drwxr-xr-x 1 bhunter bhunter    110 Jan 30 20:12 doc
-rw-r--r-- 1 bhunter bhunter   4591 Jan 30 20:12 Dockerfile.android
-rw-r--r-- 1 bhunter bhunter   3772 Jan 30 20:12 Dockerfile.mingw
-rw-r--r-- 1 bhunter bhunter   3355 Jan 30 20:12 Dockerfile.raspberrypi
drwxr-xr-x 1 bhunter bhunter     52 Jan 30 20:12 examples
drwxr-xr-x 1 bhunter bhunter    136 Jan 30 20:12 .git
drwxr-xr-x 1 bhunter bhunter     46 Jan 30 20:12 .github
-rw-r--r-- 1 bhunter bhunter    449 Jan 30 20:12 .gitignore
-rwxr-xr-x 1 bhunter bhunter   1206 Jan 30 20:12 import-po
drwxr-xr-x 1 bhunter bhunter     40 Jan 30 20:12 lib
-rw-r--r-- 1 bhunter bhunter   6968 Jan 30 20:12 LICENSE.OpenSSL
drwxr-xr-x 1 bhunter bhunter    144 Jan 30 20:12 m4
-rw-r--r-- 1 bhunter bhunter   1318 Jan 30 20:12 Makefile.am
-rwxr-xr-x 1 bhunter bhunter   1167 Jan 30 20:12 makerelease
-rw-r--r-- 1 bhunter bhunter  14538 Jan 30 20:12 makerelease-osx.mk
-rw-r--r-- 1 bhunter bhunter   2074 Jan 30 20:12 mingw-build-memo
-rwxr-xr-x 1 bhunter bhunter   2953 Jan 30 20:12 mingw-config
-rwxr-xr-x 1 bhunter bhunter   2057 Jan 30 20:12 mingw-release
-rw-r--r-- 1 bhunter bhunter    117 Jan 30 20:12 NEWS
drwxr-xr-x 1 bhunter bhunter     22 Jan 30 20:12 osx-package
drwxr-xr-x 1 bhunter bhunter     92 Jan 30 20:12 po
-rw-r--r-- 1 bhunter bhunter     15 Jan 30 20:12 README
-rw-r--r-- 1 bhunter bhunter   2907 Jan 30 20:12 README.android
-rw-r--r-- 1 bhunter bhunter   1563 Jan 30 20:12 README.mingw
-rw-r--r-- 1 bhunter bhunter  22434 Jan 30 20:12 README.rst
-rw-r--r-- 1 bhunter bhunter   1663 Jan 30 20:12 script-helper
drwxr-xr-x 1 bhunter bhunter  36890 Jan 30 20:12 src
drwxr-xr-x 1 bhunter bhunter  10676 Jan 30 20:12 test
-rw-r--r-- 1 bhunter bhunter   1521 Jan 30 20:12 .travis.yml

```
```

```
Tool: bash

**Input:**
```json
{
  "command": "find aria2/src -name \"*.cc\" -o -name \"*.h\" | head -100",
  "description": "Find source files in aria2"
}
```
**Output:**
```
aria2/src/A2STR.cc
aria2/src/A2STR.h
aria2/src/ARC4Encryptor.h
aria2/src/AbstractAuthResolver.cc
aria2/src/AbstractAuthResolver.h
aria2/src/AbstractBtMessage.cc
aria2/src/AbstractBtMessage.h
aria2/src/AbstractCommand.cc
aria2/src/AbstractCommand.h
aria2/src/AbstractDiskWriter.cc
aria2/src/AbstractDiskWriter.h
aria2/src/AbstractHttpServerResponseCommand.cc
aria2/src/AbstractHttpServerResponseCommand.h
aria2/src/AbstractOptionHandler.cc
aria2/src/AbstractOptionHandler.h
aria2/src/AbstractProxyRequestCommand.cc
aria2/src/AbstractProxyRequestCommand.h
aria2/src/AbstractProxyResponseCommand.cc
aria2/src/AbstractProxyResponseCommand.h
aria2/src/AbstractSingleDiskAdaptor.cc
aria2/src/AbstractSingleDiskAdaptor.h
aria2/src/ActivePeerConnectionCommand.cc
aria2/src/ActivePeerConnectionCommand.h
aria2/src/AdaptiveFileAllocationIterator.cc
aria2/src/AdaptiveFileAllocationIterator.h
aria2/src/AdaptiveURISelector.cc
aria2/src/AdaptiveURISelector.h
aria2/src/Adler32MessageDigestImpl.cc
aria2/src/Adler32MessageDigestImpl.h
aria2/src/AnnounceList.cc
aria2/src/AnnounceList.h
aria2/src/AnnounceTier.cc
aria2/src/AnnounceTier.h
aria2/src/AnonDiskWriterFactory.h
aria2/src/ApiCallbackDownloadEventListener.cc
aria2/src/ApiCallbackDownloadEventListener.h
aria2/src/AppleMessageDigestImpl.cc
aria2/src/AppleTLSContext.cc
aria2/src/AppleTLSContext.h
aria2/src/AppleTLSSession.cc
aria2/src/AppleTLSSession.h
aria2/src/AsyncNameResolver.cc
aria2/src/AsyncNameResolver.h
aria2/src/AsyncNameResolverMan.cc
aria2/src/AsyncNameResolverMan.h
aria2/src/AuthConfig.cc
aria2/src/AuthConfig.h
aria2/src/AuthConfigFactory.cc
aria2/src/AuthConfigFactory.h
aria2/src/AuthResolver.h
aria2/src/AutoSaveCommand.cc
aria2/src/AutoSaveCommand.h
aria2/src/BackupIPv4ConnectCommand.cc
aria2/src/BackupIPv4ConnectCommand.h
aria2/src/BencodeDiskWriter.h
aria2/src/BencodeDiskWriterFactory.h
aria2/src/BencodeParser.cc
aria2/src/BencodeParser.h
aria2/src/BinaryStream.h
aria2/src/BitfieldMan.cc
aria2/src/BitfieldMan.h
aria2/src/BtAbortOutstandingRequestEvent.cc
aria2/src/BtAbortOutstandingRequestEvent.h
aria2/src/BtAllowedFastMessage.cc
aria2/src/BtAllowedFastMessage.h
aria2/src/BtAnnounce.cc
aria2/src/BtAnnounce.h
aria2/src/BtBitfieldMessage.cc
aria2/src/BtBitfieldMessage.h
aria2/src/BtBitfieldMessageValidator.cc
aria2/src/BtBitfieldMessageValidator.h
aria2/src/BtCancelMessage.cc
aria2/src/BtCancelMessage.h
aria2/src/BtCancelSendingPieceEvent.h
aria2/src/BtCheckIntegrityEntry.cc
aria2/src/BtCheckIntegrityEntry.h
aria2/src/BtChokeMessage.cc
aria2/src/BtChokeMessage.h
aria2/src/BtChokingEvent.h
aria2/src/BtConstants.h
aria2/src/BtDependency.cc
aria2/src/BtDependency.h
aria2/src/BtExtendedMessage.cc
aria2/src/BtExtendedMessage.h
aria2/src/BtFileAllocationEntry.cc
aria2/src/BtFileAllocationEntry.h
aria2/src/BtHandshakeMessage.cc
aria2/src/BtHandshakeMessage.h
aria2/src/BtHandshakeMessageValidator.cc
aria2/src/BtHandshakeMessageValidator.h
aria2/src/BtHaveAllMessage.cc
aria2/src/BtHaveAllMessage.h
aria2/src/BtHaveMessage.cc
aria2/src/BtHaveMessage.h
aria2/src/BtHaveNoneMessage.cc
aria2/src/BtHaveNoneMessage.h
aria2/src/BtInteractive.h
aria2/src/BtInterestedMessage.cc
aria2/src/BtInterestedMessage.h
aria2/src/BtKeepAliveMessage.cc

```
```

```
Tool: bash

**Input:**
```json
{
  "command": "wc -l aria2/src/*.cc aria2/src/*.h 2>/dev/null | tail -5",
  "description": "Count lines in aria2 source"
}
```
**Output:**
```
    252 aria2/src/XmlRpcRequestParserStateImpl.h
    106 aria2/src/XmlRpcRequestParserStateMachine.h
     74 aria2/src/XORCloser.h
     66 aria2/src/ZeroBtMessage.h
 140662 total

```
```

---

## Assistant (Plan · gemini-claude-opus-4-5-thinking · 432.5s)

Let me explore the aria2 codebase more deeply to understand its architecture, specifically focusing on HTTP download functionality (excluding BitTorrent/Magnet).

```
Tool: task

**Input:**
```json
{
  "description": "Explore aria2 HTTP download core",
  "prompt": "Explore the aria2 codebase at /home/bhunter/repos/github/ari2-go/aria2/src to understand the HTTP download architecture.\n\nFocus on:\n1. Core download engine architecture (DownloadEngine, RequestGroup, etc.)\n2. HTTP protocol implementation (HttpConnection, HttpRequest, HttpResponse)\n3. Segmented/parallel download mechanism\n4. File writing and disk I/O\n5. Progress tracking and statistics\n6. Connection pooling and reuse\n7. Resume/checkpoint functionality\n8. Rate limiting\n9. Retry and error handling\n10. RPC interface (JSON-RPC/XML-RPC)\n\nIGNORE all BitTorrent, DHT, Magnet, and Metalink related files.\n\nReturn a detailed summary of the architecture and key classes/files that would need to be ported for HTTP-only downloads.",
  "subagent_type": "explore"
}
```
**Output:**
```
Now I have a comprehensive understanding of the aria2 HTTP download architecture. Let me compile this into a detailed summary:

---

# Aria2 HTTP Download Architecture - Detailed Analysis

## Executive Summary

Aria2 is a sophisticated download manager built around an event-driven command-pattern architecture. The codebase is well-structured with clear separation of concerns between:
- Download orchestration (DownloadEngine, RequestGroup, RequestGroupMan)
- Protocol implementation (HTTP/FTP specific commands)
- Segmented download management (SegmentMan, PieceStorage, Piece)
- Disk I/O (DiskAdaptor, DiskWriter hierarchy)
- Progress/statistics tracking (NetStat, PeerStat, SpeedCalc, TransferStat)
- RPC interface (JSON-RPC/XML-RPC via RpcMethod hierarchy)

---

## 1. Core Download Engine Architecture

### Key Files:
- `/home/bhunter/repos/github/ari2-go/aria2/src/DownloadEngine.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/RequestGroup.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/RequestGroupMan.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/DownloadContext.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/Command.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/AbstractCommand.h` / `.cc`

### DownloadEngine
The central event loop and coordinator:
- **Event Polling**: Uses `EventPoll` interface for non-blocking I/O (epoll/kqueue/select)
- **Command Queue**: Maintains two command queues:
  - `commands_`: Regular download commands
  - `routineCommands_`: Periodic maintenance commands
- **Socket Pooling**: Connection reuse via `socketPool_` multimap (key = "host:port/proxy")
- **DNS Caching**: Built-in DNS cache via `DNSCache`
- **Cookie Storage**: `CookieStorage` for HTTP cookies
- **Authentication**: `AuthConfigFactory` for HTTP auth
- **Statistics**: Delegates to `StatCalc` for periodic stats calculation
- **Halt Handling**: Graceful and force shutdown via signal handling

**Main Loop** (`run()`):
1. Wait for socket events via `eventPoll_->poll()`
2. Execute commands matching status filter
3. Calculate statistics periodically
4. Handle halt requests from signals

### RequestGroup
Represents a single download task (one URL or mirror group):
- **State Machine**: WAITING -> ACTIVE states
- **Segment Management**: Holds `SegmentMan` for parallel segment tracking
- **Piece Storage**: `PieceStorage` for data management
- **Progress Info**: `BtProgressInfoFile` for resume capability (.aria2 files)
- **Speed Limits**: Per-download speed limits (`maxDownloadSpeedLimit_`, `maxUploadSpeedLimit_`)
- **Concurrency Control**: `numConcurrentCommand_` for parallel connections per download
- **Error Tracking**: `lastErrorCode_`, `lastErrorMessage_`
- **Dependencies**: Can depend on other downloads (metalink scenarios)

### RequestGroupMan
Manages all RequestGroups:
- **Active/Reserved Queues**: `requestGroups_` (active), `reservedGroups_` (waiting)
- **Download Results**: `downloadResults_` for completed downloads
- **Global Speed Limits**: `maxOverallDownloadSpeedLimit_`, `maxOverallUploadSpeedLimit_`
- **Concurrent Download Limit**: `maxConcurrentDownloads_`
- **Server Statistics**: `ServerStatMan` for adaptive server selection
- **Write Disk Cache**: `WrDiskCache` for buffered writes

### DownloadContext
Download metadata container:
- **File Entries**: List of `FileEntry` objects (files in download)
- **Piece Hashes**: For integrity verification
- **Total Length**: Known or unknown file size handling
- **Digest/Checksum**: Whole-file hash support

---

## 2. HTTP Protocol Implementation

### Key Files:
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpConnection.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpRequest.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpResponse.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpHeader.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpHeaderProcessor.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpInitiateConnectionCommand.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpRequestCommand.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpResponseCommand.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpDownloadCommand.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpSkipResponseCommand.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpProxyRequestCommand.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpProxyResponseCommand.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/Request.h` / `.cc`

### HTTP Command Chain (State Machine):
```
HttpInitiateConnectionCommand
    |
    v (resolve hostname, check connection pool)
ConnectCommand (if new connection needed)
    |
    v (TCP connect, optionally TLS handshake)
HttpProxyRequestCommand (if HTTP proxy with CONNECT)
    |
    v
HttpProxyResponseCommand
    |
    v
HttpRequestCommand (send HTTP request)
    |
    v
HttpResponseCommand (receive/parse headers)
    |
    v
HttpDownloadCommand (download body) or HttpSkipResponseCommand (skip body for redirects/errors)
```

### HttpConnection
Manages HTTP request/response pipeline:
- **Request Queue**: `outstandingHttpRequests_` for pipelining support
- **Socket Buffering**: `SocketBuffer` for outgoing data, `SocketRecvBuffer` for incoming
- **Pipelining**: Can issue multiple requests before receiving responses
- **Confidential Info Removal**: Strips passwords from logs

### HttpRequest
Builds HTTP request strings:
- **Range Support**: Generates Range headers for segmented downloads
- **Proxy Support**: HTTP proxy CONNECT method
- **Authentication**: Basic/Digest auth via `AuthConfig`
- **Cookies**: Integrates with `CookieStorage`
- **Compression**: Accept-Encoding: gzip support
- **Conditional Requests**: If-Modified-Since header

### HttpResponse
Parses and validates HTTP responses:
- **Status Code Handling**: Redirects (3xx), errors (4xx/5xx)
- **Content-Length/Transfer-Encoding**: Determines body handling
- **Content-Disposition**: Filename extraction
- **Content-Encoding**: gzip/deflate decompression
- **Digest Header**: Checksum extraction for verification

### Request
URL representation and connection tracking:
- **URI Parsing**: Protocol, host, port, path, query
- **Retry Tracking**: `tryCount_`, redirect counting
- **Persistent Connection**: Keep-alive and pipelining hints
- **Connection Info**: Connected address for connection reuse

---

## 3. Segmented/Parallel Download Mechanism

### Key Files:
- `/home/bhunter/repos/github/ari2-go/aria2/src/SegmentMan.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/Segment.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/PiecedSegment.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/Piece.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/PieceStorage.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/DefaultPieceStorage.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/UnknownLengthPieceStorage.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/BitfieldMan.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/FileEntry.h` / `.cc`

### SegmentMan
Coordinates parallel segment downloads:
- **Segment Assignment**: `getSegment()` returns next segment for a command (cuid)
- **Segment Tracking**: `usedSegmentEntries_` tracks which command owns which segment
- **Completion**: `completeSegment()` marks segment done
- **Cancellation**: `cancelSegment()` releases segment for retry
- **Peer Statistics**: Tracks download speeds per connection

### Segment (Interface)
Abstract segment representation:
- **Position**: File offset and length
- **Progress**: `getWrittenLength()`, `updateWrittenLength()`
- **Hash Support**: Per-piece hash calculation for verification
- **Piece Association**: Links to underlying `Piece`

### PiecedSegment
Concrete Segment backed by a Piece:
- Used when file size is known and file is split into pieces

### Piece
Fundamental download unit:
- **Block-Level Tracking**: 16KB blocks within piece
- **Bitfield**: Tracks completed blocks
- **Multi-User**: Can be used by multiple commands (end-game mode)
- **Hash Calculation**: Incremental hash for piece verification
- **Write Cache**: Integration with `WrDiskCacheEntry`

### PieceStorage (Interface)
Abstract storage management:
- **Piece Management**: Get missing pieces, complete/cancel pieces
- **Bitfield**: Track overall completion state
- **Disk Adaptor**: Access to disk I/O layer
- **End-Game Mode**: Special mode when few pieces remain

### DefaultPieceStorage
Standard implementation for known-length files:
- **Piece Selection**: Via `PieceSelector` strategy
- **Used Pieces Set**: Tracks in-flight pieces
- **Have Advertising**: For peer notification (BitTorrent, but mechanism useful)

### UnknownLengthPieceStorage
For streaming/unknown-length downloads:
- Single growing piece
- No segmented download support

### BitfieldMan
Bit-level completion tracking:
- Efficient bitfield operations
- Filter support for selective file download

---

## 4. File Writing and Disk I/O

### Key Files:
- `/home/bhunter/repos/github/ari2-go/aria2/src/DiskAdaptor.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/AbstractSingleDiskAdaptor.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/DirectDiskAdaptor.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/MultiDiskAdaptor.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/DiskWriter.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/AbstractDiskWriter.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/DefaultDiskWriter.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/WrDiskCache.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/WrDiskCacheEntry.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/BinaryStream.h`

### DiskAdaptor (Interface)
High-level file operations:
- **File Management**: Open, close, create files
- **Read/Write**: Via `BinaryStream` interface
- **File Allocation**: Pre-allocation strategies (adaptive, fallocate, truncate)
- **Multiple Files**: Handles multi-file downloads as single byte stream

### DirectDiskAdaptor
Single-file direct I/O

### MultiDiskAdaptor
Multi-file I/O with offset translation

### DiskWriter (Interface)
Low-level file operations:
- Open/close, read/write at offset
- Truncate, allocate space
- Read-only mode support

### AbstractDiskWriter
Platform-specific implementation:
- File descriptor management
- Memory-mapped I/O support
- OS buffer flushing

### WrDiskCache
Write caching layer:
- Buffers writes before disk flush
- Reduces disk I/O for small writes
- Entry management per piece

### StreamFilter
Data transformation pipeline:
- **SinkStreamFilter**: Final write to disk
- **ChunkedDecodingStreamFilter**: HTTP chunked transfer decoding
- **GZipDecodingStreamFilter**: gzip/deflate decompression
- Chainable filter architecture

---

## 5. Progress Tracking and Statistics

### Key Files:
- `/home/bhunter/repos/github/ari2-go/aria2/src/TransferStat.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/NetStat.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/PeerStat.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/SpeedCalc.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/ServerStat.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/ServerStatMan.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/StatCalc.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/ConsoleStatCalc.h` / `.cc`

### TransferStat
Aggregated transfer statistics:
- Download/upload speeds
- Session download/upload lengths
- Used for display and speed limiting

### NetStat
Network statistics per context:
- Uses `SpeedCalc` for speed calculation
- Download start/stop time tracking
- Status tracking (IDLE, ACTIVE)

### PeerStat
Per-connection statistics:
- Speed tracking for individual connections
- Hostname and protocol info
- Used for adaptive URI selection

### SpeedCalc
Rolling window speed calculation:
- Time-slotted byte counting
- Removes stale slots for accurate current speed
- Tracks maximum speed

### ServerStat
Per-server statistics:
- Download speed history
- Single/multi-connection average speeds
- Error status tracking
- Used for adaptive server selection

---

## 6. Connection Pooling and Reuse

### Key Files:
- `/home/bhunter/repos/github/ari2-go/aria2/src/DownloadEngine.h` (socketPool_)
- `/home/bhunter/repos/github/ari2-go/aria2/src/SocketCore.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/EvictSocketPoolCommand.h` / `.cc`

### Connection Pool (in DownloadEngine):
- **Pool Storage**: `std::multimap<string, SocketPoolEntry>` keyed by "host:port/proxy"
- **Timeout**: Connections expire after configurable timeout (default 15s)
- **Eviction**: Periodic cleanup via `EvictSocketPoolCommand`
- **Retrieval**: `popPooledSocket()` finds reusable connection

### SocketCore
Socket abstraction:
- TCP/UDP socket wrapper
- Non-blocking I/O
- TLS support (via TLSSession)
- Platform abstraction (Windows/Unix)

---

## 7. Resume/Checkpoint Functionality

### Key Files:
- `/home/bhunter/repos/github/ari2-go/aria2/src/BtProgressInfoFile.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/DefaultBtProgressInfoFile.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/NullProgressInfoFile.h`

### DefaultBtProgressInfoFile
.aria2 control file handling:
- **Format**: Binary format with download state
- **Content**: Bitfield, in-flight pieces, file info
- **Operations**: save(), load(), exists(), removeFile()
- **Optimization**: Tracks last digest to avoid unnecessary writes

### Resume Flow:
1. Check for .aria2 file existence
2. Load bitfield and in-flight piece state
3. Restore PieceStorage state
4. Resume from last known position

---

## 8. Rate Limiting

### Key Files:
- `/home/bhunter/repos/github/ari2-go/aria2/src/RequestGroup.h` (per-download limits)
- `/home/bhunter/repos/github/ari2-go/aria2/src/RequestGroupMan.h` (global limits)
- `/home/bhunter/repos/github/ari2-go/aria2/src/DownloadCommand.cc` (enforcement)

### Rate Limiting Implementation:
- **Per-Download**: `RequestGroup::maxDownloadSpeedLimit_`
- **Global**: `RequestGroupMan::maxOverallDownloadSpeedLimit_`
- **Enforcement**: In `DownloadCommand::executeInternal()`:
  - Check if speed exceeds limit
  - If exceeded, disable socket read/write checks
  - Command re-added without I/O monitoring
  - Resumes when speed drops below limit

### Speed Checking:
```cpp
if (getDownloadEngine()->getRequestGroupMan()->doesOverallDownloadSpeedExceed() ||
    getRequestGroup()->doesDownloadSpeedExceed()) {
    addCommandSelf();
    disableReadCheckSocket();
    disableWriteCheckSocket();
    return false;
}
```

---

## 9. Retry and Error Handling

### Key Files:
- `/home/bhunter/repos/github/ari2-go/aria2/src/error_code.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/DlAbortEx.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/DlRetryEx.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/DownloadFailureException.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/AbstractCommand.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/DownloadCommand.cc`

### Error Codes (error_code.h):
```cpp
enum Value {
    FINISHED = 0,
    UNKNOWN_ERROR = 1,
    TIME_OUT = 2,
    RESOURCE_NOT_FOUND = 3,
    MAX_FILE_NOT_FOUND = 4,
    TOO_SLOW_DOWNLOAD_SPEED = 5,
    NETWORK_PROBLEM = 6,
    CANNOT_RESUME = 8,
    HTTP_PROTOCOL_ERROR = 22,
    HTTP_TOO_MANY_REDIRECTS = 23,
    HTTP_AUTH_FAILED = 24,
    // ... more
};
```

### Exception Hierarchy:
- **DlRetryEx**: Recoverable error, retry with backoff
- **DlAbortEx**: Abort current request, try different URI
- **DownloadFailureException**: Fatal error, fail download

### Retry Mechanism:
1. Command catches exception
2. `prepareForRetry(waitTime)` called
3. Segment cancelled and returned to pool
4. URI marked with error result
5. New command created after wait period
6. `Request::tryCount_` incremented

### Lowest Speed Limit:
```cpp
void DownloadCommand::checkLowestDownloadSpeed() const {
    if (lowestDownloadSpeedLimit_ > 0 &&
        peerStat_->getDownloadStartTime().difference(global::wallclock()) >= startupIdleTime_) {
        int nowSpeed = peerStat_->calculateDownloadSpeed();
        if (nowSpeed <= lowestDownloadSpeedLimit_) {
            throw DL_ABORT_EX2(..., error_code::TOO_SLOW_DOWNLOAD_SPEED);
        }
    }
}
```

---

## 10. RPC Interface

### Key Files:
- `/home/bhunter/repos/github/ari2-go/aria2/src/RpcMethod.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/RpcMethodImpl.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/RpcMethodFactory.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/RpcRequest.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/RpcResponse.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpServerCommand.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpServer.h` / `.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/XmlRpc*.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/JsonDiskWriter.h`

### RPC Methods (aria2.* namespace):
- `aria2.addUri` - Add download by URI
- `aria2.remove` / `aria2.forceRemove` - Remove download
- `aria2.pause` / `aria2.unpause` - Pause/resume
- `aria2.tellStatus` - Get download status
- `aria2.tellActive` / `aria2.tellWaiting` / `aria2.tellStopped` - List downloads
- `aria2.getOption` / `aria2.changeOption` - Options
- `aria2.getGlobalStat` - Global statistics
- `aria2.getVersion` - Version info
- `aria2.shutdown` / `aria2.forceShutdown` - Shutdown
- `aria2.getFiles` / `aria2.getUris` / `aria2.getServers` - Download details

### RPC Architecture:
- **RpcMethod**: Base class with `process()` virtual method
- **RpcMethodFactory**: Creates method instances by name
- **Request/Response**: Structured data exchange
- **Transport**: HTTP server with JSON-RPC and XML-RPC support

---

## Files to Port for HTTP-Only Downloads

### Tier 1 - Core Engine (Must Have):
1. `DownloadEngine.h/.cc` - Main event loop
2. `RequestGroup.h/.cc` - Download task management
3. `RequestGroupMan.h/.cc` - Multi-download management
4. `DownloadContext.h/.cc` - Download metadata
5. `Command.h` - Command interface
6. `AbstractCommand.h/.cc` - Base command implementation
7. `Option.h/.cc` - Configuration handling
8. `EventPoll.h` + implementations - Event polling

### Tier 2 - HTTP Protocol (Must Have):
1. `HttpConnection.h/.cc` - HTTP connection management
2. `HttpRequest.h/.cc` - Request building
3. `HttpResponse.h/.cc` - Response parsing
4. `HttpHeader.h/.cc` - Header handling
5. `HttpHeaderProcessor.h/.cc` - Header parsing
6. `HttpInitiateConnectionCommand.h/.cc` - Connection initiation
7. `HttpRequestCommand.h/.cc` - Request sending
8. `HttpResponseCommand.h/.cc` - Response handling
9. `HttpDownloadCommand.h/.cc` - Body download
10. `HttpSkipResponseCommand.h/.cc` - Skip response body
11. `Request.h/.cc` - URL representation
12. `ConnectCommand.h/.cc` - TCP connection
13. `uri.h/.cc` - URI parsing

### Tier 3 - Segmented Downloads (Must Have):
1. `SegmentMan.h/.cc` - Segment coordination
2. `Segment.h` - Segment interface
3. `PiecedSegment.h/.cc` - Segment implementation
4. `Piece.h/.cc` - Download unit
5. `PieceStorage.h` - Storage interface
6. `DefaultPieceStorage.h/.cc` - Standard storage
7. `UnknownLengthPieceStorage.h/.cc` - Unknown length
8. `BitfieldMan.h/.cc` - Bitfield tracking
9. `FileEntry.h/.cc` - File representation

### Tier 4 - Disk I/O (Must Have):
1. `DiskAdaptor.h/.cc` - Disk interface
2. `AbstractSingleDiskAdaptor.h/.cc`
3. `DirectDiskAdaptor.h/.cc`
4. `MultiDiskAdaptor.h/.cc`
5. `DiskWriter.h` - Writer interface
6. `AbstractDiskWriter.h/.cc`
7. `DefaultDiskWriter.h/.cc`
8. `BinaryStream.h`

### Tier 5 - Statistics (Should Have):
1. `TransferStat.h/.cc`
2. `NetStat.h/.cc`
3. `PeerStat.h/.cc`
4. `SpeedCalc.h/.cc`
5. `ServerStat.h/.cc`
6. `ServerStatMan.h/.cc`

### Tier 6 - Connection/Resume (Should Have):
1. Socket pooling (in DownloadEngine)
2. `SocketCore.h/.cc` - Socket abstraction
3. `SocketBuffer.h/.cc` - Buffering
4. `SocketRecvBuffer.h/.cc`
5. `BtProgressInfoFile.h` - Resume interface
6. `DefaultBtProgressInfoFile.h/.cc` - Resume implementation

### Tier 7 - Stream Processing (Should Have):
1. `StreamFilter.h/.cc`
2. `SinkStreamFilter.h/.cc`
3. `ChunkedDecodingStreamFilter.h/.cc`
4. `GZipDecodingStreamFilter.h/.cc` (optional)

### Tier 8 - Error Handling (Must Have):
1. `error_code.h`
2. `Exception.h`
3. `DlAbortEx.h/.cc`
4. `DlRetryEx.h/.cc`
5. `DownloadFailureException.h/.cc`
6. `RecoverableException.h`

### Tier 9 - RPC (Optional for Go port):
1. `RpcMethod.h/.cc`
2. `RpcMethodImpl.h/.cc`
3. HTTP server components
(Note: Go has excellent HTTP/JSON libraries, may be simpler to reimplement)

### Tier 10 - Utilities (Must Have):
1. `Timer.h/.cc` / `TimerA2.h`
2. `util.h/.cc` - Various utilities
3. `a2functional.h` - Functional helpers
4. `fmt.h/.cc` - Formatting
5. `Logger.h/.cc` / `LogFactory.h/.cc` - Logging
6. `CookieStorage.h/.cc` - HTTP cookies
7. `AuthConfig.h/.cc` - Authentication
8. `DNSCache.h/.cc` - DNS caching

---

## Key Design Patterns to Preserve in Go Port

1. **Command Pattern**: Each state transition is a Command object
2. **Event-Driven I/O**: Non-blocking sockets with event polling
3. **Segment-Based Parallelism**: File divided into independent segments
4. **Connection Pooling**: Reuse HTTP connections by host
5. **Hierarchical Statistics**: Per-connection -> Per-download -> Global
6. **Resume Checkpointing**: Periodic state persistence
7. **Filter Chain**: Chainable data transformations (chunked, gzip)
8. **Adaptive Selection**: Choose servers/URIs based on performance history

<task_metadata>
session_id: ses_3f0a4ed4affemKoFnxlhDLrlUJ
</task_metadata>
```
```

```
Tool: task

**Input:**
```json
{
  "description": "Explore aria2 options and config",
  "prompt": "Explore the aria2 codebase at /home/bhunter/repos/github/ari2-go/aria2/src to understand:\n\n1. Command-line option handling (OptionParser, OptionHandler)\n2. Configuration file parsing\n3. All HTTP-related options (connections, timeouts, headers, user-agent, etc.)\n4. Session management\n5. Download queue management\n\nLook at files like:\n- Option*.cc/h\n- Prefs*.cc/h\n- SessionSerializer*.cc/h\n- RequestGroupMan*.cc/h\n\nReturn a comprehensive list of all HTTP-relevant options and configuration mechanisms.",
  "subagent_type": "explore"
}
```
**Output:**
```
Now I have a comprehensive understanding of the aria2 codebase. Let me compile this information into a comprehensive report.

## Comprehensive Report: aria2 Options and Configuration Mechanisms

Based on my exploration of the aria2 codebase at `/home/bhunter/repos/github/ari2-go/aria2/src`, here is a comprehensive overview of all HTTP-relevant options and configuration mechanisms.

---

### 1. COMMAND-LINE OPTION HANDLING

#### Key Files:
- `/home/bhunter/repos/github/ari2-go/aria2/src/OptionParser.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/OptionParser.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/OptionHandler.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/AbstractOptionHandler.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/OptionHandlerImpl.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/OptionHandlerFactory.cc`

#### Architecture:
- **OptionParser**: Singleton class that manages all option handlers and parses command-line arguments and configuration files
- **OptionHandler**: Abstract interface for option handlers with methods like `parse()`, `getName()`, `getDefaultValue()`
- **AbstractOptionHandler**: Base implementation with flags for `hidden`, `eraseAfterParse`, `initialOption`, `changeOption`, `changeOptionForReserved`, `changeGlobalOption`, `cumulative`

#### Option Handler Types (from `OptionHandlerImpl.h`):
1. **BooleanOptionHandler**: For true/false options
2. **NumberOptionHandler**: For numeric values with min/max bounds
3. **UnitNumberOptionHandler**: For numbers with units (e.g., "16M", "1G")
4. **FloatNumberOptionHandler**: For floating-point values
5. **DefaultOptionHandler**: For string options
6. **CumulativeOptionHandler**: For options that can be specified multiple times (e.g., `--header`)
7. **ParameterOptionHandler**: For options with enumerated values
8. **HttpProxyOptionHandler**: Specialized handler for proxy URLs
9. **LocalFilePathOptionHandler**: For file path options
10. **ChecksumOptionHandler**: For checksum specifications
11. **DeprecatedOptionHandler**: Wrapper for deprecated options

#### Parsing Flow:
1. `OptionParser::parseArg()` - Parses command-line arguments using `getopt_long()`
2. `OptionParser::parse(Option&, istream&)` - Parses configuration file (key=value format)
3. `OptionParser::parse(Option&, KeyVals&)` - Parses from key-value pairs
4. `OptionParser::parseDefaultValues()` - Sets default values

---

### 2. CONFIGURATION FILE PARSING

#### Configuration File Format:
- Lines starting with `#` are comments
- Empty lines are ignored
- Format: `option-name=value` (no `--` prefix)
- Default config path: `~/.aria2/aria2.conf` (via `util::getConfigFile()`)

#### Related Options:
- `PREF_CONF_PATH` ("conf-path"): Custom config file path
- `PREF_NO_CONF` ("no-conf"): Disable config file loading

---

### 3. ALL HTTP-RELATED OPTIONS

#### From `prefs.h` and `OptionHandlerFactory.cc`:

##### HTTP Connection Options:
| Option Name | Pref Constant | Default | Description |
|-------------|---------------|---------|-------------|
| `timeout` | `PREF_TIMEOUT` | 60 | Overall timeout in seconds (1-600) |
| `connect-timeout` | `PREF_CONNECT_TIMEOUT` | 60 | Connection timeout in seconds (1-600) |
| `dns-timeout` | `PREF_DNS_TIMEOUT` | 30 | DNS lookup timeout (hidden, 1-60) |
| `max-tries` | `PREF_MAX_TRIES` | 5 | Maximum retry attempts (0=unlimited) |
| `retry-wait` | `PREF_RETRY_WAIT` | 0 | Wait between retries in seconds (0-600) |
| `max-connection-per-server` | `PREF_MAX_CONNECTION_PER_SERVER` | 1 | Max connections per server (1-16) |
| `split` | `PREF_SPLIT` | 5 | Number of connections per download |
| `min-split-size` | `PREF_MIN_SPLIT_SIZE` | 20M | Minimum size per split (1M-1G) |
| `lowest-speed-limit` | `PREF_LOWEST_SPEED_LIMIT` | 0 | Close connection if speed drops below |
| `max-file-not-found` | `PREF_MAX_FILE_NOT_FOUND` | 0 | Max 404 errors before giving up |
| `socket-recv-buffer-size` | `PREF_SOCKET_RECV_BUFFER_SIZE` | 0 | Socket receive buffer size (0-16M) |

##### HTTP Headers & User-Agent:
| Option Name | Pref Constant | Default | Description |
|-------------|---------------|---------|-------------|
| `user-agent` | `PREF_USER_AGENT` | "aria2/VERSION" | User-Agent string |
| `header` | `PREF_HEADER` | (none) | Custom headers (cumulative, can specify multiple) |
| `referer` | `PREF_REFERER` | (none) | Referer header value |

##### HTTP Behavior Options:
| Option Name | Pref Constant | Default | Description |
|-------------|---------------|---------|-------------|
| `enable-http-keep-alive` | `PREF_ENABLE_HTTP_KEEP_ALIVE` | true | Enable HTTP keep-alive |
| `enable-http-pipelining` | `PREF_ENABLE_HTTP_PIPELINING` | false | Enable HTTP pipelining |
| `max-http-pipelining` | `PREF_MAX_HTTP_PIPELINING` | 2 | Max pipelining requests (hidden, 1-8) |
| `http-no-cache` | `PREF_HTTP_NO_CACHE` | false | Send no-cache headers |
| `http-accept-gzip` | `PREF_HTTP_ACCEPT_GZIP` | false | Accept gzip encoding |
| `use-head` | `PREF_USE_HEAD` | false | Use HEAD for first request |
| `http-auth-challenge` | `PREF_HTTP_AUTH_CHALLENGE` | false | Wait for auth challenge |
| `conditional-get` | `PREF_CONDITIONAL_GET` | false | Use conditional GET with timestamps |
| `remote-time` | `PREF_REMOTE_TIME` | false | Apply remote file time to local file |
| `content-disposition-default-utf8` | `PREF_CONTENT_DISPOSITION_DEFAULT_UTF8` | false | Treat Content-Disposition as UTF-8 |
| `no-want-digest-header` | `PREF_NO_WANT_DIGEST_HEADER` | false | Don't send Want-Digest header |

##### HTTP Authentication:
| Option Name | Pref Constant | Default | Description |
|-------------|---------------|---------|-------------|
| `http-user` | `PREF_HTTP_USER` | (none) | HTTP username |
| `http-passwd` | `PREF_HTTP_PASSWD` | (none) | HTTP password |
| `no-netrc` | `PREF_NO_NETRC` | false | Disable netrc |
| `netrc-path` | `PREF_NETRC_PATH` | ~/.netrc | Path to netrc file |

##### HTTPS/TLS Options:
| Option Name | Pref Constant | Default | Description |
|-------------|---------------|---------|-------------|
| `check-certificate` | `PREF_CHECK_CERTIFICATE` | true | Verify server certificate |
| `ca-certificate` | `PREF_CA_CERTIFICATE` | (system) | CA certificate file |
| `certificate` | `PREF_CERTIFICATE` | (none) | Client certificate |
| `private-key` | `PREF_PRIVATE_KEY` | (none) | Client private key |
| `min-tls-version` | `PREF_MIN_TLS_VERSION` | TLSv1.2 | Minimum TLS version (TLSv1.1, TLSv1.2, TLSv1.3) |

##### HTTP Cookie Options:
| Option Name | Pref Constant | Default | Description |
|-------------|---------------|---------|-------------|
| `load-cookies` | `PREF_LOAD_COOKIES` | (none) | Load cookies from Netscape format file |
| `save-cookies` | `PREF_SAVE_COOKIES` | (none) | Save cookies to file |

##### HTTP Proxy Options:
| Option Name | Pref Constant | Default | Description |
|-------------|---------------|---------|-------------|
| `http-proxy` | `PREF_HTTP_PROXY` | (none) | HTTP proxy URL |
| `https-proxy` | `PREF_HTTPS_PROXY` | (none) | HTTPS proxy URL |
| `all-proxy` | `PREF_ALL_PROXY` | (none) | Proxy for all protocols |
| `no-proxy` | `PREF_NO_PROXY` | (none) | Comma-separated bypass list |
| `proxy-method` | `PREF_PROXY_METHOD` | get | Proxy method: get or tunnel |
| `http-proxy-user` | `PREF_HTTP_PROXY_USER` | (none) | HTTP proxy username |
| `http-proxy-passwd` | `PREF_HTTP_PROXY_PASSWD` | (none) | HTTP proxy password |
| `https-proxy-user` | `PREF_HTTPS_PROXY_USER` | (none) | HTTPS proxy username |
| `https-proxy-passwd` | `PREF_HTTPS_PROXY_PASSWD` | (none) | HTTPS proxy password |
| `all-proxy-user` | `PREF_ALL_PROXY_USER` | (none) | All proxy username |
| `all-proxy-passwd` | `PREF_ALL_PROXY_PASSWD` | (none) | All proxy password |

##### Speed/Limit Options:
| Option Name | Pref Constant | Default | Description |
|-------------|---------------|---------|-------------|
| `max-download-limit` | `PREF_MAX_DOWNLOAD_LIMIT` | 0 | Max download speed per download |
| `max-overall-download-limit` | `PREF_MAX_OVERALL_DOWNLOAD_LIMIT` | 0 | Max overall download speed |
| `max-resume-failure-tries` | `PREF_MAX_RESUME_FAILURE_TRIES` | 0 | Max resume failures |

##### URI Selection Options:
| Option Name | Pref Constant | Default | Description |
|-------------|---------------|---------|-------------|
| `uri-selector` | `PREF_URI_SELECTOR` | feedback | URI selection: inorder, feedback, adaptive |
| `reuse-uri` | `PREF_REUSE_URI` | true | Reuse already used URIs |
| `stream-piece-selector` | `PREF_STREAM_PIECE_SELECTOR` | default | Piece selection: default, inorder, random, geom |

---

### 4. SESSION MANAGEMENT

#### Key Files:
- `/home/bhunter/repos/github/ari2-go/aria2/src/SessionSerializer.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/SessionSerializer.cc`

#### Session Options:
| Option Name | Pref Constant | Default | Description |
|-------------|---------------|---------|-------------|
| `save-session` | `PREF_SAVE_SESSION` | (none) | File to save session on exit |
| `save-session-interval` | `PREF_SAVE_SESSION_INTERVAL` | 0 | Auto-save interval in seconds |
| `force-save` | `PREF_FORCE_SAVE` | false | Force save completed downloads |
| `save-not-found` | `PREF_SAVE_NOT_FOUND` | true | Save 404 errors to session |
| `keep-unfinished-download-result` | `PREF_KEEP_UNFINISHED_DOWNLOAD_RESULT` | true | Keep unfinished results |

#### Session File Format:
- URI lines followed by option lines
- Options prefixed with single space: ` option=value`
- Supports gzip compression (`.gz` extension)
- Stores GID, pause state, and all initial options

#### SessionSerializer Behavior:
- Saves active, waiting, and completed/errored downloads
- Filters based on `saveError_`, `saveInProgress_`, `saveWaiting_` flags
- Calculates SHA1 hash for change detection
- Handles metadata downloads (magnet URIs, torrent files)

---

### 5. DOWNLOAD QUEUE MANAGEMENT

#### Key Files:
- `/home/bhunter/repos/github/ari2-go/aria2/src/RequestGroupMan.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/RequestGroupMan.cc`

#### Queue Architecture:
- **requestGroups_**: Active downloads (`RequestGroupList`)
- **reservedGroups_**: Waiting/queued downloads (`RequestGroupList`)
- **downloadResults_**: Completed/errored downloads (`DownloadResultList`)
- **unfinishedDownloadResults_**: For session serialization

#### Queue Options:
| Option Name | Pref Constant | Default | Description |
|-------------|---------------|---------|-------------|
| `max-concurrent-downloads` | `PREF_MAX_CONCURRENT_DOWNLOADS` | 5 | Max simultaneous downloads |
| `optimize-concurrent-downloads` | `PREF_OPTIMIZE_CONCURRENT_DOWNLOADS` | false | Auto-optimize concurrency |
| `max-download-result` | `PREF_MAX_DOWNLOAD_RESULT` | 1000 | Max results to keep in memory |
| `pause` | `PREF_PAUSE` | false | Add download in paused state |
| `pause-metadata` | `PREF_PAUSE_METADATA` | false | Pause after metadata download |
| `auto-save-interval` | `PREF_AUTO_SAVE_INTERVAL` | 60 | Control file auto-save interval |

#### Queue Methods:
- `addRequestGroup()` / `addReservedGroup()`: Add downloads
- `insertReservedGroup(pos, groups)`: Insert at specific position
- `changeReservedGroupPosition(gid, pos, how)`: Move in queue (POS_SET, POS_CUR, POS_END)
- `removeReservedGroup(gid)`: Remove from waiting queue
- `fillRequestGroupFromReserver()`: Move from reserved to active
- `removeStoppedGroup()`: Clean up finished downloads

#### Concurrent Download Optimization:
Formula: `maxConcurrent = A + B * log10(speed * 8 / 1000000)`
- Default coefficients: A=5, B=25
- Adjusts based on measured download speed

---

### 6. OPTION TAGS (from `help_tags.h`)

Options are tagged for help organization:
- `TAG_BASIC`: Basic options
- `TAG_ADVANCED`: Advanced options
- `TAG_HTTP`: HTTP-specific options
- `TAG_HTTPS`: HTTPS-specific options
- `TAG_FTP`: FTP-specific options
- `TAG_METALINK`: Metalink options
- `TAG_BITTORRENT`: BitTorrent options
- `TAG_COOKIE`: Cookie options
- `TAG_HOOK`: Hook/callback options
- `TAG_FILE`: File-related options
- `TAG_RPC`: RPC options
- `TAG_CHECKSUM`: Checksum options
- `TAG_EXPERIMENTAL`: Experimental options
- `TAG_DEPRECATED`: Deprecated options

---

### 7. OPTION CLASS (`Option.h/cc`)

#### Features:
- Hierarchical options with parent fallback
- Bitfield for tracking which options are defined
- Methods: `put()`, `get()`, `getAsInt()`, `getAsLLInt()`, `getAsBool()`, `getAsDouble()`
- Local vs inherited option distinction (`definedLocal()` vs `defined()`)
- Merge support for combining option sets

---

### 8. HTTP REQUEST CONSTRUCTION (`HttpRequest.cc`)

#### How Options Are Used:
The `HttpRequest::createRequest()` method builds HTTP requests using:
- `userAgent_`: Set via `PREF_USER_AGENT`
- `headers_`: Custom headers from `PREF_HEADER`
- `noCache_`: From `PREF_HTTP_NO_CACHE`
- `acceptGzip_`: From `PREF_HTTP_ACCEPT_GZIP`
- `noWantDigest_`: From `PREF_NO_WANT_DIGEST_HEADER`
- Cookie storage, proxy settings, authentication

---

### Summary of Key Files:

| Purpose | Files |
|---------|-------|
| Option definitions | `prefs.h`, `prefs.cc` |
| Option parsing | `OptionParser.h/cc`, `OptionHandler.h` |
| Option handlers | `OptionHandlerImpl.h`, `AbstractOptionHandler.h/cc` |
| Option factory | `OptionHandlerFactory.cc` |
| Option storage | `Option.h/cc` |
| Session management | `SessionSerializer.h/cc` |
| Queue management | `RequestGroupMan.h/cc` |
| HTTP requests | `HttpRequest.h/cc`, `HttpConnection.h/cc` |
| Help tags | `help_tags.h/cc` |

<task_metadata>
session_id: ses_3f0a4e21effeFsHb9I05BTedjh
</task_metadata>
```
```

```
Tool: task

**Input:**
```json
{
  "description": "Explore aria2 RPC interface",
  "prompt": "Explore the aria2 codebase at /home/bhunter/repos/github/ari2-go/aria2/src to understand the RPC interface:\n\n1. JSON-RPC implementation\n2. XML-RPC implementation  \n3. WebSocket support\n4. All RPC methods available\n5. Authentication mechanism\n6. HTTP server implementation for RPC\n\nLook at files like:\n- *Rpc*.cc/h\n- *JsonRpc*.cc/h\n- *XmlRpc*.cc/h\n- HttpServer*.cc/h\n- WebSocket*.cc/h\n\nReturn a list of all RPC methods and their signatures that would need to be implemented.",
  "subagent_type": "explore"
}
```
**Output:**
```
Now I have a comprehensive understanding of the aria2 RPC interface. Let me compile this information into a comprehensive report.

## Summary: aria2 RPC Interface Analysis

Based on my exploration of the aria2 codebase at `/home/bhunter/repos/github/ari2-go/aria2/src`, here is a complete analysis of the RPC interface:

---

### 1. JSON-RPC Implementation

**Key Files:**
- `/home/bhunter/repos/github/ari2-go/aria2/src/rpc_helper.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/rpc_helper.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/json.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/json.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/JsonDiskWriter.h` (in glob results)
- `/home/bhunter/repos/github/ari2-go/aria2/src/ValueBaseJsonParser.h` (referenced)

**How it works:**
- JSON-RPC requests are parsed using `json::JsonDiskWriter` and `json::ValueBaseJsonParser`
- Endpoint: `POST /jsonrpc` for JSON-RPC, `GET /jsonrpc` for JSONP
- The `processJsonRpcRequest()` function in `rpc_helper.cc` processes JSON-RPC requests
- Supports batch requests (array of requests)
- Content-Type: `application/json-rpc` (or `text/javascript` for JSONP)

---

### 2. XML-RPC Implementation

**Key Files:**
- `/home/bhunter/repos/github/ari2-go/aria2/src/XmlRpcDiskWriter.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/XmlRpcDiskWriter.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/XmlRpcRequestParserStateMachine.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/XmlRpcRequestParserStateMachine.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/XmlRpcRequestParserController.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/XmlRpcRequestParserController.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/XmlRpcRequestParserState.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/XmlRpcRequestParserStateImpl.h`

**How it works:**
- Conditional compilation with `#ifdef ENABLE_XML_RPC`
- Endpoint: `POST /rpc`
- Uses XML parser with state machine for parsing XML-RPC requests
- Content-Type: `text/xml`
- The `xmlParseMemory()` function in `rpc_helper.cc` parses XML-RPC requests

---

### 3. WebSocket Support

**Key Files:**
- `/home/bhunter/repos/github/ari2-go/aria2/src/WebSocketSession.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/WebSocketSession.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/WebSocketInteractionCommand.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/WebSocketInteractionCommand.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/WebSocketResponseCommand.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/WebSocketResponseCommand.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/WebSocketSessionMan.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/WebSocketSessionMan.cc`

**How it works:**
- Conditional compilation with `#ifdef ENABLE_WEBSOCKET`
- Uses the `wslay` library for WebSocket protocol handling
- Endpoint: `GET /jsonrpc` with WebSocket upgrade headers
- WebSocket handshake validation in `HttpServerCommand.cc`:
  - Must be GET request
  - Must have `Sec-WebSocket-Key` header
  - Must have `Sec-WebSocket-Version: 13`
  - Request path must be `/jsonrpc`
- Server key generated using SHA-1 of client key + magic GUID
- Only supports JSON-RPC over WebSocket (not XML-RPC)
- Supports notifications (push messages to clients)

---

### 4. HTTP Server Implementation for RPC

**Key Files:**
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpServer.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpServer.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpServerCommand.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpServerCommand.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpServerBodyCommand.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpServerBodyCommand.cc`
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpServerResponseCommand.h`
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpServerResponseCommand.cc`

**Request Types (enum RequestType):**
```cpp
enum RequestType { RPC_TYPE_NONE, RPC_TYPE_XML, RPC_TYPE_JSON, RPC_TYPE_JSONP };
```

**Routing (from HttpServer.cc setupResponseRecv):**
- `GET /jsonrpc` -> RPC_TYPE_JSONP
- `POST /jsonrpc` -> RPC_TYPE_JSON
- `POST /rpc` -> RPC_TYPE_XML (when ENABLE_XML_RPC)

**Features:**
- HTTP Basic Authentication support
- CORS support with `Access-Control-Allow-Origin` header
- GZip compression support (when HAVE_ZLIB)
- Keep-Alive support
- SSL/TLS support (when ENABLE_SSL)
- Request size limiting via `--rpc-max-request-size`
- 30 second request timeout

---

### 5. Authentication Mechanism

**Key Files:**
- `/home/bhunter/repos/github/ari2-go/aria2/src/RpcMethod.cc` (token-based auth)
- `/home/bhunter/repos/github/ari2-go/aria2/src/HttpServer.cc` (HTTP Basic auth)

**Two authentication methods:**

1. **HTTP Basic Authentication** (in HttpServer.cc):
   - Uses `Authorization: Basic <base64>` header
   - Username/password set via `--rpc-user` and `--rpc-passwd` options
   - HMAC-based comparison for timing-safe password checking

2. **Token-based Authentication** (in RpcMethod.cc):
   - Token passed as first parameter with `token:` prefix
   - Example: `["token:mysecrettoken", "http://example.com/file.zip"]`
   - Token is automatically stripped from parameters before method processing
   - Set via `--rpc-secret` option
   - Validated by `DownloadEngine::validateToken()`

---

### 6. All RPC Methods and Their Signatures

**Core RPC Data Structures:**

```go
// RpcRequest
type RpcRequest struct {
    MethodName string
    Params     []interface{}  // List of parameters
    ID         interface{}    // Request ID (for JSON-RPC)
    JsonRpc    bool           // true for JSON-RPC, false for XML-RPC
}

// RpcResponse
type RpcResponse struct {
    Code       int            // 0 for success, non-zero for error
    Authorized AuthState      // AUTHORIZED or NOTAUTHORIZED
    Param      interface{}    // Result or error details
    ID         interface{}    // Request ID
}
```

---

### Complete List of RPC Methods with Signatures:

#### Download Management Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `aria2.addUri` | `addUri([secret], uris[], options{}, position)` | Add new download from URIs |
| `aria2.addTorrent` | `addTorrent([secret], torrent_base64, uris[], options{}, position)` | Add new download from torrent (ENABLE_BITTORRENT) |
| `aria2.addMetalink` | `addMetalink([secret], metalink_base64, options{}, position)` | Add new download from metalink (ENABLE_METALINK) |
| `aria2.remove` | `remove([secret], gid)` | Remove download |
| `aria2.forceRemove` | `forceRemove([secret], gid)` | Force remove download |
| `aria2.pause` | `pause([secret], gid)` | Pause download |
| `aria2.pauseAll` | `pauseAll([secret])` | Pause all downloads |
| `aria2.forcePause` | `forcePause([secret], gid)` | Force pause download |
| `aria2.forcePauseAll` | `forcePauseAll([secret])` | Force pause all downloads |
| `aria2.unpause` | `unpause([secret], gid)` | Unpause download |
| `aria2.unpauseAll` | `unpauseAll([secret])` | Unpause all downloads |

#### Status Query Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `aria2.tellStatus` | `tellStatus([secret], gid, keys[])` | Get download status |
| `aria2.tellActive` | `tellActive([secret], keys[])` | Get list of active downloads |
| `aria2.tellWaiting` | `tellWaiting([secret], offset, num, keys[])` | Get list of waiting downloads |
| `aria2.tellStopped` | `tellStopped([secret], offset, num, keys[])` | Get list of stopped downloads |
| `aria2.getUris` | `getUris([secret], gid)` | Get URIs for download |
| `aria2.getFiles` | `getFiles([secret], gid)` | Get files for download |
| `aria2.getPeers` | `getPeers([secret], gid)` | Get peers for download (ENABLE_BITTORRENT) |
| `aria2.getServers` | `getServers([secret], gid)` | Get servers for download |

#### Option Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `aria2.getOption` | `getOption([secret], gid)` | Get options for download |
| `aria2.changeOption` | `changeOption([secret], gid, options{})` | Change options for download |
| `aria2.getGlobalOption` | `getGlobalOption([secret])` | Get global options |
| `aria2.changeGlobalOption` | `changeGlobalOption([secret], options{})` | Change global options |

#### URI/Position Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `aria2.changeUri` | `changeUri([secret], gid, fileIndex, delUris[], addUris[], position)` | Change URIs for file |
| `aria2.changePosition` | `changePosition([secret], gid, pos, how)` | Change queue position. `how` is "POS_SET", "POS_CUR", or "POS_END" |

#### Session/System Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `aria2.getVersion` | `getVersion([secret])` | Get aria2 version |
| `aria2.getSessionInfo` | `getSessionInfo([secret])` | Get session info |
| `aria2.shutdown` | `shutdown([secret])` | Shutdown aria2 |
| `aria2.forceShutdown` | `forceShutdown([secret])` | Force shutdown aria2 |
| `aria2.getGlobalStat` | `getGlobalStat([secret])` | Get global statistics |
| `aria2.saveSession` | `saveSession([secret])` | Save session to file |
| `aria2.purgeDownloadResult` | `purgeDownloadResult([secret])` | Purge completed/error/removed downloads |
| `aria2.removeDownloadResult` | `removeDownloadResult([secret], gid)` | Remove specific download result |

#### System Methods (XML-RPC style)

| Method | Signature | Description |
|--------|-----------|-------------|
| `system.multicall` | `multicall([secret], methods[])` | Execute multiple methods in one call |
| `system.listMethods` | `listMethods()` | List all available methods |
| `system.listNotifications` | `listNotifications()` | List all available notifications |

---

### Notifications (WebSocket push events)

| Notification | Description |
|--------------|-------------|
| `aria2.onDownloadStart` | Download started |
| `aria2.onDownloadPause` | Download paused |
| `aria2.onDownloadStop` | Download stopped |
| `aria2.onDownloadComplete` | Download completed |
| `aria2.onDownloadError` | Download error |
| `aria2.onBtDownloadComplete` | BitTorrent download completed (ENABLE_BITTORRENT) |

---

### Status Response Fields (for tellStatus, tellActive, tellWaiting, tellStopped)

```go
type StatusResponse struct {
    GID             string   `json:"gid"`
    Status          string   `json:"status"`  // "active", "waiting", "paused", "error", "complete", "removed"
    TotalLength     string   `json:"totalLength"`
    CompletedLength string   `json:"completedLength"`
    UploadLength    string   `json:"uploadLength"`
    Bitfield        string   `json:"bitfield"`
    DownloadSpeed   string   `json:"downloadSpeed"`
    UploadSpeed     string   `json:"uploadSpeed"`
    InfoHash        string   `json:"infoHash"`  // BitTorrent only
    NumSeeders      string   `json:"numSeeders"` // BitTorrent only
    Seeder          string   `json:"seeder"`     // BitTorrent only
    PieceLength     string   `json:"pieceLength"`
    NumPieces       string   `json:"numPieces"`
    Connections     string   `json:"connections"`
    ErrorCode       string   `json:"errorCode"`
    ErrorMessage    string   `json:"errorMessage"`
    FollowedBy      []string `json:"followedBy"`
    Following       string   `json:"following"`
    BelongsTo       string   `json:"belongsTo"`
    Dir             string   `json:"dir"`
    Files           []File   `json:"files"`
    Bittorrent      *BTInfo  `json:"bittorrent"` // BitTorrent only
    VerifiedLength  string   `json:"verifiedLength"`
    VerifyIntegrityPending string `json:"verifyIntegrityPending"`
}

type File struct {
    Index           string `json:"index"`
    Path            string `json:"path"`
    Length          string `json:"length"`
    CompletedLength string `json:"completedLength"`
    Selected        string `json:"selected"` // "true" or "false"
    URIs            []URI  `json:"uris"`
}

type URI struct {
    URI    string `json:"uri"`
    Status string `json:"status"` // "used" or "waiting"
}

type BTInfo struct {
    AnnounceList [][]string `json:"announceList"`
    Comment      string     `json:"comment"`
    CreationDate int64      `json:"creationDate"`
    Mode         string     `json:"mode"` // "single" or "multi"
    Info         *BTInfoInner `json:"info"`
}

type BTInfoInner struct {
    Name string `json:"name"`
}
```

---

### Global Statistics Response

```go
type GlobalStatResponse struct {
    DownloadSpeed   string `json:"downloadSpeed"`
    UploadSpeed     string `json:"uploadSpeed"`
    NumActive       string `json:"numActive"`
    NumWaiting      string `json:"numWaiting"`
    NumStopped      string `json:"numStopped"`
    NumStoppedTotal string `json:"numStoppedTotal"`
}
```

---

### Version Response

```go
type VersionResponse struct {
    Version         string   `json:"version"`
    EnabledFeatures []string `json:"enabledFeatures"`
}
```

---

### Session Info Response

```go
type SessionInfoResponse struct {
    SessionID string `json:"sessionId"`
}
```

---

### JSON-RPC Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| -32700 | 500 | Parse error |
| -32600 | 400 | Invalid Request |
| -32601 | 404 | Method not found |
| -32602 | 500 | Invalid params |
| 1 | 400 | Error caught while executing RpcMethod |

<task_metadata>
session_id: ses_3f0a4e21bffeMTGcdkd7x8yKQo
</task_metadata>
```
```

---

## Assistant (Plan · gemini-claude-opus-4-5-thinking · 153.1s)

Now I have a comprehensive understanding of the aria2 codebase. Let me compile this into a complete Go porting plan.

---

# 🚀 Aria2-Go: Complete HTTP Download Manager Port Plan

## Overview

This is a comprehensive plan to port aria2c to Go, focusing **exclusively on native HTTP/HTTPS downloads** - no BitTorrent, no Magnet links, no Metalink, no FTP. The goal is a 1:1 feature port of all HTTP download capabilities.

---

## Project Scope

### ✅ **IN SCOPE (Must Implement)**
| Feature | Description |
|---------|-------------|
| **Segmented Downloads** | Split files into segments for parallel downloading |
| **Multi-connection Downloads** | Multiple connections per server |
| **Connection Pooling** | HTTP keep-alive and connection reuse |
| **Resume Support** | Pause/resume with `.aria2` control files |
| **HTTP/HTTPS** | Full HTTP/1.1 support with TLS |
| **HTTP Proxy** | CONNECT tunneling and direct proxy |
| **Cookies** | Netscape format cookie file support |
| **Authentication** | Basic/Digest HTTP auth |
| **Rate Limiting** | Per-download and global speed limits |
| **RPC Interface** | JSON-RPC 2.0 + WebSocket |
| **Session Management** | Save/restore download sessions |
| **Download Queue** | Priority-based queue management |
| **Adaptive Selection** | Choose best server/mirror based on speed |
| **gzip Support** | Accept-Encoding: gzip decompression |
| **Chunked Transfer** | HTTP chunked encoding support |
| **Checksum Verification** | MD5/SHA-1/SHA-256 verification |

### ❌ **OUT OF SCOPE (Will Not Implement)**
- BitTorrent protocol
- Magnet links
- DHT (Distributed Hash Table)
- Metalink format
- FTP/SFTP protocols
- UDP-based protocols

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLI / Config                             │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Download Engine                             │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │ RequestGroupMan │  │   Event Loop    │  │  Connection     │ │
│  │ (Queue Manager) │  │ (goroutines)    │  │     Pool        │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                                │
           ┌────────────────────┼────────────────────┐
           ▼                    ▼                    ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│  RequestGroup   │  │  RequestGroup   │  │  RequestGroup   │
│  (Download 1)   │  │  (Download 2)   │  │  (Download N)   │
└─────────────────┘  └─────────────────┘  └─────────────────┘
           │
           ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Segment Manager                              │
│  ┌───────┐ ┌───────┐ ┌───────┐ ┌───────┐ ┌───────┐ ┌───────┐  │
│  │ Seg 1 │ │ Seg 2 │ │ Seg 3 │ │ Seg 4 │ │ Seg 5 │ │ Seg N │  │
│  └───────┘ └───────┘ └───────┘ └───────┘ └───────┘ └───────┘  │
└─────────────────────────────────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────────────────────────────────┐
│                      HTTP Client Layer                           │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │  HTTP Request   │  │  HTTP Response  │  │ Stream Filters  │ │
│  │    Builder      │  │    Parser       │  │ (gzip, chunked) │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────────────────────────────────┐
│                       Disk I/O Layer                             │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │  Write Cache    │  │  Disk Adaptor   │  │  Control File   │ │
│  │  (buffered)     │  │  (pwrite)       │  │  (.aria2)       │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────────────────────────────────┐
│                        RPC Server                                │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │   JSON-RPC 2.0  │  │    WebSocket    │  │   Auth Layer    │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

---

## Package Structure

```
aria2-go/
├── cmd/
│   └── aria2go/
│       └── main.go                 # CLI entry point
├── internal/
│   ├── engine/
│   │   ├── engine.go               # DownloadEngine equivalent
│   │   ├── request_group.go        # RequestGroup - single download
│   │   ├── request_group_man.go    # RequestGroupMan - queue manager
│   │   ├── download_context.go     # Download metadata
│   │   └── gid.go                  # Global ID generation
│   ├── segment/
│   │   ├── segment.go              # Segment interface
│   │   ├── segment_man.go          # Segment manager
│   │   ├── piece.go                # Piece/block tracking
│   │   ├── bitfield.go             # BitfieldMan equivalent
│   │   └── piece_storage.go        # Piece storage interface
│   ├── http/
│   │   ├── client.go               # HTTP client wrapper
│   │   ├── request.go              # Request builder
│   │   ├── response.go             # Response parser
│   │   ├── header.go               # Header handling
│   │   ├── connection.go           # HTTP connection
│   │   ├── pool.go                 # Connection pool
│   │   └── proxy.go                # Proxy support
│   ├── disk/
│   │   ├── adaptor.go              # DiskAdaptor interface
│   │   ├── direct_adaptor.go       # Single file writer
│   │   ├── multi_adaptor.go        # Multi-file writer
│   │   ├── writer.go               # Low-level disk writer
│   │   └── cache.go                # Write cache
│   ├── control/
│   │   ├── control_file.go         # .aria2 file handling
│   │   └── session.go              # Session save/load
│   ├── stats/
│   │   ├── transfer_stat.go        # Transfer statistics
│   │   ├── speed_calc.go           # Speed calculation
│   │   ├── server_stat.go          # Per-server stats
│   │   └── console.go              # Console output
│   ├── rpc/
│   │   ├── server.go               # RPC HTTP server
│   │   ├── jsonrpc.go              # JSON-RPC handler
│   │   ├── websocket.go            # WebSocket handler
│   │   ├── methods.go              # RPC method implementations
│   │   ├── auth.go                 # Token/Basic auth
│   │   └── notifications.go        # WebSocket notifications
│   ├── filter/
│   │   ├── filter.go               # StreamFilter interface
│   │   ├── gzip.go                 # gzip decompression
│   │   └── chunked.go              # Chunked decoding
│   ├── cookie/
│   │   └── storage.go              # Cookie storage
│   ├── auth/
│   │   ├── config.go               # AuthConfig
│   │   └── netrc.go                # Netrc parser
│   └── util/
│       ├── uri.go                  # URI parsing
│       ├── hash.go                 # Hash calculations
│       ├── format.go               # Formatting utilities
│       └── timer.go                # Timer utilities
├── pkg/
│   ├── option/
│   │   ├── option.go               # Option storage
│   │   ├── handler.go              # Option handlers
│   │   ├── parser.go               # CLI/config parser
│   │   └── prefs.go                # Option constants
│   └── error/
│       └── codes.go                # Error codes
├── go.mod
├── go.sum
└── README.md
```

---

## Implementation Phases

### Phase 1: Core Foundation (Week 1-2)
**Goal:** Basic single-file HTTP download without segmentation

| Task | Priority | Files |
|------|----------|-------|
| Option system | High | `pkg/option/*.go` |
| URI parsing | High | `internal/util/uri.go` |
| HTTP client wrapper | High | `internal/http/client.go` |
| Basic HTTP request/response | High | `internal/http/request.go`, `response.go` |
| Single-file disk writer | High | `internal/disk/writer.go`, `direct_adaptor.go` |
| Download engine skeleton | High | `internal/engine/engine.go` |
| RequestGroup basics | High | `internal/engine/request_group.go` |
| GID generation | Medium | `internal/engine/gid.go` |
| Error codes | Medium | `pkg/error/codes.go` |

**Deliverable:** `aria2go download https://example.com/file.zip`

---

### Phase 2: Segmented Downloads (Week 3-4)
**Goal:** Parallel multi-segment downloads

| Task | Priority | Files |
|------|----------|-------|
| Bitfield tracking | High | `internal/segment/bitfield.go` |
| Piece management | High | `internal/segment/piece.go` |
| Segment manager | High | `internal/segment/segment_man.go` |
| Piece storage | High | `internal/segment/piece_storage.go` |
| Multi-connection per download | High | Engine updates |
| Range header support | High | `internal/http/request.go` |
| Speed calculator | Medium | `internal/stats/speed_calc.go` |
| Transfer stats | Medium | `internal/stats/transfer_stat.go` |

**Deliverable:** Downloads split into N parallel segments

---

### Phase 3: Resume & Control Files (Week 5)
**Goal:** Pause/resume functionality

| Task | Priority | Files |
|------|----------|-------|
| Control file format (.aria2) | High | `internal/control/control_file.go` |
| Save/load bitfield state | High | Control file integration |
| Resume on restart | High | Engine updates |
| Auto-save interval | Medium | Periodic save |
| File allocation | Medium | `internal/disk/adaptor.go` |

**Deliverable:** `Ctrl+C` then restart resumes download

---

### Phase 4: Connection Management (Week 6)
**Goal:** Connection pooling and keep-alive

| Task | Priority | Files |
|------|----------|-------|
| Connection pool | High | `internal/http/pool.go` |
| Keep-alive support | High | `internal/http/connection.go` |
| Connection timeout | High | Pool configuration |
| Eviction policy | Medium | Pool maintenance |
| DNS caching | Low | (Go stdlib handles) |

**Deliverable:** Connections reused across requests

---

### Phase 5: HTTP Features (Week 7)
**Goal:** Complete HTTP feature set

| Task | Priority | Files |
|------|----------|-------|
| HTTP proxy (CONNECT) | High | `internal/http/proxy.go` |
| gzip decompression | High | `internal/filter/gzip.go` |
| Chunked transfer decoding | High | `internal/filter/chunked.go` |
| Cookie support | High | `internal/cookie/storage.go` |
| HTTP authentication | High | `internal/auth/config.go` |
| Custom headers | Medium | Option integration |
| Conditional GET | Medium | If-Modified-Since |
| Content-Disposition | Medium | Filename extraction |

**Deliverable:** Full HTTP/1.1 compliance

---

### Phase 6: Rate Limiting & Queue (Week 8)
**Goal:** Speed control and queue management

| Task | Priority | Files |
|------|----------|-------|
| Per-download rate limit | High | `internal/engine/request_group.go` |
| Global rate limit | High | `internal/engine/request_group_man.go` |
| Download queue | High | `internal/engine/request_group_man.go` |
| Queue position changes | Medium | Queue management |
| Concurrent download limit | Medium | Engine config |
| Lowest speed limit | Medium | Stall detection |
| Server statistics | Low | `internal/stats/server_stat.go` |
| Adaptive URI selection | Low | URI selector |

**Deliverable:** Speed-controlled queued downloads

---

### Phase 7: RPC Server (Week 9-10)
**Goal:** Full JSON-RPC 2.0 API

| Task | Priority | Files |
|------|----------|-------|
| HTTP server for RPC | High | `internal/rpc/server.go` |
| JSON-RPC handler | High | `internal/rpc/jsonrpc.go` |
| aria2.addUri | High | `internal/rpc/methods.go` |
| aria2.remove/pause/unpause | High | Methods |
| aria2.tellStatus/Active/Waiting/Stopped | High | Methods |
| aria2.getOption/changeOption | High | Methods |
| aria2.getGlobalStat | High | Methods |
| aria2.shutdown | High | Methods |
| Token authentication | High | `internal/rpc/auth.go` |
| WebSocket support | Medium | `internal/rpc/websocket.go` |
| WebSocket notifications | Medium | `internal/rpc/notifications.go` |
| system.multicall | Medium | Methods |

**Deliverable:** RPC-controllable daemon mode

---

### Phase 8: Session Management (Week 11)
**Goal:** Save/restore download sessions

| Task | Priority | Files |
|------|----------|-------|
| Session file format | High | `internal/control/session.go` |
| Save session on exit | High | Session integration |
| Load session on start | High | Session integration |
| Auto-save interval | Medium | Periodic save |
| aria2.saveSession | Medium | RPC method |

**Deliverable:** `--save-session` and `--input-file` work

---

### Phase 9: Checksum & Verification (Week 12)
**Goal:** File integrity verification

| Task | Priority | Files |
|------|----------|-------|
| MD5 hash calculation | High | `internal/util/hash.go` |
| SHA-1 hash calculation | High | Hash utilities |
| SHA-256 hash calculation | High | Hash utilities |
| Whole-file checksum | High | Post-download verify |
| Per-piece checksum | Medium | Piece verification |
| Want-Digest header | Low | HTTP request |

**Deliverable:** `--checksum=sha-256=abc123` works

---

### Phase 10: CLI & Polish (Week 13-14)
**Goal:** Full CLI parity and testing

| Task | Priority | Files |
|------|----------|-------|
| Full CLI parser | High | `cmd/aria2go/main.go` |
| Config file support | High | Option loading |
| Console progress display | High | `internal/stats/console.go` |
| Quiet/verbose modes | Medium | Output control |
| Log file support | Medium | Logging |
| Signal handling | Medium | Graceful shutdown |
| Man page / help | Low | Documentation |

**Deliverable:** Feature-complete CLI

---

## Key Options to Implement

### Connection Options
```go
const (
    OptTimeout              = "timeout"               // 60 seconds
    OptConnectTimeout       = "connect-timeout"       // 60 seconds  
    OptMaxTries             = "max-tries"             // 5
    OptRetryWait            = "retry-wait"            // 0 seconds
    OptMaxConnPerServer     = "max-connection-per-server" // 1
    OptSplit                = "split"                 // 5
    OptMinSplitSize         = "min-split-size"        // 20M
    OptLowestSpeedLimit     = "lowest-speed-limit"    // 0 (disabled)
    OptMaxFileNotFound      = "max-file-not-found"    // 0
)
```

### HTTP Options
```go
const (
    OptUserAgent            = "user-agent"
    OptHeader               = "header"                // Cumulative
    OptReferer              = "referer"
    OptEnableHttpKeepAlive  = "enable-http-keep-alive"  // true
    OptEnableHttpPipelining = "enable-http-pipelining"  // false
    OptHttpNoCache          = "http-no-cache"           // false
    OptHttpAcceptGzip       = "http-accept-gzip"        // false
    OptConditionalGet       = "conditional-get"         // false
    OptRemoteTime           = "remote-time"             // false
)
```

### Authentication Options
```go
const (
    OptHttpUser             = "http-user"
    OptHttpPasswd           = "http-passwd"
    OptNoNetrc              = "no-netrc"              // false
    OptNetrcPath            = "netrc-path"            // ~/.netrc
)
```

### Proxy Options
```go
const (
    OptHttpProxy            = "http-proxy"
    OptHttpsProxy           = "https-proxy"
    OptAllProxy             = "all-proxy"
    OptNoProxy              = "no-proxy"
    OptProxyMethod          = "proxy-method"          // "get" or "tunnel"
)
```

### Download Options
```go
const (
    OptDir                  = "dir"                   // Current dir
    OptOut                  = "out"                   // Output filename
    OptMaxDownloadLimit     = "max-download-limit"    // 0 (unlimited)
    OptMaxOverallDownloadLimit = "max-overall-download-limit" // 0
    OptMaxConcurrentDownloads  = "max-concurrent-downloads"   // 5
    OptContinue             = "continue"              // false
    OptAutoFileRenaming     = "auto-file-renaming"    // true
)
```

### RPC Options
```go
const (
    OptEnableRpc            = "enable-rpc"            // false
    OptRpcListenPort        = "rpc-listen-port"       // 6800
    OptRpcListenAll         = "rpc-listen-all"        // false
    OptRpcSecret            = "rpc-secret"
    OptRpcMaxRequestSize    = "rpc-max-request-size"  // 2M
    OptRpcAllowOriginAll    = "rpc-allow-origin-all"  // false
)
```

### Session Options
```go
const (
    OptInputFile            = "input-file"
    OptSaveSession          = "save-session"
    OptSaveSessionInterval  = "save-session-interval" // 0
    OptConfPath             = "conf-path"             // ~/.aria2/aria2.conf
    OptNoConf               = "no-conf"               // false
)
```

---

## RPC Methods to Implement

### Core Methods (Must Have)
| Method | Parameters |
|--------|------------|
| `aria2.addUri` | `(secret, uris[], options{}, position?)` |
| `aria2.remove` | `(secret, gid)` |
| `aria2.forceRemove` | `(secret, gid)` |
| `aria2.pause` | `(secret, gid)` |
| `aria2.pauseAll` | `(secret)` |
| `aria2.forcePause` | `(secret, gid)` |
| `aria2.forcePauseAll` | `(secret)` |
| `aria2.unpause` | `(secret, gid)` |
| `aria2.unpauseAll` | `(secret)` |
| `aria2.tellStatus` | `(secret, gid, keys[]?)` |
| `aria2.tellActive` | `(secret, keys[]?)` |
| `aria2.tellWaiting` | `(secret, offset, num, keys[]?)` |
| `aria2.tellStopped` | `(secret, offset, num, keys[]?)` |
| `aria2.getUris` | `(secret, gid)` |
| `aria2.getFiles` | `(secret, gid)` |
| `aria2.getServers` | `(secret, gid)` |
| `aria2.getOption` | `(secret, gid)` |
| `aria2.changeOption` | `(secret, gid, options{})` |
| `aria2.getGlobalOption` | `(secret)` |
| `aria2.changeGlobalOption` | `(secret, options{})` |
| `aria2.getGlobalStat` | `(secret)` |
| `aria2.getVersion` | `(secret)` |
| `aria2.getSessionInfo` | `(secret)` |
| `aria2.shutdown` | `(secret)` |
| `aria2.forceShutdown` | `(secret)` |
| `aria2.saveSession` | `(secret)` |
| `aria2.purgeDownloadResult` | `(secret)` |
| `aria2.removeDownloadResult` | `(secret, gid)` |
| `aria2.changeUri` | `(secret, gid, fileIndex, delUris[], addUris[], position?)` |
| `aria2.changePosition` | `(secret, gid, pos, how)` |
| `system.multicall` | `(methods[])` |
| `system.listMethods` | `()` |
| `system.listNotifications` | `()` |

### WebSocket Notifications
| Notification | Payload |
|--------------|---------|
| `aria2.onDownloadStart` | `{gid}` |
| `aria2.onDownloadPause` | `{gid}` |
| `aria2.onDownloadStop` | `{gid}` |
| `aria2.onDownloadComplete` | `{gid}` |
| `aria2.onDownloadError` | `{gid}` |

---

## Go-Specific Design Decisions

### 1. Event Loop → Goroutines
aria2 uses a single-threaded event loop with epoll/kqueue. In Go, we'll use:
- **One goroutine per segment download**
- **Channels for coordination**
- **Context for cancellation**

```go
type DownloadEngine struct {
    ctx        context.Context
    cancel     context.CancelFunc
    groups     *RequestGroupManager
    connPool   *ConnectionPool
    wg         sync.WaitGroup
    statsCh    chan TransferStat
}
```

### 2. Command Pattern → State Machine
Instead of Command objects, use a state machine per segment:

```go
type SegmentState int
const (
    StateIdle SegmentState = iota
    StateConnecting
    StateSendingRequest
    StateReceivingHeaders
    StateDownloading
    StateComplete
    StateFailed
)
```

### 3. Socket I/O → net/http with Transport
Leverage Go's `http.Transport` with custom settings:

```go
transport := &http.Transport{
    MaxIdleConns:          100,
    MaxIdleConnsPerHost:   16,
    MaxConnsPerHost:       16,
    IdleConnTimeout:       15 * time.Second,
    DisableCompression:    !opts.AcceptGzip,
    Proxy:                 proxyFunc,
    DialContext:           dialFunc,
}
```

### 4. Disk I/O → pwrite syscall
For concurrent segment writes:

```go
func (w *DiskWriter) WriteAt(p []byte, off int64) (int, error) {
    return syscall.Pwrite(w.fd, p, off)
}
```

### 5. Rate Limiting → golang.org/x/time/rate
Use Go's rate limiter:

```go
limiter := rate.NewLimiter(rate.Limit(bytesPerSecond), burstSize)
limiter.WaitN(ctx, len(chunk))
```

---

## Dependencies

```go
// go.mod
module github.com/yourusername/aria2-go

go 1.21

require (
    github.com/spf13/cobra v1.8.0       // CLI framework
    github.com/gorilla/websocket v1.5.1  // WebSocket support
    golang.org/x/time v0.5.0             // Rate limiting
    golang.org/x/sync v0.6.0             // errgroup, semaphore
)
```

---

## Questions for You

Before I proceed with implementation, I need clarification on a few things:

```
Tool: question

**Input:**
```json
{
  "questions": [
    {
      "question": "What should the CLI binary be named?",
      "header": "Binary Name",
      "options": [
        {
          "label": "aria2go",
          "description": "Clear Go port naming"
        },
        {
          "description": "Hyphenated variant",
          "label": "aria2-go"
        },
        {
          "label": "argo",
          "description": "Short memorable name"
        },
        {
          "label": "goaria2",
          "description": "Go-prefixed naming"
        }
      ]
    },
    {
      "question": "Should I implement XML-RPC in addition to JSON-RPC, or JSON-RPC only?",
      "header": "RPC Protocol",
      "options": [
        {
          "label": "JSON-RPC only (Recommended)",
          "description": "Modern, simpler, what most clients use"
        },
        {
          "label": "Both JSON-RPC and XML-RPC",
          "description": "Full aria2 compatibility but more work"
        }
      ]
    },
    {
      "options": [
        {
          "label": "Use net/http (Recommended)",
          "description": "Go stdlib, battle-tested, handles most cases"
        },
        {
          "label": "Custom HTTP implementation",
          "description": "Full control but significant work"
        }
      ],
      "question": "What's your preferred approach for the HTTP client layer?",
      "header": "HTTP Client"
    },
    {
      "question": "Should I use any third-party CLI framework?",
      "header": "CLI Framework",
      "options": [
        {
          "label": "cobra (Recommended)",
          "description": "Industry standard, great for complex CLIs"
        },
        {
          "label": "urfave/cli",
          "description": "Alternative popular framework"
        },
        {
          "label": "stdlib flag only",
          "description": "No dependencies but more manual work"
        }
      ]
    },
    {
      "header": "Concurrency Limits",
      "options": [
        {
          "label": "Match aria2 defaults",
          "description": "5 concurrent downloads, 1-16 conn/server"
        },
        {
          "description": "More aggressive defaults for modern connections",
          "label": "Higher defaults"
        }
      ],
      "question": "What's your target for concurrent downloads and connections?"
    }
  ]
}
```
**Output:**
```
User has answered your questions: "What should the CLI binary be named?"="aria2go", "Should I implement XML-RPC in addition to JSON-RPC, or JSON-RPC only?"="JSON-RPC only (Recommended)", "What's your preferred approach for the HTTP client layer?"="Use net/http (Recommended)", "Should I use any third-party CLI framework?"="cobra (Recommended)", "What's your target for concurrent downloads and connections?"="Higher defaults". You can now continue with the user's answers in mind.
```
```

---

## Assistant (Plan · gemini-claude-opus-4-5-thinking · 58.3s)

Excellent choices! Let me finalize the plan with your preferences incorporated.

---

# 📋 Final Implementation Plan: `aria2go`

## Confirmed Design Decisions

| Decision | Choice |
|----------|--------|
| **Binary Name** | `aria2go` |
| **RPC Protocol** | JSON-RPC 2.0 only (no XML-RPC) |
| **HTTP Client** | Go stdlib `net/http` |
| **CLI Framework** | `cobra` |
| **Defaults** | Higher: 10 concurrent downloads, 1-32 connections/server |

---

## Updated Defaults (Higher Performance)

```go
const (
    DefaultMaxConcurrentDownloads = 10   // was 5 in aria2
    DefaultMaxConnPerServer       = 4    // was 1 in aria2
    DefaultMaxConnPerServerLimit  = 32   // was 16 in aria2
    DefaultSplit                  = 10   // was 5 in aria2
    DefaultMinSplitSize           = "10M" // was 20M in aria2
    DefaultTimeout                = 30   // was 60 in aria2 (faster timeout)
    DefaultConnectTimeout         = 15   // was 60 in aria2
)
```

---

## Complete Task Breakdown

### Phase 1: Foundation (Est. 3-4 days)
```
□ 1.1  Initialize Go module and project structure
□ 1.2  Implement option system (Option, OptionHandler, parser)
□ 1.3  Implement all HTTP-related option constants (prefs.go)
□ 1.4  Implement URI parsing utility
□ 1.5  Implement error codes
□ 1.6  Implement GID (Global ID) generation
□ 1.7  Create basic DownloadEngine skeleton
□ 1.8  Create basic RequestGroup skeleton
□ 1.9  Implement simple single-connection HTTP download
□ 1.10 Implement direct disk writer (single file)
□ 1.11 Basic CLI with cobra (aria2go download <url>)
```

### Phase 2: Segmented Downloads (Est. 4-5 days)
```
□ 2.1  Implement BitfieldMan for tracking downloaded pieces
□ 2.2  Implement Piece struct with block-level tracking
□ 2.3  Implement PieceStorage interface
□ 2.4  Implement DefaultPieceStorage
□ 2.5  Implement UnknownLengthPieceStorage (streaming)
□ 2.6  Implement Segment interface
□ 2.7  Implement SegmentMan (segment coordinator)
□ 2.8  Add Range header support to HTTP requests
□ 2.9  Implement parallel segment goroutines
□ 2.10 Implement segment assignment logic
□ 2.11 Implement segment completion/cancellation
□ 2.12 Implement SpeedCalc (rolling window speed)
□ 2.13 Implement TransferStat
□ 2.14 Wire segments to RequestGroup
```

### Phase 3: Resume & Control Files (Est. 2-3 days)
```
□ 3.1  Define .aria2 control file binary format
□ 3.2  Implement ControlFile save()
□ 3.3  Implement ControlFile load()
□ 3.4  Implement auto-save on interval
□ 3.5  Implement resume from control file on start
□ 3.6  Implement graceful shutdown with save
□ 3.7  Clean up control file on successful completion
```

### Phase 4: Connection Management (Est. 2-3 days)
```
□ 4.1  Create custom http.Transport configuration
□ 4.2  Implement connection pool with timeouts
□ 4.3  Implement keep-alive management
□ 4.4  Implement connection eviction
□ 4.5  Add --enable-http-keep-alive option
□ 4.6  Implement connection reuse metrics
```

### Phase 5: HTTP Features (Est. 3-4 days)
```
□ 5.1  Implement HTTP proxy support (CONNECT tunnel)
□ 5.2  Implement direct proxy (GET through proxy)
□ 5.3  Implement proxy authentication
□ 5.4  Implement --no-proxy bypass list
□ 5.5  Implement gzip Accept-Encoding and decompression
□ 5.6  Implement chunked transfer decoding (via stdlib)
□ 5.7  Implement Netscape cookie file loading
□ 5.8  Implement cookie saving
□ 5.9  Implement HTTP Basic authentication
□ 5.10 Implement netrc file parsing
□ 5.11 Implement custom headers (--header option)
□ 5.12 Implement conditional GET (If-Modified-Since)
□ 5.13 Implement Content-Disposition filename extraction
□ 5.14 Implement --remote-time (set file mtime)
```

### Phase 6: Rate Limiting & Queue (Est. 2-3 days)
```
□ 6.1  Implement per-download rate limiter
□ 6.2  Implement global rate limiter
□ 6.3  Implement RequestGroupMan (queue manager)
□ 6.4  Implement download queue (reserved/active)
□ 6.5  Implement queue position changes
□ 6.6  Implement --max-concurrent-downloads
□ 6.7  Implement --lowest-speed-limit (stall detection)
□ 6.8  Implement retry logic with backoff
□ 6.9  Implement --max-tries and --retry-wait
□ 6.10 Implement ServerStat for adaptive selection
□ 6.11 Implement URI selector (inorder, feedback, adaptive)
```

### Phase 7: RPC Server (Est. 4-5 days)
```
□ 7.1  Implement RPC HTTP server
□ 7.2  Implement JSON-RPC 2.0 request parsing
□ 7.3  Implement JSON-RPC 2.0 response formatting
□ 7.4  Implement token authentication (--rpc-secret)
□ 7.5  Implement aria2.addUri
□ 7.6  Implement aria2.remove, aria2.forceRemove
□ 7.7  Implement aria2.pause, aria2.forcePause, aria2.pauseAll
□ 7.8  Implement aria2.unpause, aria2.unpauseAll
□ 7.9  Implement aria2.tellStatus
□ 7.10 Implement aria2.tellActive, tellWaiting, tellStopped
□ 7.11 Implement aria2.getUris, getFiles, getServers
□ 7.12 Implement aria2.getOption, changeOption
□ 7.13 Implement aria2.getGlobalOption, changeGlobalOption
□ 7.14 Implement aria2.getGlobalStat
□ 7.15 Implement aria2.getVersion, getSessionInfo
□ 7.16 Implement aria2.shutdown, forceShutdown
□ 7.17 Implement aria2.changePosition, changeUri
□ 7.18 Implement aria2.purgeDownloadResult, removeDownloadResult
□ 7.19 Implement system.multicall, listMethods
□ 7.20 Implement WebSocket upgrade
□ 7.21 Implement WebSocket JSON-RPC handling
□ 7.22 Implement WebSocket notifications
□ 7.23 Add CORS support (--rpc-allow-origin-all)
```

### Phase 8: Session Management (Est. 2 days)
```
□ 8.1  Define session file format
□ 8.2  Implement session save (--save-session)
□ 8.3  Implement session load (--input-file)
□ 8.4  Implement auto-save interval
□ 8.5  Implement aria2.saveSession RPC method
□ 8.6  Handle completed/errored downloads in session
```

### Phase 9: Checksum & Verification (Est. 2 days)
```
□ 9.1  Implement MD5 hash calculation
□ 9.2  Implement SHA-1 hash calculation
□ 9.3  Implement SHA-256 hash calculation
□ 9.4  Implement --checksum option
□ 9.5  Implement post-download verification
□ 9.6  Implement Want-Digest header support
```

### Phase 10: CLI Polish (Est. 3-4 days)
```
□ 10.1  Implement all CLI options via cobra
□ 10.2  Implement config file loading (--conf-path)
□ 10.3  Implement console progress display
□ 10.4  Implement --quiet, --console-log-level
□ 10.5  Implement log file (--log)
□ 10.6  Implement signal handling (SIGINT, SIGTERM)
□ 10.7  Implement --help for all options
□ 10.8  Implement daemon mode (--daemon)
□ 10.9  Write comprehensive tests
□ 10.10 Write README documentation
```

---

## Estimated Timeline

| Phase | Duration | Cumulative |
|-------|----------|------------|
| Phase 1: Foundation | 3-4 days | Week 1 |
| Phase 2: Segmented Downloads | 4-5 days | Week 2 |
| Phase 3: Resume & Control | 2-3 days | Week 2-3 |
| Phase 4: Connection Mgmt | 2-3 days | Week 3 |
| Phase 5: HTTP Features | 3-4 days | Week 4 |
| Phase 6: Rate Limiting & Queue | 2-3 days | Week 4-5 |
| Phase 7: RPC Server | 4-5 days | Week 5-6 |
| Phase 8: Session Management | 2 days | Week 6 |
| Phase 9: Checksum | 2 days | Week 7 |
| Phase 10: CLI Polish | 3-4 days | Week 7-8 |

**Total: ~6-8 weeks** for a full feature-complete port

---

## Final Dependencies

```go
// go.mod
module github.com/yourusername/aria2go

go 1.22

require (
    github.com/spf13/cobra v1.8.0
    github.com/gorilla/websocket v1.5.1
    golang.org/x/time v0.5.0
    golang.org/x/sync v0.6.0
)
```

---

## Ready to Proceed?

This plan covers a **complete 1:1 port** of aria2's HTTP download functionality to Go. The implementation is broken into 10 phases with clear deliverables.

Would you like me to start implementing **Phase 1 (Foundation)** now? This will give you:
- Working project structure
- Option system
- Basic single-file HTTP download: `aria2go download https://example.com/file.zip`

---

