// Copyright 2017 Brian Lachniet. All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

/*
Gomir mirrors git repositories
*/
package main

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// TODO: Add support for controlling concurrency during fetch/push
// TODO: Add documentation describing where repos are stored
// TODO: Add documentation describing what to do if something goes wrong during a fetch/push

// Set via ldflags
var version string
var gitCommit string
var buildDate string

func main() {
	rootCmd := &cobra.Command{
		Use:  "gomir",
		Long: `Mirror Git repositories between two disconnected networks`,
	}

	addCmd := &cobra.Command{
		Use:   "add <fetchURL> <pushURL> [<localDest>]",
		Short: "Add a repository to mirror",
		Args:  cobra.RangeArgs(2, 3),
		Run: func(cmd *cobra.Command, args []string) {
			switch len(args) {
			case 2:
				add(args[0], args[1], "")
			case 3:
				add(args[0], args[1], args[2])
			default:
				fmt.Println("Wrong number of arguments")
				os.Exit(1)
			}
		},
	}

	fetchCmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch changes for all mirroed repositories",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fetch()
		},
	}

	pushCmd := &cobra.Command{
		Use:   "push",
		Short: "Push changes for all mirrored repositories",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			push()
		},
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show gomir version information",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("  Version:    %v\n", version)
			fmt.Printf("  Git commit: %v\n", gitCommit)
			fmt.Printf("  Built:      %v\n", buildDate)
		},
	}

	rootCmd.AddCommand(addCmd, fetchCmd, pushCmd, versionCmd)
	rootCmd.Execute()
}

func add(fetchURL, pushURL, localDest string) {
	// Try to generate a localDest
	if localDest == "" {
		u, err := url.Parse(fetchURL)
		if err != nil {
			fmt.Println("Could not generate a localDest")
			os.Exit(1)
		}
		localDest = fmt.Sprintf("%v%v", u.Host, u.Path)
	}

	localDest = ensureGitExt(localDest)

	// Clone
	if err := gitCloneMirror(fetchURL, localDest); err != nil {
		fmt.Println("Error cloning repository")
		os.Exit(1)
	}

	// Set Push URL
	if err := gitSetOriginPushURL(localDest, pushURL); err != nil {
		fmt.Println("Error setting push URL")
		os.Exit(1)
	}
}

func fetch() {
	errCount := performOperationAsync(fetchSingle)
	if errCount > 0 {
		color.Red("Fetch failed for %v repos", errCount)
		os.Exit(1)
	}
}

func fetchSingle(gitDir string) bool {
	logFile, logger, err := getLog(gitDir, "FETCH: ")
	if err != nil {
		color.Red("Error opening log file for %v", gitDir)
		return false
	}
	defer logFile.Close()

	logger.Println("Start")
	success := gitFetchPrune(gitDir, logFile) == nil
	logger.Printf("Done, success:%v", success)
	return success
}

func push() {
	errCount := performOperationAsync(pushSingle)
	if errCount > 0 {
		color.Red("Push failed for %v repos", errCount)
		os.Exit(1)
	}
}

func pushSingle(gitDir string) bool {
	logFile, logger, err := getLog(gitDir, "PUSH: ")
	if err != nil {
		color.Red("Error opening log file for %v", gitDir)
		return false
	}
	defer logFile.Close()

	logger.Println("Start")

	// Where are we pushing to?
	pushURL, err := gitGetOriginPushURL(gitDir)
	if err != nil {
		logger.Printf("Error retrieving push URL: %+v", err)
		return false
	}

	// If pushing using file protocol and destination repository does
	// not alread exist, initialize it
	isFileProtocol := pushURL.Scheme == "" || strings.ToLower(pushURL.Scheme) == "file"
	if isFileProtocol {
		_, err := os.Stat(pushURL.Path)
		if err != nil && os.IsNotExist(err) {
			if err := gitInitBareRepo(pushURL.Path, logFile); err != nil {
				logger.Printf("Error initializing bare git repository: %+v", err)
				return false
			}
		}
	}

	// Push
	if err := gitPushMirror(gitDir, logFile); err != nil {
		logger.Printf("Error pushing: %+v", err)
		return false
	}

	// Update server info
	if isFileProtocol {
		if err := gitUpdateServerInfo(pushURL.Path, logFile); err != nil {
			logger.Printf("Error updating server info: %+v", err)
			return false
		}
	}

	logger.Println("Done")
	return true
}

func findGitDirs() []string {
	gitDirs := []string{}
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err == nil &&
			info.IsDir() &&
			strings.ToLower(filepath.Base(path)) != ".git" &&
			strings.ToLower(filepath.Ext(path)) == ".git" {
			gitDirs = append(gitDirs, path)
			return filepath.SkipDir
		}
		return nil
	})
	return gitDirs
}

func ensureGitExt(str string) string {
	if strings.ToLower(filepath.Ext(str)) != ".git" {
		return fmt.Sprintf("%v.git", str)
	}
	return str
}

func getLog(gitDir, prefix string) (io.WriteCloser, *log.Logger, error) {
	logPath := fmt.Sprintf("%v.log", gitDir)
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Error opening log file")
	}

	logger := log.New(logFile, prefix, log.Ldate|log.Ltime|log.Lshortfile|log.LUTC)
	return logFile, logger, nil
}

type gitDirOperation func(gitDir string) bool

func performOperationAsync(op gitDirOperation) int64 {
	var wg sync.WaitGroup
	var errCount int64
	for _, gitDir := range findGitDirs() {
		wg.Add(1)
		go func(gitDir string) {
			defer wg.Done()

			if op(gitDir) {
				color.Green("[✔] %v", gitDir)
			} else {
				atomic.AddInt64(&errCount, 1)
				color.Red("[X] %v", gitDir)
			}
		}(gitDir)
	}

	wg.Wait()
	return errCount
}
