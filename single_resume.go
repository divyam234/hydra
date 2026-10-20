package hydra

import (
	"fmt"
	"os"
)

func saveSingleResume(tmp, final string, urls []string, info probeInfo) error {
	f, err := os.Open(tmp)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	digest, err := hashFileBlock(f, 0, st.Size())
	if err != nil {
		return err
	}
	m := newMeta(urls, final, info, max(DefaultResumeBlockSize, info.Size))
	m.Single = true
	m.PrefixSize = st.Size()
	m.PrefixHash = fmt.Sprintf("%x", digest)
	return m.save(sidecarPath(final))
}
