package scanner

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ai-asset-discovery/internal/config"
	"github.com/ai-asset-discovery/internal/model"
)

// FileScanner scans filesystem for agent evidence.
type FileScanner struct{}

// NewFileScanner creates a new FileScanner.
func NewFileScanner() *FileScanner {
	return &FileScanner{}
}

// Scan scans filesystem paths defined in rules and returns matched agents.
func (fs *FileScanner) Scan(rules []model.AgentRule) ([]model.DiscoveredAgent, error) {
	var results []model.DiscoveredAgent

	for _, rule := range rules {
		if len(rule.Files) == 0 {
			continue
		}
		matched := fs.matchFiles(rule)
		if matched != nil {
			results = append(results, *matched)
		}
	}
	return results, nil
}

func (fs *FileScanner) matchFiles(rule model.AgentRule) *model.DiscoveredAgent {
	var evidences []model.FileEvidence
	hasRequired := false

	for _, fr := range rule.Files {
		// OS filter
		if fr.OS != "" && fr.OS != "all" && fr.OS != config.OSName() {
			continue
		}

		expandedPath, err := config.ExpandPath(fr.Path)
		if err != nil {
			continue
		}

		info, err := os.Stat(expandedPath)
		if err != nil {
			// If this file is required but missing, skip the rule entirely
			if fr.Required {
				return nil
			}
			continue
		}

		isDir := info.IsDir()
		if fr.FileType == "directory" && !isDir {
			continue
		}
		if fr.FileType == "file" && isDir {
			continue
		}

		// For directories with max_depth, walk the tree and collect evidence
		if isDir && fr.MaxDepth > 0 {
			subFiles, err := fs.walkDir(expandedPath, fr.MaxDepth)
			if err != nil {
				if fr.Required {
					return nil
				}
				continue
			}
			contentMatched := false
			for _, sf := range subFiles {
				subEvidence := model.FileEvidence{
					Path:       sf,
					RuleSource: rule.Name,
					MatchType:  "file",
				}
				// Check content on walked files if contains is specified
				if fr.Contains != "" {
					if c, matched := fs.checkFileContent(sf, fr.Contains); matched {
						subEvidence.Content = c
						contentMatched = true
					} else {
						continue
					}
				}
				evidences = append(evidences, subEvidence)
			}
			if fr.Contains != "" && !contentMatched {
				if fr.Required {
					return nil
				}
				continue
			}
			if fr.Required {
				hasRequired = true
			}
			continue
		}

		evidence := model.FileEvidence{
			Path:       expandedPath,
			RuleSource: rule.Name,
			MatchType:  fr.FileType,
		}

		// Check content if specified (non-walked files only, directories handled above)
		if fr.Contains != "" && !isDir {
			if content, matched := fs.checkFileContent(expandedPath, fr.Contains); matched {
				evidence.Content = content
			} else {
				if fr.Required {
					return nil
				}
				continue
			}
		}

		evidences = append(evidences, evidence)
		if fr.Required {
			hasRequired = true
		}
	}

	if len(evidences) == 0 {
		return nil
	}

	// Must have at least one required match if any required rules exist
	hasRequiredRule := false
	for _, fr := range rule.Files {
		if fr.Required {
			hasRequiredRule = true
			break
		}
	}
	if hasRequiredRule && !hasRequired {
		return nil
	}

	return &model.DiscoveredAgent{
		Name:        rule.Name,
		DisplayName: rule.DisplayName,
		Confidence:  model.Confidence(rule.MinConfidence),
		AssetType:   model.AssetTypeFile,
		Files:       evidences,
	}
}

func (fs *FileScanner) checkFileContent(path, contains string) (string, bool) {
	// Only read first 10KB for performance
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	if len(data) > 10240 {
		data = data[:10240]
	}
	content := string(data)
	if strings.Contains(content, contains) {
		return content, true
	}
	return "", false
}

func (fs *FileScanner) walkDir(root string, maxDepth int) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		rel, _ := filepath.Rel(root, path)
		depth := len(strings.Split(rel, string(filepath.Separator)))
		if depth > maxDepth && d.IsDir() {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
