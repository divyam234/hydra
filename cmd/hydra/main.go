package main

import (
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
	rootCmd = &cobra.Command{
		Use:   "hydra",
		Short: "Hydra - Multi-Connection Download Manager",
		Long:  `Hydra is a high-performance, multi-connection download manager written in Go.`,
	}

	downloadCmd = &cobra.Command{
		Use:   "download [urls...]",
		Short: "Download files from URLs",
		Args:  cobra.MinimumNArgs(1),
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
			if httpProxy, _ := cmd.Flags().GetString("http-proxy"); httpProxy != "" {
				// Note: internal engine uses separate options, but our library wrapper only exposed WithProxy (AllProxy)
				// To fully support http vs https proxy in library, we might need to expand options.
				// For now, let's assume WithProxy sets AllProxy which falls back.
				// Wait, internal http client logic checks AllProxy first.
				opts = append(opts, downloader.WithProxy(httpProxy))
			}
			// Handling https/all proxy similarly if set
			if allProxy, _ := cmd.Flags().GetString("all-proxy"); allProxy != "" {
				opts = append(opts, downloader.WithProxy(allProxy))
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

			// Add download
			_, err := eng.AddDownload(context.Background(), args)
			if err != nil {
				fmt.Printf("Failed to add download: %v\n", err)
				os.Exit(1)
			}

			if err := eng.Wait(); err != nil {
				fmt.Printf("Engine error: %v\n", err)
				os.Exit(1)
			}
		},
	}
)

func init() {
	rootCmd.AddCommand(downloadCmd)

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
	downloadCmd.Flags().String("http-proxy", "", "Set HTTP proxy")
	downloadCmd.Flags().String("https-proxy", "", "Set HTTPS proxy")
	downloadCmd.Flags().String("all-proxy", "", "Set proxy for all protocols")
	downloadCmd.Flags().Int("timeout", 0, "Timeout in seconds")
	downloadCmd.Flags().Int("connect-timeout", 0, "Connect timeout in seconds")
	downloadCmd.Flags().Int("max-pieces-per-segment", 20, "Max pieces per segment (chunk size control)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
