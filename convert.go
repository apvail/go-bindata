// This work is subject to the CC0 1.0 Universal (CC0 1.0) Public Domain Dedication
// license. Its contents can be found at:
// http://creativecommons.org/publicdomain/zero/1.0/

package bindata

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Translate reads assets from an input directory, converts them
// to Go code and writes new files to the output specified
// in the given configuration.
func Translate(c *Config) error {
	var toc []Asset

	// Ensure our configuration has sane values.
	err := c.validate()
	if err != nil {
		return err
	}

	var knownFuncs = make(map[string]int)
	var visitedPaths = make(map[string]bool)
	// Locate all the assets.
	for _, input := range c.Input {
		err = findFiles(input.Path, c.Prefix, input.Recursive, &toc, c.Ignore, knownFuncs, visitedPaths)
		if err != nil {
			return err
		}
	}

	// Create output file.
	fd, err := os.Create(c.Output)
	if err != nil {
		return err
	}

	defer fd.Close()

	// Create a buffered writer for better performance.
	bfd := bufio.NewWriter(fd)
	defer bfd.Flush()

	// Write the header. This makes e.g. Github ignore diffs in generated files.
	if _, err = fmt.Fprint(bfd, "// Code generated by go-bindata.\n"); err != nil {
		return err
	}
	if _, err = fmt.Fprint(bfd, "// sources:\n"); err != nil {
		return err
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	for _, asset := range toc {
		relative, _ := filepath.Rel(wd, asset.Path)
		if _, err = fmt.Fprintf(bfd, "// %s\n", filepath.ToSlash(relative)); err != nil {
			return err
		}
	}
	if _, err = fmt.Fprint(bfd, "// DO NOT EDIT!\n\n"); err != nil {
		return err
	}

	// Write build tags, if applicable.
	if len(c.Tags) > 0 {
		if _, err = fmt.Fprintf(bfd, "// +build %s\n\n", c.Tags); err != nil {
			return err
		}
	}

	// Write package declaration.
	_, err = fmt.Fprintf(bfd, "package %s\n\n", c.Package)
	if err != nil {
		return err
	}

	// Write assets.
	if c.Debug || c.Dev {
		err = writeDebug(bfd, c, toc)
	} else {
		err = writeRelease(bfd, c, toc)
	}

	if err != nil {
		return err
	}

	// Write table of contents
	if err := writeTOC(bfd, toc); err != nil {
		return err
	}
	// Write hierarchical tree of assets
	if err := writeTOCTree(bfd, toc); err != nil {
		return err
	}

	// Write restore procedure
	return writeRestore(bfd)
}

// Implement sort.Interface for []os.FileInfo based on Name()
type ByName []os.FileInfo

func (v ByName) Len() int           { return len(v) }
func (v ByName) Swap(i, j int)      { v[i], v[j] = v[j], v[i] }
func (v ByName) Less(i, j int) bool { return v[i].Name() < v[j].Name() }

// findFiles recursively finds all the file paths in the given directory tree.
// They are added to the given map as keys. Values will be safe function names
// for each file, which will be used when generating the output code.
func findFiles(dir string, prefix *regexp.Regexp, recursive bool, toc *[]Asset, ignore []*regexp.Regexp, knownFuncs map[string]int, visitedPaths map[string]bool) error {
	dirpath := dir

	fi, err := os.Stat(dirpath)
	if err != nil {
		return err
	}

	var list []os.FileInfo

	if !fi.IsDir() {
		dirpath = filepath.Dir(dirpath)
		list = []os.FileInfo{fi}
	} else {
		visitedPaths[dirpath] = true
		fd, err := os.Open(dirpath)
		if err != nil {
			return err
		}

		defer fd.Close()

		list, err = fd.Readdir(0)
		if err != nil {
			return err
		}

		// Sort to make output stable between invocations
		sort.Sort(ByName(list))
	}

	for _, file := range list {
		var asset Asset
		asset.Path = filepath.Join(dirpath, file.Name())
		asset.Name = filepath.ToSlash(asset.Path)

		ignoring := false
		for _, re := range ignore {
			if re.MatchString(asset.Path) {
				ignoring = true
				break
			}
		}
		if ignoring {
			continue
		}

		if file.IsDir() {
			if recursive {
				recursivePath := filepath.Join(dir, file.Name())
				visitedPaths[asset.Path] = true
				findFiles(recursivePath, prefix, recursive, toc, ignore, knownFuncs, visitedPaths)
			}
			continue
		} else if file.Mode()&os.ModeSymlink == os.ModeSymlink {
			var linkPath string
			if linkPath, err = os.Readlink(asset.Path); err != nil {
				return err
			}
			if !filepath.IsAbs(linkPath) {
				if linkPath, err = filepath.Abs(dirpath + "/" + linkPath); err != nil {
					return err
				}
			}
			if _, ok := visitedPaths[linkPath]; !ok {
				visitedPaths[linkPath] = true
				findFiles(asset.Path, prefix, recursive, toc, ignore, knownFuncs, visitedPaths)
			}
			continue
		}

		if prefix != nil && prefix.MatchString(asset.Name) {
			asset.Name = prefix.ReplaceAllString(asset.Name, "")
		} else {
			asset.Name = filepath.Join(dir, file.Name())
		}

		// If we have a leading slash, get rid of it.
		if len(asset.Name) > 0 && asset.Name[0] == '/' {
			asset.Name = asset.Name[1:]
		}

		// This shouldn't happen.
		if len(asset.Name) == 0 {
			return fmt.Errorf("Invalid file: %v", asset.Path)
		}

		asset.Func = safeFunctionName(asset.Name, knownFuncs)
		asset.Path, _ = filepath.Abs(asset.Path)
		*toc = append(*toc, asset)
	}

	return nil
}

var regFuncName = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// safeFunctionName converts the given name into a name
// which qualifies as a valid function identifier. It
// also compares against a known list of functions to
// prevent conflict based on name translation.
func safeFunctionName(name string, knownFuncs map[string]int) string {
	var inBytes, outBytes []byte
	var toUpper bool

	name = strings.ToLower(name)
	inBytes = []byte(name)

	for i := 0; i < len(inBytes); i++ {
		if regFuncName.Match([]byte{inBytes[i]}) {
			toUpper = true
		} else if toUpper {
			outBytes = append(outBytes, []byte(strings.ToUpper(string(inBytes[i])))...)
			toUpper = false
		} else {
			outBytes = append(outBytes, inBytes[i])
		}
	}

	name = string(outBytes)

	// Identifier can't start with a digit.
	if unicode.IsDigit(rune(name[0])) {
		name = "_" + name
	}

	if num, ok := knownFuncs[name]; ok {
		knownFuncs[name] = num + 1
		name = fmt.Sprintf("%s%d", name, num)
	} else {
		knownFuncs[name] = 2
	}

	return name
}
