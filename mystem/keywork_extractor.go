package mystem

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os/exec"
	"strings"
	"syscall"
)

type KeywordsExtractor struct {
	binPath string
}

func NewKeywordsExtractor(binPath string) *KeywordsExtractor {
	return &KeywordsExtractor{binPath: binPath}
}

type keywordExtractRequest struct {
	Language string `json:"language"`
}

func (ke *KeywordsExtractor) ExtractKeywords(text string) (
	[]string, error) {

	if text == "" {
		return nil, nil
	}

	res, err := ke.runMystem(strings.NewReader(text))
	if err != nil {
		return nil, errors.New("failed to run mystem: " + err.Error())
	}

	scanner := bufio.NewScanner(res)

	kwsMap := map[string]struct{}{}

	for scanner.Scan() {
		line := scanner.Text()
		for _, kw := range strings.Split(line, "|") {
			kw = strings.TrimRight(kw, "?")
			if !isStopWord(kw) {
				kwsMap[kw] = struct{}{}
			}
		}
	}

	var kws []string
	for kw := range kwsMap {
		kws = append(kws, kw)
	}

	return kws, nil
}

func (ke *KeywordsExtractor) runMystem(stdin io.Reader) (io.Reader, error) {

	stdout := bytes.NewBuffer(nil)

	cmd := exec.Command(ke.binPath, "-n", "-l")
	cmd.Stdin = stdin
	cmd.Stdout = stdout

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	err := cmd.Run()

	return stdout, err
}
