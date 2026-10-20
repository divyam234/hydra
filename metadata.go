package hydra

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const metaVersionBitfield = 3

// Bound untrusted remote sizes and sidecars, including per-block SHA-256 data.
const maxResumeMetadataBytes = 64 << 20

type pieceState string

const (
	piecePending pieceState = "pending"
	pieceDone    pieceState = "done"
)

// piece is a scheduled HTTP range job. In metadata v2 it maps to one fixed-size
// resume block, not to a long-lived connection chunk.
type piece struct {
	Index int        `json:"index"`
	Start int64      `json:"start"`
	End   int64      `json:"end"` // inclusive
	State pieceState `json:"state,omitempty"`

	// Legacy v1 fields. They are still decoded so old sidecars can be migrated to
	// v2 bitfields, but new sidecars do not rely on partial per-piece progress.
	Downloaded int64 `json:"downloaded,omitempty"`
	Done       bool  `json:"done,omitempty"`
}

func (p piece) size() int64 {
	if p.End < p.Start {
		return 0
	}
	return p.End - p.Start + 1
}

func (p piece) downloadedBytes() int64 {
	sz := p.size()
	if p.Done {
		return sz
	}
	if p.Downloaded < 0 {
		return 0
	}
	if p.Downloaded > sz {
		return sz
	}
	return p.Downloaded
}

type metaFile struct {
	Version      int       `json:"version"`
	URLs         []string  `json:"urls"`
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	ETag         string    `json:"etag,omitempty"`
	LastModified string    `json:"last_modified,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// Metadata v3 uses a compact bitfield: one bit per fixed-size
	// resume block. A set bit means the complete byte range for that block is
	// already durable on disk, accompanied by its SHA-256. This avoids JSON piece arrays and makes
	// resume independent of the current split/concurrency setting.
	BlockSize   int64  `json:"block_size,omitempty"`
	BlockCount  int    `json:"block_count,omitempty"`
	Bitfield    string `json:"bitfield,omitempty"`
	BlockHashes string `json:"block_hashes,omitempty"`
	Single      bool   `json:"single,omitempty"`
	PrefixSize  int64  `json:"prefix_size,omitempty"`
	PrefixHash  string `json:"prefix_sha256,omitempty"`

	// Pieces is retained only for v1 sidecar migration and omitted for new files.
	Pieces []piece `json:"pieces,omitempty"`

	bits     []byte     `json:"-"`
	hashes   []byte     `json:"-"`
	dirty    bool       `json:"-"`
	lastSave time.Time  `json:"-"`
	mu       sync.Mutex `json:"-"`
}

func newMeta(urls []string, path string, info probeInfo, blockSize int64) *metaFile {
	if blockSize <= 0 {
		blockSize = DefaultResumeBlockSize
	}
	now := time.Now().UTC()
	count := blockCount(info.Size, blockSize)
	m := &metaFile{
		Version:      metaVersionBitfield,
		URLs:         urls,
		Path:         path,
		Size:         info.Size,
		ETag:         info.ETag,
		LastModified: info.LastModified,
		CreatedAt:    now,
		UpdatedAt:    now,
		BlockSize:    blockSize,
		BlockCount:   count,
		bits:         make([]byte, bitfieldBytes(count)),
		lastSave:     now,
	}
	m.syncBitfield()
	return m
}

// splitPieces is kept for tests and legacy migration. New downloads schedule
// large HTTP range jobs with metaFile.pendingPieces while using fixed-size
// resume blocks only for bitfield checkpointing.
func splitPieces(size int64, n int) []piece {
	if n < 1 {
		n = 1
	}
	if int64(n) > size && size > 0 {
		n = int(size)
	}
	pieces := make([]piece, 0, n)
	base := size / int64(n)
	rem := size % int64(n)
	var off int64
	for i := 0; i < n; i++ {
		sz := base
		if int64(i) < rem {
			sz++
		}
		start := off
		end := off + sz - 1
		pieces = append(pieces, piece{Index: i, Start: start, End: end, State: piecePending})
		off += sz
	}
	return pieces
}

func blockCount(size, blockSize int64) int {
	if size <= 0 {
		return 0
	}
	if blockSize <= 0 {
		blockSize = DefaultResumeBlockSize
	}
	return int((size-1)/blockSize + 1)
}

func bitfieldBytes(count int) int {
	if count <= 0 {
		return 0
	}
	return (count-1)/8 + 1
}

func setBit(b []byte, i int) {
	if i < 0 || i/8 >= len(b) {
		return
	}
	b[i/8] |= 1 << uint(i%8)
}

func clearBit(b []byte, i int) {
	if i < 0 || i/8 >= len(b) {
		return
	}
	b[i/8] &^= 1 << uint(i%8)
}

func testBit(b []byte, i int) bool {
	if i < 0 || i/8 >= len(b) {
		return false
	}
	return b[i/8]&(1<<uint(i%8)) != 0
}

func countSetBits(b []byte, maxBits int) int {
	if maxBits <= 0 {
		return 0
	}
	full := maxBits / 8
	rem := maxBits % 8
	n := 0
	for i := 0; i < full && i < len(b); i++ {
		n += bits.OnesCount8(b[i])
	}
	if rem > 0 && full < len(b) {
		mask := byte((1 << uint(rem)) - 1)
		n += bits.OnesCount8(b[full] & mask)
	}
	return n
}

func (m *metaFile) syncBitfield() {
	if m.BlockCount <= 0 || len(m.bits) == 0 {
		m.Bitfield = ""
		return
	}
	m.Bitfield = base64.StdEncoding.EncodeToString(m.bits)
}

func (m *metaFile) ensureBits() {
	if m.BlockSize <= 0 {
		m.BlockSize = DefaultResumeBlockSize
	}
	m.BlockCount = blockCount(m.Size, m.BlockSize)
	need := bitfieldBytes(m.BlockCount)
	if len(m.bits) != need {
		old := m.bits
		m.bits = make([]byte, need)
		copy(m.bits, old)
	}
	// Clear padding bits so old/corrupt metadata cannot inflate progress.
	for i := m.BlockCount; i < len(m.bits)*8; i++ {
		clearBit(m.bits, i)
	}
}

func (m *metaFile) completedBytes() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Version == metaVersionBitfield {
		m.ensureBits()
		var n int64
		for i := 0; i < m.BlockCount; i++ {
			if testBit(m.bits, i) {
				n += m.blockSize(i)
			}
		}
		return n
	}
	var n int64
	for _, p := range m.Pieces {
		n += p.downloadedBytes()
	}
	return n
}

func (m *metaFile) blockSize(i int) int64 {
	if i < 0 || i >= m.BlockCount || m.BlockSize <= 0 {
		return 0
	}
	start := int64(i) * m.BlockSize
	if start >= m.Size {
		return 0
	}
	return min(m.BlockSize, m.Size-start)
}

func (m *metaFile) donePieces() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Version == metaVersionBitfield {
		m.ensureBits()
		return countSetBits(m.bits, m.BlockCount)
	}
	var n int
	for _, p := range m.Pieces {
		if p.Done {
			n++
		}
	}
	return n
}

func (m *metaFile) piecesTotal() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Version == metaVersionBitfield {
		m.ensureBits()
		return m.BlockCount
	}
	return len(m.Pieces)
}

// pendingPieces returns HTTP range jobs for missing data. ResumeBlockSize only
// controls the bitfield/checkpoint granularity; it does not directly decide the
// size of every HTTP Range request. Range request sizes are planned from Split
// and MinSplitSize, then rounded to resume-block boundaries so a completed bit
// always represents durable bytes on disk.
func (m *metaFile) pendingPieces(split int, minSplitSize int64) []piece {
	m.mu.Lock()
	defer m.mu.Unlock()
	if split < 1 {
		split = 1
	}
	if minSplitSize < 1 {
		minSplitSize = 1
	}
	if m.Version != metaVersionBitfield {
		out := make([]piece, 0)
		for _, p := range m.Pieces {
			if !p.Done && p.downloadedBytes() < p.size() {
				out = append(out, p)
			}
		}
		return out
	}
	m.ensureBits()
	if m.BlockCount == 0 {
		return nil
	}

	totalMissingBlocks := 0
	for i := 0; i < m.BlockCount; i++ {
		if !testBit(m.bits, i) {
			totalMissingBlocks++
		}
	}
	if totalMissingBlocks == 0 {
		return nil
	}

	targetBlocksBySplit := (totalMissingBlocks-1)/split + 1
	targetBlocksByMinSize := int((minSplitSize-1)/m.BlockSize + 1)
	if targetBlocksByMinSize < 1 {
		targetBlocksByMinSize = 1
	}
	targetBlocks := max(targetBlocksBySplit, targetBlocksByMinSize)
	targetBlocks = min(targetBlocks, m.BlockCount)
	if targetBlocks < 1 {
		targetBlocks = 1
	}

	out := make([]piece, 0, min(split, totalMissingBlocks))
	jobIndex := 0
	for i := 0; i < m.BlockCount; {
		for i < m.BlockCount && testBit(m.bits, i) {
			i++
		}
		if i >= m.BlockCount {
			break
		}
		runStart := i
		for i < m.BlockCount && !testBit(m.bits, i) {
			i++
		}
		runEnd := i // exclusive
		for b := runStart; b < runEnd; {
			endBlock := b + targetBlocks
			if endBlock > runEnd {
				endBlock = runEnd
			}
			start := int64(b) * m.BlockSize
			end := m.Size - 1
			if endBlock < m.BlockCount {
				end = int64(endBlock)*m.BlockSize - 1
			}
			out = append(out, piece{Index: jobIndex, Start: start, End: end, State: piecePending})
			jobIndex++
			b = endBlock
		}
	}
	return out
}

// markRangeComplete sets all resume-block bits whose bytes are fully covered by
// [start, end] and returns the number of newly completed blocks. It marks the metadata dirty but does not fsync immediately; callers use
// saveIfDue to batch sidecar writes and avoid blocking every copy buffer.
func (m *metaFile) markRangeComplete(start, end int64, sidecar string) (int, error) {
	return m.markRangeCompleteFromFile(nil, start, end, sidecar)
}

func (m *metaFile) markRangeCompleteFromFile(f *os.File, start, end int64, sidecar string) (int, error) {
	_ = sidecar // kept in the signature for older internal call sites; saving is batched by saveIfDue.
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Version != metaVersionBitfield {
		return 0, nil
	}
	m.ensureBits()
	if start < 0 {
		start = 0
	}
	if end >= m.Size {
		end = m.Size - 1
	}
	if end < start || m.BlockSize <= 0 {
		return 0, nil
	}
	first := int(start / m.BlockSize)
	last := int(end / m.BlockSize)
	changed := 0
	for i := first; i <= last && i < m.BlockCount; i++ {
		blockStart := int64(i) * m.BlockSize
		blockEnd := blockStart + m.blockSize(i) - 1
		if start <= blockStart && end >= blockEnd && !testBit(m.bits, i) {
			if f != nil {
				digest, err := hashFileBlock(f, blockStart, m.blockSize(i))
				if err != nil {
					return changed, err
				}
				m.ensureHashes()
				copy(m.hashes[i*sha256.Size:], digest[:])
			}
			setBit(m.bits, i)
			changed++
		}
	}
	if changed == 0 {
		return 0, nil
	}
	m.UpdatedAt = time.Now().UTC()
	m.dirty = true
	return changed, nil
}

func (m *metaFile) saveIfDue(sidecar string, interval time.Duration) error {
	return m.saveIfDueAfterSync(sidecar, interval, nil)
}

func (m *metaFile) saveIfDueAfterSync(sidecar string, interval time.Duration, syncData func() error) error {
	now := time.Now().UTC()
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.dirty {
		return nil
	}
	if interval > 0 && !m.lastSave.IsZero() && now.Sub(m.lastSave) < interval {
		return nil
	}
	// Hold the metadata lock across data sync and snapshotting: no new completed
	// bits may be published until their file bytes have been made durable.
	if syncData != nil {
		if err := syncData(); err != nil {
			return err
		}
	}
	m.UpdatedAt = now
	if err := m.saveLocked(sidecar); err != nil {
		return err
	}
	m.dirty = false
	m.lastSave = now
	return nil
}

func (m *metaFile) markDone(index int, sidecar string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	if m.Version == metaVersionBitfield {
		m.ensureBits()
		// Internal helper used by fixtures and legacy callers. Production marks
		// blocks via markRangeCompleteFromFile with its already-open output.
		if f, err := os.Open(m.Path); err == nil {
			digest, hashErr := hashFileBlock(f, int64(index)*m.BlockSize, m.blockSize(index))
			_ = f.Close()
			if hashErr != nil {
				return hashErr
			}
			m.ensureHashes()
			copy(m.hashes[index*sha256.Size:], digest[:])
		}
		setBit(m.bits, index)
		m.syncBitfield()
		m.UpdatedAt = now
		if err := m.saveLocked(sidecar); err != nil {
			return err
		}
		m.dirty = false
		m.lastSave = now
		return nil
	}
	for i := range m.Pieces {
		if m.Pieces[i].Index == index {
			m.Pieces[i].Downloaded = m.Pieces[i].size()
			m.Pieces[i].Done = true
			m.Pieces[i].State = pieceDone
			break
		}
	}
	m.UpdatedAt = now
	if err := m.saveLocked(sidecar); err != nil {
		return err
	}
	m.dirty = false
	m.lastSave = now
	return nil
}

func (m *metaFile) compatible(path string, info probeInfo) bool {
	if m.Path != path || m.Size != info.Size {
		return false
	}
	if m.Version != metaVersionBitfield || m.BlockSize <= 0 || m.BlockCount <= 0 || len(m.bits) != bitfieldBytes(m.BlockCount) {
		return false
	}
	// Never mix known different representations, even in the default mode.
	// Disappearing validators are also a loss of the old identity guarantee.
	if m.ETag != info.ETag || m.LastModified != info.LastModified {
		return false
	}
	if !strongETag(info.ETag) {
		return false
	}
	return true
}

func loadMeta(sidecar string) (*metaFile, error) {
	f, err := os.Open(sidecar)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	const maxSidecarBytes = 2 * maxResumeMetadataBytes
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if st.Size() > maxSidecarBytes {
		return nil, errors.New("resume sidecar exceeds 128 MiB")
	}
	b, err := io.ReadAll(io.LimitReader(f, maxSidecarBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxSidecarBytes {
		return nil, errors.New("resume sidecar exceeds 128 MiB")
	}
	var m metaFile
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m.Version != 0 && m.Version != 1 && m.Version != 2 && m.Version != metaVersionBitfield {
		return nil, errors.New("unsupported resume metadata version")
	}
	if err := validateMetadataSize(m.Size, m.BlockSize); err != nil {
		return nil, err
	}
	m.normalize()
	return &m, nil
}

func validateMetadataSize(size, blockSize int64) error {
	if size < 0 {
		return errors.New("negative resume metadata size")
	}
	if blockSize <= 0 {
		blockSize = DefaultResumeBlockSize
	}
	if size > 0 && (size-1)/blockSize >= int64(maxResumeMetadataBytes)*8 {
		return errors.New("resume bitfield exceeds 64 MiB; increase ResumeBlockSize")
	}
	if size > 0 && (size-1)/blockSize >= int64(maxResumeMetadataBytes)/(sha256.Size+1) {
		return errors.New("resume integrity metadata exceeds 64 MiB; increase ResumeBlockSize")
	}
	return nil
}

func (m *metaFile) normalize() {
	m.lastSave = time.Now().UTC()
	if m.Version == metaVersionBitfield {
		if m.Bitfield != "" {
			if decoded, err := base64.StdEncoding.DecodeString(m.Bitfield); err == nil {
				m.bits = decoded
			}
		}
		if decoded, err := base64.StdEncoding.DecodeString(m.BlockHashes); err == nil && len(decoded) == blockCount(m.Size, m.BlockSize)*sha256.Size {
			m.hashes = decoded
		}
		m.ensureBits()
		return
	}
	// Old checkpoints did not authenticate completed blocks. Restart rather
	// than carrying forward potentially corrupted data from those versions.
	m.Version = metaVersionBitfield
	m.bits = nil
	m.Pieces = nil
	m.ensureBits()
}

func (m *metaFile) migrateLegacyPieces() {
	if m.BlockSize <= 0 {
		m.BlockSize = DefaultResumeBlockSize
	}
	m.Version = metaVersionBitfield
	m.BlockCount = blockCount(m.Size, m.BlockSize)
	m.bits = make([]byte, bitfieldBytes(m.BlockCount))
	for _, p := range m.Pieces {
		completeUntil := p.Start + p.downloadedBytes()
		for i := 0; i < m.BlockCount; i++ {
			blockStart := int64(i) * m.BlockSize
			blockEnd := blockStart + m.blockSize(i)
			if blockStart >= p.Start && blockEnd <= completeUntil {
				setBit(m.bits, i)
			}
		}
	}
	m.Pieces = nil
	m.ensureBits()
}

func (m *metaFile) save(sidecar string) error {
	now := time.Now().UTC()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UpdatedAt = now
	if err := m.saveLocked(sidecar); err != nil {
		return err
	}
	m.dirty = false
	m.lastSave = now
	return nil
}

func (m *metaFile) saveLocked(sidecar string) error {
	if m.Version == metaVersionBitfield {
		m.ensureBits()
		m.syncBitfield()
		m.BlockHashes = base64.StdEncoding.EncodeToString(m.hashes)
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := sidecar + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, sidecar); err != nil {
		return err
	}
	return syncDir(filepath.Dir(sidecar))
}

func (m *metaFile) ensureHashes() {
	if len(m.hashes) != m.BlockCount*sha256.Size {
		m.hashes = make([]byte, m.BlockCount*sha256.Size)
	}
}

func hashFileBlock(f *os.File, start, size int64) ([sha256.Size]byte, error) {
	h := sha256.New()
	n, err := io.Copy(h, io.NewSectionReader(f, start, size))
	if err == nil && n != size {
		err = io.ErrUnexpectedEOF
	}
	var digest [sha256.Size]byte
	copy(digest[:], h.Sum(nil))
	return digest, err
}

// Never trust a completion bit without checking the actual saved bytes. A
// damaged block is made pending and overwritten, not published or skipped.
func (m *metaFile) verifyCompletedBlocks(ctx context.Context, f *os.File) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := 0; i < m.BlockCount; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !testBit(m.bits, i) {
			continue
		}
		valid := len(m.hashes) == m.BlockCount*sha256.Size
		if valid {
			digest, err := hashFileBlock(f, int64(i)*m.BlockSize, m.blockSize(i))
			if err != nil {
				return err
			}
			valid = bytes.Equal(digest[:], m.hashes[i*sha256.Size:(i+1)*sha256.Size])
		}
		if !valid {
			clearBit(m.bits, i)
			m.dirty = true
		}
	}
	return nil
}

func sidecarPath(path string) string { return path + ".hydra" }
func partPath(path string) string    { return path + ".part" }

func ensureParent(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func removeIfExists(path string) error {
	st, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("refusing to remove directory: %s", path)
	}
	err = os.Remove(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
