// Copyright 2017 Brian Lachniet. All rights reserved.

// Use of this source code is governed by a MIT

// license that can be found in the LICENSE file.

package main

import (
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func Test_gitCloneMirror(t *testing.T) {
	type args struct {
		fetchURL  string
		localDest string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"EmptyFetchURL",
			args{"", "emptyfetch"},
			true,
		},
		{
			"EmptyLocalDest",
			args{"https://github.com/octocat/Hello-World.git", ""},
			true,
		},
		{
			"SuccessfulMirror",
			args{"https://github.com/octocat/Hello-World.git", "successfulmirror"},
			false,
		},
		{
			"SuccessfulMirrorNoDotGit",
			args{"https://github.com/octocat/Hello-World", "successfulmirrornodotgit"},
			false,
		},
	}

	baseTempDir, err := ioutil.TempDir("", "Test_gitCloneMirror")
	if err != nil {
		t.Fatalf("Error generating base temp dir: %+v", err)
	}

	for _, tt := range tests {

		// If localDest is set, prepend the base temp dir to it
		localDest := tt.args.localDest
		if localDest != "" {
			localDest = path.Join(baseTempDir, tt.args.localDest)
		}

		t.Run(tt.name, func(t *testing.T) {
			err := gitCloneMirror(tt.args.fetchURL, localDest)
			if (err != nil) != tt.wantErr {
				t.Errorf("gitCloneMirror() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil {
				ensureDirExists(t, path.Join(localDest, "hooks"))
				ensureDirExists(t, path.Join(localDest, "info"))
				ensureDirExists(t, path.Join(localDest, "objects"))
				ensureDirExists(t, path.Join(localDest, "refs"))
				ensureFileExists(t, path.Join(localDest, "config"))
				ensureFileExists(t, path.Join(localDest, "description"))
				ensureFileExists(t, path.Join(localDest, "HEAD"))
				ensureFileExists(t, path.Join(localDest, "packed-refs"))
			}
		})
	}

	// Remove any temp files
	os.RemoveAll(baseTempDir)
}

func ensureDirExists(t *testing.T, name string) {
	st, err := os.Stat(name)
	if os.IsNotExist(err) {
		t.Errorf("Directory does not exist: %v", name)
	} else if err != nil {
		t.Errorf("Error checking for directory: %v", err)
	} else if !st.IsDir() {
		t.Errorf("Expected directory, but was not: %v", name)
	}
}

func ensureFileExists(t *testing.T, name string) {
	st, err := os.Stat(name)
	if os.IsNotExist(err) {
		t.Errorf("File does not exist: %v", name)
	} else if err != nil {
		t.Errorf("Error checking for file: %v", err)
	} else if st.IsDir() {
		t.Errorf("Expected file, but was not: %v", name)
	}
}
