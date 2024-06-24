package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	SSOTFile      = "./ssot.yaml"
	SSOTExtPrefix = ".ssot"
)

var (
	commentPrefixMap = map[string]string{
		".sql": "--",
		".go":  "//",
		".js":  "//",
	}
)

type SSOT struct {
	Files        []string                  `yaml:"files"`
	Constants    map[string]string         `yaml:"constants"`
	LineMatchMap map[string]*regexp.Regexp `yaml:"-"`
	CurrentExt   string                    `yaml:"-"`
}

func (ssot *SSOT) Initialize() (err error) {
	ssot.LineMatchMap = make(map[string]*regexp.Regexp)
	for ext := range commentPrefixMap {
		re, ok := ssot.LineMatchMap[ext]
		if ok {
			continue
		}
		re, err = ssot.lineMatchRegex(ext)
		if err != nil {
			err = fmt.Errorf("preparing regex for '%s'; %w", ext, err)
			goto end
		}
		ssot.LineMatchMap[ext] = re
	}
end:
	return err
}

func (ssot *SSOT) lineMatchRegex(ext string) (re *regexp.Regexp, err error) {
	var regex, prefix string
	re, ok := ssot.LineMatchMap[ext]
	if ok {
		goto end
	}
	prefix, ok = commentPrefixMap[ext]
	if !ok {
		err = fmt.Errorf("unsupported file type '%s'", ext)
		goto end
	}
	regex = fmt.Sprintf(`^(.*)%s\s*ssot\[\s*([^]]+)\s*\]:\s*(.+)\s*$`, regexp.QuoteMeta(prefix))
	re, err = regexp.Compile(regex)
	if err != nil {
		ssot.LineMatchMap[ext] = re
	}
end:
	return re, err
}

func main() {
	slog.Info("SSOT: Reading", "ssot_file", SSOTFile)
	data, err := os.ReadFile(SSOTFile)
	if err != nil {
		slog.Error("Reading SSOT file",
			"ssot_file", SSOTFile,
			"error", err,
		)
		os.Exit(1)
	}

	slog.Info("SSOT: Parsing")
	var ssot SSOT
	err = yaml.Unmarshal(data, &ssot)
	if err != nil {
		slog.Error("Parsing SSOT file",
			"ssot_file", SSOTFile,
			"error", err,
		)
		os.Exit(2)
	}

	slog.Info("SSOT: Initializing")
	err = ssot.Initialize()
	if err != nil {
		slog.Error("Initializing", "ssot_file", SSOTFile, "error", err)
		os.Exit(1)
	}

	errs := make([]error, 0)
	slog.Info("SSOT: Scanning")
	for _, f := range ssot.Files {
		slog.Info("SSOT: File", "source_file", f)
		err = ssot.maybeUpdateFile(f)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		slog.Error("SSOT Error:", "error", errors.Join(errs...))
		os.Exit(3)
	}
	slog.Info("SSOT: Scanning complete")
}

// Define the regex patterns to match ssot comments
var regexSSOTFileMatch = regexp.MustCompile(`ssot\[([^]]+)\]: .+`)

func (ssot *SSOT) maybeUpdateFile(fp string) (err error) {
	var scanner *bufio.Scanner
	var buf bytes.Buffer
	var updated []byte
	var line, ext, commentChars string
	var re *regexp.Regexp
	var ok bool

	// Read the entire file content
	content, err := os.ReadFile(fp)
	if err != nil {
		err = fmt.Errorf("reading file %s; %w", fp, err)
		goto end
	}

	// Check if the file contains any SSOT comments
	if !regexSSOTFileMatch.Match(content) {
		err = fmt.Errorf("no SSOT comments found in %s. Cannot continue; %w", fp, err)
		goto end
	}
	ext = filepath.Ext(fp)
	re, ok = ssot.LineMatchMap[ext]
	if !ok {
		err = fmt.Errorf("line match regex not found for '%s'", ext)
		goto end
	}
	commentChars, ok = commentPrefixMap[ext]
	if !ok {
		err = fmt.Errorf("comment prefix characters not found for file type '%s'", ext)
		goto end
	}
	scanner = bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line, err = ssot.maybeUpdateLine(re, commentChars, scanner.Text())
		if err != nil {
			err = fmt.Errorf("update line %s; %w", line, err)
			goto end
		}
		_, err = buf.WriteString(line + "\n")
		if err != nil {
			err = fmt.Errorf("writing line to buffer %s; %w", line, err)
			goto end
		}
	}
	err = scanner.Err()
	if err != nil {
		err = fmt.Errorf("updating file %s; %w", fp, err)
		goto end
	}

	updated = buf.Bytes()
	if len(bytes.TrimSpace(updated)) == 0 {
		goto end
	}
	if bytes.Equal(content, updated) {
		goto end
	}

	err = os.WriteFile(fp, updated, os.ModePerm)
	if err != nil {
		err = fmt.Errorf("replacing original file %s; %w", fp, err)
		goto end
	}
	slog.Info("SSOT: Updated", "source_file", fp)
end:
	return err
}

func (ssot *SSOT) maybeUpdateLine(re *regexp.Regexp, commentChars, line string) (_ string, err error) {
	var code, constant, regex, newValue, rp string
	var ok bool
	var matches []string

	n := len(line)

	matches = re.FindStringSubmatch(line)
	if matches == nil {
		// This is expected for almost every line
		goto end
	}

	code = matches[1]
	constant = matches[2]
	regex = matches[3]

	re, err = regexp.Compile(regex)
	if err != nil {
		err = fmt.Errorf("compiling SSOT comment regex '%s' for link '%s';%w", regex, line, err)
		goto end
	}
	matches = re.FindStringSubmatch(code)
	if matches == nil {
		err = fmt.Errorf("regular expression '%s' not matching code '%s'", regex, code)
		goto end
	}

	newValue, ok = ssot.Constants[constant]
	if !ok {
		err = fmt.Errorf("SSOT constant '%s' not found in line:%s", constant, line)
		goto end
	}
	newValue = strings.Replace(matches[0], matches[1], newValue, -1)

	re, err = decorateLineRegex(regex)
	if err != nil {
		err = fmt.Errorf("compiling SSOT comment regex '%s' for link '%s';%w", regex, line, err)
		goto end
	}

	rp = fmt.Sprintf("$1%s$3", newValue)
	line = re.ReplaceAllString(code, rp)
	line = fmt.Sprintf("%s%sssot[%s]: %s", line, commentChars, constant, regex)
end:
	line = strings.TrimRight(line, " \t")
	if n != len(line) {
		slog.Warn(fmt.Sprintf("SSOT: Line lengths differ: %d vs %d: %s", n, len(line), line))
	}
	return line, err
}

// decorateLineRegex ensures 3 capture groups to match an entire string; one
// before and one after after the single capture group in the parameter `regex`.
// The output regexp will be created from a regex with beginning and ending string anchors
// no matter if incoming parameter `regex` container them or not.
// This behavior is optimize for DX for the SSOT comment/directive use-case.
func decorateLineRegex(regex string) (re *regexp.Regexp, err error) {
	if len(regex) == 0 {
		err = fmt.Errorf("regular expression is empty")
		goto end
	}
	if regex[0] == '^' {
		regex = "^()" + regex[:1]
	} else {
		regex = "^(.+?)" + regex
	}
	if regex[len(regex)-1] == '$' {
		regex = regex[:1] + "()$"
	} else {
		regex = regex + "(.+?)$"
	}
	re, err = regexp.Compile(regex)
end:
	return re, err
}

func Must(err error) {
	if err != nil {
		slog.Warn("Error occurred when calling Must()", "error", err)
	}
}
