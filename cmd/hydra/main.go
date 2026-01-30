package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/divyam234/hydra/pkg/downloader"
	"github.com/spf13/cobra"
)

var (
	// These are set by ldflags during build
	version   = "dev"
	buildTime = "unknown"

	rootCmd = &cobra.Command{
		Use:   "hydra",
		Short: "Hydra - Multi-Connection Download Manager",
		Long:  `Hydra is a high-performance, multi-connection download manager written in Go.`,
	}

	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version number of Hydra",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Hydra %s (Built: %s)\n", version, buildTime)
		},
	}

	downloadCmd = &cobra.Command{
		Use:   "download [urls...]",
		Short: "Download files from URLs",
		Run: func(cmd *cobra.Command, args []string) {
			var opts []downloader.Option

			// Load flags into options
			if dir, _ := cmd.Flags().GetString("dir"); dir != "" {
				opts = append(opts, downloader.WithDir(dir))
			}
			if out, _ := cmd.Flags().GetString("out"); out != "" {
				opts = append(opts, downloader.WithFilename(out))
			}
			if ua, _ := cmd.Flags().GetString("user-agent"); ua != "" {
				opts = append(opts, downloader.WithUserAgent(ua))
			}
			if split, _ := cmd.Flags().GetInt("split"); split > 0 {
				opts = append(opts, downloader.WithSplit(split))
			}
			if limit, _ := cmd.Flags().GetString("max-download-limit"); limit != "" {
				opts = append(opts, downloader.WithMaxSpeed(limit))
			}
			if checksum, _ := cmd.Flags().GetString("checksum"); checksum != "" {
				opts = append(opts, downloader.WithChecksum(checksum))
			}
			if tries, _ := cmd.Flags().GetInt("max-tries"); tries > 0 {
				opts = append(opts, downloader.WithRetries(tries))
			}
			if wait, _ := cmd.Flags().GetInt("retry-wait"); wait > 0 {
				opts = append(opts, downloader.WithRetryWait(wait))
			}
			if lowest, _ := cmd.Flags().GetString("lowest-speed-limit"); lowest != "" {
				opts = append(opts, downloader.WithLowestSpeed(lowest))
			}
			if cookies, _ := cmd.Flags().GetString("load-cookies"); cookies != "" {
				opts = append(opts, downloader.WithCookieFile(cookies))
			}
			if headers, _ := cmd.Flags().GetStringSlice("header"); len(headers) > 0 {
				for _, h := range headers {
					parts := strings.SplitN(h, ":", 2)
					if len(parts) == 2 {
						opts = append(opts, downloader.WithHeader(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])))
					}
				}
			}
			if ref, _ := cmd.Flags().GetString("referer"); ref != "" {
				opts = append(opts, downloader.WithReferer(ref))
			}
			if user, _ := cmd.Flags().GetString("http-user"); user != "" {
				pass, _ := cmd.Flags().GetString("http-passwd")
				opts = append(opts, downloader.WithAuth(user, pass))
			}
			if proxy, _ := cmd.Flags().GetString("proxy"); proxy != "" {
				opts = append(opts, downloader.WithProxy(proxy))
			}
			if noProxy, _ := cmd.Flags().GetString("no-proxy"); noProxy != "" {
				opts = append(opts, downloader.WithNoProxy(noProxy))
			}
			if timeout, _ := cmd.Flags().GetInt("timeout"); timeout > 0 {
				opts = append(opts, downloader.WithTimeout(timeout))
			}
			if connectTimeout, _ := cmd.Flags().GetInt("connect-timeout"); connectTimeout > 0 {
				opts = append(opts, downloader.WithConnectTimeout(connectTimeout))
			}
			if maxPieces, _ := cmd.Flags().GetInt("max-pieces-per-segment"); maxPieces > 0 {
				opts = append(opts, downloader.WithMaxPiecesPerSegment(maxPieces))
			}
			if sel, _ := cmd.Flags().GetString("piece-selector"); sel != "" {
				opts = append(opts, downloader.WithPieceSelector(sel))
			}
			if alloc, _ := cmd.Flags().GetString("file-allocation"); alloc != "" {
				opts = append(opts, downloader.WithFileAllocation(alloc))
			}
			if maxConcurrent, _ := cmd.Flags().GetInt("max-concurrent-downloads"); maxConcurrent > 0 {
				opts = append(opts, downloader.WithMaxConcurrentDownloads(maxConcurrent))
			}
			if quiet, _ := cmd.Flags().GetBool("quiet"); quiet {
				opts = append(opts, downloader.WithQuiet(true))
			}
			if allowOverwrite, _ := cmd.Flags().GetBool("allow-overwrite"); allowOverwrite {
				opts = append(opts, downloader.WithAllowOverwrite(true))
			}
			// Default is true, so only set if false
			if autoRenaming, _ := cmd.Flags().GetBool("auto-file-renaming"); !autoRenaming {
				opts = append(opts, downloader.WithAutoFileRenaming(false))
			}
			if logFile, _ := cmd.Flags().GetString("log"); logFile != "" {
				opts = append(opts, downloader.WithLogFile(logFile))
			}

			// SSL Verification

			checkCert, _ := cmd.Flags().GetBool("check-certificate")
			insecure, _ := cmd.Flags().GetBool("insecure")
			if insecure {
				checkCert = false
			}
			opts = append(opts, downloader.WithCheckCertificate(checkCert))

			eng := downloader.NewEngine(opts...)
			defer eng.Shutdown()

			// Setup signal handling
			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				<-sigs
				fmt.Println("\nShutdown signal received. Saving state...")
				eng.Shutdown()
			}()

			// Add downloads
			addedCount := 0

			// 1. Process Input File
			if inputFile, _ := cmd.Flags().GetString("input-file"); inputFile != "" {
				f, err := os.Open(inputFile)
				if err != nil {
					fmt.Printf("Failed to open input file: %v\n", err)
					os.Exit(1)
				}
				defer f.Close()

				scanner := bufio.NewScanner(f)
				for scanner.Scan() {
					line := strings.TrimSpace(scanner.Text())
					if line != "" && !strings.HasPrefix(line, "#") {
						// Each line is a separate download
						_, err := eng.AddDownload(context.Background(), []string{line})
						if err != nil {
							fmt.Printf("Failed to add download from file (%s): %v\n", line, err)
						} else {
							addedCount++
						}
					}
				}
				if err := scanner.Err(); err != nil {
					fmt.Printf("Error reading input file: %v\n", err)
				}
			}

			// 2. Process CLI Args
			if len(args) > 0 {
				forceSequential, _ := cmd.Flags().GetBool("force-sequential")
				if forceSequential {
					// Treat each arg as a separate download
					for _, arg := range args {
						_, err := eng.AddDownload(context.Background(), []string{arg})
						if err != nil {
							fmt.Printf("Failed to add download (%s): %v\n", arg, err)
						} else {
							addedCount++
						}
					}
				} else {
					// Treat all args as mirrors for ONE download
					_, err := eng.AddDownload(context.Background(), args)
					if err != nil {
						fmt.Printf("Failed to add download: %v\n", err)
					} else {
						addedCount++
					}
				}
			}

			if addedCount == 0 {
				fmt.Println("No downloads specified.")
				if len(args) == 0 {
					cmd.Help()
				}
				os.Exit(1)
			}

			if err := eng.Wait(); err != nil {
				// Don't print error if it's just "download failed" generic message,
				// as individual errors are logged.
				// fmt.Printf("Engine error: %v\n", err)
				os.Exit(1)
			}
		},
	}
)

func init() {
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(versionCmd)

	downloadCmd.Flags().StringP("dir", "d", "", "Directory to store the downloaded file")
	downloadCmd.Flags().StringP("out", "o", "", "The filename of the downloaded file")
	downloadCmd.Flags().StringP("user-agent", "U", "", "Set User-Agent header")
	downloadCmd.Flags().IntP("split", "s", 5, "Number of connections to download file")
	downloadCmd.Flags().String("max-download-limit", "0", "Max download speed per download (e.g. 1M)")
	downloadCmd.Flags().String("checksum", "", "Verify checksum after download (e.g. sha-1=digest)")
	downloadCmd.Flags().Int("max-tries", 5, "Number of retries")
	downloadCmd.Flags().Int("retry-wait", 0, "Wait time between retries in seconds")
	downloadCmd.Flags().String("lowest-speed-limit", "0", "Close connection if speed is lower than this (e.g. 10K)")
	downloadCmd.Flags().String("load-cookies", "", "Load cookies from file (Netscape/Mozilla format)")
	downloadCmd.Flags().StringSlice("header", nil, "Append header to HTTP request")
	downloadCmd.Flags().String("referer", "", "Set Referer header")
	downloadCmd.Flags().String("http-user", "", "Set HTTP Basic Auth user")
	downloadCmd.Flags().String("http-passwd", "", "Set HTTP Basic Auth password")
	downloadCmd.Flags().String("proxy", "", "Set proxy (http/https/socks5) e.g. http://user:pass@host:port")
	downloadCmd.Flags().String("no-proxy", "", "Comma separated list of domains to ignore proxy")
	downloadCmd.Flags().Int("timeout", 0, "Timeout in seconds")
	downloadCmd.Flags().Int("connect-timeout", 0, "Connect timeout in seconds")
	downloadCmd.Flags().Int("max-pieces-per-segment", 20, "Max pieces per segment (chunk size control)")
	downloadCmd.Flags().String("piece-selector", "inorder", "Piece selection strategy: inorder, random")
	downloadCmd.Flags().String("file-allocation", "trunc", "File allocation method: none, trunc, falloc")

	// New Flags
	downloadCmd.Flags().BoolP("check-certificate", "V", true, "Verify SSL/TLS certificates")
	downloadCmd.Flags().BoolP("insecure", "k", false, "Skip SSL/TLS verification (same as --check-certificate=false)")
	downloadCmd.Flags().StringP("input-file", "i", "", "Downloads URIs found in FILE")
	downloadCmd.Flags().IntP("max-concurrent-downloads", "j", 5, "Set maximum number of parallel downloads")
	downloadCmd.Flags().BoolP("force-sequential", "Z", false, "Fetch URIs in the command-line sequentially (treat as separate downloads). Use with -j to control concurrency.")
	downloadCmd.Flags().BoolP("quiet", "q", false, "Make the operation quiet")
	downloadCmd.Flags().Bool("allow-overwrite", false, "Restart download from scratch if the corresponding control file doesn't exist")
	downloadCmd.Flags().Bool("auto-file-renaming", true, "Rename file if the same file already exists")
	downloadCmd.Flags().StringP("log", "l", "", "The file name of the log file. If - is specified, log to stdout.")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
