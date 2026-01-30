// Package downloader provides a high-level API for downloading files using aria2go.
//
// Basic usage:
//
//	res, err := downloader.Download(ctx, "http://example.com/file.zip")
//
// With options:
//
//	res, err := downloader.Download(ctx, "http://example.com/file.zip",
//	    downloader.WithDir("/tmp"),
//	    downloader.WithSplit(8),
//	    downloader.WithMaxSpeed("5M"),
//	)
//
// Advanced usage with the Engine:
//
//	eng := downloader.NewEngine(downloader.WithMaxSpeed("10M"))
//	defer eng.Shutdown()
//
//	id1, _ := eng.AddDownload(ctx, []string{"url1"})
//	id2, _ := eng.AddDownload(ctx, []string{"url2"})
//
//	eng.Wait()
package downloader
