package identity

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// MergePolicy decides whether entries sharing only a full name merge
// across accounts. Rare multi-word names merge on their own; common names
// park as pending until the user approves or denies them in the sidecar
// file. A nil policy applies the same rarity rule without recording
// anything.
type MergePolicy struct {
	mu       sync.Mutex
	approved map[string]bool
	denied   map[string]bool
	pending  map[string][]string // normalized name -> networks seen
}

// allowNameMerge reports whether two same-named entries may merge, and
// records the pair as pending when the answer is "ask the user".
func (mp *MergePolicy) allowNameMerge(name string, networks []string) bool {
	n := Normalize(name)
	if len(strings.Fields(n)) < 2 {
		return false
	}
	if mp == nil {
		return !commonFullName(n)
	}
	mp.mu.Lock()
	defer mp.mu.Unlock()
	switch {
	case mp.denied[n]:
		return false
	case mp.approved[n]:
		return true
	case !commonFullName(n):
		return true
	}
	if mp.pending == nil {
		mp.pending = map[string][]string{}
	}
	mp.pending[n] = appendUnique(mp.pending[n], networks...)
	return false
}

// commonFullName reports whether both the first and last word of a
// normalized name are frequent, which makes distinct same-named humans
// plausible.
func commonFullName(normalized string) bool {
	words := strings.Fields(normalized)
	if len(words) < 2 {
		return false
	}
	return commonFirstNames[words[0]] && commonSurnames[words[len(words)-1]]
}

type mergeFile struct {
	Approved []string      `yaml:"approved"`
	Denied   []string      `yaml:"denied"`
	Pending  []pendingPair `yaml:"pending"`
}

type pendingPair struct {
	Name     string   `yaml:"name"`
	Networks []string `yaml:"networks,flow"`
}

// LoadMergePolicy reads the sidecar file. A missing file is an empty
// policy, not an error.
func LoadMergePolicy(path string) (*MergePolicy, error) {
	mp := &MergePolicy{approved: map[string]bool{}, denied: map[string]bool{}}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return mp, nil
	}
	if err != nil {
		return nil, err
	}
	var f mergeFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("identity: %s: %w", path, err)
	}
	for _, n := range f.Approved {
		mp.approved[Normalize(n)] = true
	}
	for _, n := range f.Denied {
		mp.denied[Normalize(n)] = true
	}
	return mp, nil
}

const mergeFileHeader = `# Cross-network people who share a common name. Move a pending name to
# approved to treat the entries as one person, or to denied to keep them
# separate. Pending names stay separate until you decide.
`

// SaveMergePolicy writes the sidecar back with the current pending pairs.
// It leaves a missing file missing when there is nothing to decide.
func SaveMergePolicy(path string, mp *MergePolicy) error {
	if mp == nil {
		return nil
	}
	mp.mu.Lock()
	f := mergeFile{}
	for n := range mp.approved {
		f.Approved = append(f.Approved, n)
	}
	for n := range mp.denied {
		f.Denied = append(f.Denied, n)
	}
	for n, networks := range mp.pending {
		if mp.approved[n] || mp.denied[n] {
			continue
		}
		f.Pending = append(f.Pending, pendingPair{Name: n, Networks: networks})
	}
	mp.mu.Unlock()
	if len(f.Approved) == 0 && len(f.Denied) == 0 && len(f.Pending) == 0 {
		return nil
	}
	sort.Strings(f.Approved)
	sort.Strings(f.Denied)
	sort.Slice(f.Pending, func(i, j int) bool { return f.Pending[i].Name < f.Pending[j].Name })
	data, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append([]byte(mergeFileHeader), data...), 0o600)
}
