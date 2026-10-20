# Go API

Use Hydra to download files from a Go application.

## Install

```bash
go get github.com/divyam234/hydra
```

## Download a file

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/divyam234/hydra"
)

func main() {
	result, err := hydra.Download(context.Background(), hydra.Request{
		URLs: []string{"https://example.com/file.zip"},
		Dir:  "downloads",
	}, hydra.DefaultOptions())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result.Path)
}
```

Pass a cancellable context to stop a download and save progress for resuming.

## Configuration

Start with `hydra.DefaultOptions()` and set the fields you need:

| Field | Purpose |
| --- | --- |
| `Split` | Requested parallel connections per file. |
| `MaxConnectionsPerServer` | Connection limit per server. |
| `Proxy` | HTTP, HTTPS, SOCKS5, or SOCKS5H proxy URL. |
| `ExistingFile` | Resume, overwrite, skip, or fail when the target exists. |
| `OnProgress` | Receive progress snapshots. |
| `OnEvent` | Receive download and retry events. |

Keep progress and event callbacks fast and safe to call from worker goroutines.

## Queue manager

Use `NewManager` to download multiple files. `MaxConcurrentDownloads` limits how many files run at once; `Split` controls parallel connections within each file.

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/divyam234/hydra"
)

func main() {
	ctx := context.Background()
	opts := hydra.DefaultOptions()
	opts.MaxConcurrentDownloads = 2

	mgr, err := hydra.NewManager(ctx, opts)
	if err != nil {
		log.Print(err)
		return
	}
	defer mgr.Close()

	var ids []hydra.JobID
	for _, url := range []string{
		"https://example.com/a.zip",
		"https://example.com/b.zip",
	} {
		id, err := mgr.Enqueue(hydra.Request{
			URLs: []string{url},
			Dir:  "downloads",
		})
		if err != nil {
			log.Print(err)
			return
		}
		ids = append(ids, id)
	}

	for _, job := range mgr.WaitAll(ctx, ids...) {
		if job.Err != nil {
			log.Printf("%s: %v", job.ID, job.Err)
			continue
		}
		fmt.Println(job.ID, job.Result.Path)
	}
}
```

### Methods

| Method | Behavior |
| --- | --- |
| `NewManager(ctx, opts)` | Creates workers using the supplied options and context. |
| `mgr.Enqueue(req)` | Submits a download and returns its `JobID`, or an error if submission fails. Blocks until a worker accepts the job. |
| `mgr.Wait(ctx, id)` | Waits for one job and returns its `Result` and download error. |
| `mgr.WaitAll(ctx, ids...)` | Waits for the supplied jobs and returns a `[]JobResult` in the same order as the IDs, not completion order. Check each entry's `Err`. |
| `mgr.Close()` | Cancels active downloads, waits for workers to stop, and releases idle connections in its own transport. Does not drain downloads to completion. |

`Enqueue` generates an ID when `Request.ID` is empty. Custom IDs must be unique within the manager, including completed jobs. Successful submission does not mean the download succeeded; use `Wait` or `WaitAll` to get its outcome. Completed results can be read more than once.

### Cancellation and progress

- Cancel the context passed to `NewManager` to stop all its downloads. There is no per-job cancellation method.
- Cancelling the context passed to `Wait` or `WaitAll` only stops waiting; it does not cancel downloads.
- Call `Close` when finished. To let downloads complete normally, wait for their results before closing.
- Set `Options.OnProgress` or `Options.OnEvent` before creating the manager. Callbacks may run concurrently; use their `ID` field to associate updates with jobs.
- A caller-supplied `Options.Transport` remains caller-owned; closing the manager does not close its idle connections.

[Back to README](../README.md)
