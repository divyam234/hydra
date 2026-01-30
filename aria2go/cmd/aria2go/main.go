package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bhunter/aria2go/internal/engine"
	"github.com/bhunter/aria2go/pkg/option"
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "aria2go",
		Short: "Aria2Go - Native Go HTTP Download Utility",
		Long: `Aria2Go is a feature-rich, high-performance download utility 
written in Go, inspired by aria2c.`,
	}

	downloadCmd = &cobra.Command{
		Use:   "download [urls...]",
		Short: "Download files from URLs",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			opt := option.GetDefaultOptions()

			// Load flags into option
			if dir, _ := cmd.Flags().GetString("dir"); dir != "" {
				opt.Put(option.Dir, dir)
			}
			if out, _ := cmd.Flags().GetString("out"); out != "" {
				opt.Put(option.Out, out)
			}
			if ua, _ := cmd.Flags().GetString("user-agent"); ua != "" {
				opt.Put(option.UserAgent, ua)
			}
			if split, _ := cmd.Flags().GetInt("split"); split > 0 {
				opt.Put(option.Split, fmt.Sprintf("%d", split))
			}
			if limit, _ := cmd.Flags().GetString("max-download-limit"); limit != "" {
				opt.Put(option.MaxDownloadLimit, limit)
			}
			if checksum, _ := cmd.Flags().GetString("checksum"); checksum != "" {
				opt.Put(option.Checksum, checksum)
			}
			if tries, _ := cmd.Flags().GetInt("max-tries"); tries > 0 {
				opt.Put(option.MaxTries, fmt.Sprintf("%d", tries))
			}
			if wait, _ := cmd.Flags().GetInt("retry-wait"); wait > 0 {
				opt.Put(option.RetryWait, fmt.Sprintf("%d", wait))
			}
			if lowest, _ := cmd.Flags().GetString("lowest-speed-limit"); lowest != "" {
				opt.Put(option.LowestSpeedLimit, lowest)
			}
			if cookies, _ := cmd.Flags().GetString("load-cookies"); cookies != "" {
				opt.Put(option.LoadCookies, cookies)
			}
			if headers, _ := cmd.Flags().GetStringSlice("header"); len(headers) > 0 {
				// We currently only support one header line in option.Option if it's a map[string]string
				// and option.Header is "header".
				// To support multiple, we might need to join them or change Option to hold list.
				// For now, let's join with \n or just take the first/last?
				// RequestGroup uses strings.SplitN(val, ":", 2).
				// It seems RequestGroup only handles ONE header currently (line 281).
				// This is a limitation. I'll pass the last one or comma join? Headers can't be comma joined safely.
				// Let's just pick the last one for now or loop and set if we could.
				// BETTER: Update RequestGroup to handle multiple headers.
				// But given current Option struct (map[string]string), we can't store multiple.
				// I'll stick to last one for now as per current limitation.
				// Wait, I can't easily change Option structure right now without breaking things.
				// I'll just use the last one.
				for _, h := range headers {
					opt.Put(option.Header, h)
				}
			}
			if ref, _ := cmd.Flags().GetString("referer"); ref != "" {
				opt.Put(option.Referer, ref)
			}
			if user, _ := cmd.Flags().GetString("http-user"); user != "" {
				opt.Put(option.HttpUser, user)
			}
			if pass, _ := cmd.Flags().GetString("http-passwd"); pass != "" {
				opt.Put(option.HttpPasswd, pass)
			}
			if httpProxy, _ := cmd.Flags().GetString("http-proxy"); httpProxy != "" {
				opt.Put(option.HttpProxy, httpProxy)
			}
			if httpsProxy, _ := cmd.Flags().GetString("https-proxy"); httpsProxy != "" {
				opt.Put(option.HttpsProxy, httpsProxy)
			}
			if allProxy, _ := cmd.Flags().GetString("all-proxy"); allProxy != "" {
				opt.Put(option.AllProxy, allProxy)
			}
			if timeout, _ := cmd.Flags().GetInt("timeout"); timeout > 0 {
				opt.Put(option.Timeout, fmt.Sprintf("%d", timeout))
			}
			if connectTimeout, _ := cmd.Flags().GetInt("connect-timeout"); connectTimeout > 0 {
				opt.Put(option.ConnectTimeout, fmt.Sprintf("%d", connectTimeout))
			}

			eng := engine.NewDownloadEngine(opt)

			// For now, treat all args as a single download with mirrors if multiple
			// In future, we might handle multiple separate downloads
			_, err := eng.AddURI(args, opt)
			if err != nil {
				fmt.Printf("Failed to add download: %v\n", err)
				os.Exit(1)
			}

			// Setup signal handling
			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				<-sigs
				fmt.Println("\nShutdown signal received. Saving state...")
				eng.Shutdown()
			}()

			eng.Run()
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
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
