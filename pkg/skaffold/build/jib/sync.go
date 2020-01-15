/*
Copyright 2020 The Skaffold Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package jib

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"time"

	"github.com/pkg/errors"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/util"
)

type SyncMap map[string]SyncEntry

type SyncEntry struct {
	Dest     []string
	FileTime time.Time
	IsDirect bool
}

type JSONSyncMap struct {
	Direct    []JSONSyncEntry `json:"direct"`
	Generated []JSONSyncEntry `json:"generated"`
}

type JSONSyncEntry struct {
	Src  string `json:"src"`
	Dest string `json:"dest"`
}

func getSyncMapFromSystem(cmd *exec.Cmd) (*SyncMap, error) {
	jsm := JSONSyncMap{}
	stdout, err := util.RunCmdOut(cmd)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Jib sync map")
	}

	// To parse the output, search for "BEGIN JIB JSON", then unmarshal the next line into the pathMap struct.
	// Syncmap is transitioning to "BEGIN JIB JSON: SYNCMAP/1" starting in jib 2.0.0
	// perhaps this feature should only be included from 2.0.0 onwards? And we generally avoid this?
	matches := regexp.MustCompile(`BEGIN JIB JSON(?:: SYNCMAP/1)?\r?\n({.*})`).FindSubmatch(stdout)
	if len(matches) == 0 {
		return nil, errors.New("failed to get Jib Sync data")
	}

	line := bytes.Replace(matches[1], []byte(`\`), []byte(`\\`), -1)
	if err := json.Unmarshal(line, &jsm); err != nil {
		return nil, errors.WithStack(err)
	}

	sm := make(SyncMap)
	if err := sm.addEntries(jsm.Direct, true); err != nil {
		return nil, errors.WithStack(err)
	}
	if err := sm.addEntries(jsm.Generated, false); err != nil {
		return nil, errors.WithStack(err)
	}
	return &sm, nil
}

func (sm SyncMap) addEntries(entries []JSONSyncEntry, direct bool) error {
	for _, entry := range entries {
		info, err := os.Stat(entry.Src)
		if err != nil {
			return errors.Wrapf(err, "could not obtain file mod time for %s", entry.Src)
		}
		sm[entry.Src] = SyncEntry{
			Dest:     []string{entry.Dest},
			FileTime: info.ModTime(),
			IsDirect: direct,
		}
	}
	return nil
}
