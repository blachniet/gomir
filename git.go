package main

import (
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

// git clone --mirror <fetchURL> <localDest>
func gitCloneMirror(fetchURL, localDest string) error {
	cmd := exec.Command("git", "clone", "--mirror", fetchURL, localDest)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return errors.Wrap(cmd.Run(), "Error cloning repository")
}

// cd <gitDir>
// git remote set-url --push origin <pushURL>
func gitSetOriginPushURL(gitDir, pushURL string) error {
	cmd := exec.Command("git", "remote", "set-url", "--push", "origin", pushURL)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Dir = gitDir
	return errors.Wrap(cmd.Run(), "Error setting push URL")
}

// cd <gitDir>
// git push --mirror
func gitPushMirror(gitDir string, logFile io.Writer) error {
	cmd := exec.Command("git", "push", "--mirror")
	cmd.Stderr = logFile
	cmd.Stdout = logFile
	cmd.Dir = gitDir
	return errors.Wrap(cmd.Run(), "Error pushing mirrored git repo")
}

// cd <gitDir>
// git remote get-url --push origin
func gitGetOriginPushURL(gitDir string) (*url.URL, error) {
	cmd := exec.Command("git", "remote", "get-url", "--push", "origin")
	cmd.Dir = gitDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, errors.Wrapf(err, "Error running git command `git remote get-url --push origin` for %#v", gitDir)
	}

	urlOutput, err := url.Parse(strings.TrimSpace(string(output)))
	if err != nil {
		return nil, errors.Wrap(err, "Error parsing push URL")
	}

	return urlOutput, err
}

// git init --bare <gitDir>
func gitInitBareRepo(gitDir string, logFile io.Writer) error {
	cmd := exec.Command("git", "init", "--bare", gitDir)
	cmd.Stderr = logFile
	cmd.Stdout = logFile
	return errors.Wrapf(cmd.Run(), "Error initializing bare git repo at %v", gitDir)
}

// cd <gitDir>
// git update-server-info
func gitUpdateServerInfo(gitDir string, logFile io.Writer) error {
	cmd := exec.Command("git", "update-server-info")
	cmd.Stderr = logFile
	cmd.Stdout = logFile
	cmd.Dir = gitDir
	return errors.Wrap(cmd.Run(), "Error updating server info")
}

// cd <gitDir>
// git fetch -p origin
func gitFetchPrune(gitDir string, logFile io.Writer) error {
	cmd := exec.Command("git", "fetch", "-p", "origin")
	cmd.Stderr = logFile
	cmd.Stdout = logFile
	cmd.Dir = gitDir
	return errors.Wrap(cmd.Run(), "Error fetching")
}
