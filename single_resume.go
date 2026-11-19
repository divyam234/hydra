package hydra

import "os"

// Single-stream HTTP resume uses a saved byte offset and a strong remote ETag.
// The source ETag must still match on resume, but existing local bytes are not
// hashed; callers needing end-to-end integrity can supply a publisher checksum.
func saveSingleResume(tmp, final string, urls []string, info probeInfo) error {
	st, err := os.Stat(tmp)
	if err != nil {
		return err
	}
	m := newMeta(urls, final, info, max(DefaultResumeBlockSize, info.Size))
	m.Single = true
	m.PrefixSize = st.Size()
	return m.save(sidecarPath(final))
}
