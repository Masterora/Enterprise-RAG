package parser

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func parseDOC(data []byte, _ ParseOptions) (string, []byte, error) {
	plainText, err := convertLegacyWordToText(data)
	if err != nil {
		return "", nil, err
	}
	return parsePlainText([]byte(plainText))
}

func convertLegacyWordToText(data []byte) (string, error) {
	if output, err := runTextutilConvert(data, ".doc"); err == nil {
		return output, nil
	}
	if output, err := runStdoutConverter(data, ".doc", "antiword"); err == nil {
		return output, nil
	}
	if output, err := runStdoutConverter(data, ".doc", "catdoc"); err == nil {
		return output, nil
	}
	return "", errors.New("当前环境无法解析 .doc 文件，请先将其转换为 docx、pdf、txt，或安装 textutil / antiword / catdoc 后重试")
}

func runTextutilConvert(data []byte, suffix string) (string, error) {
	return runTempFileConverter(data, suffix, func(path string) (*exec.Cmd, error) {
		binary, err := exec.LookPath("textutil")
		if err != nil {
			return nil, err
		}
		return exec.Command(binary, "-convert", "txt", "-stdout", path), nil
	})
}

func runStdoutConverter(data []byte, suffix, binaryName string) (string, error) {
	return runTempFileConverter(data, suffix, func(path string) (*exec.Cmd, error) {
		binary, err := exec.LookPath(binaryName)
		if err != nil {
			return nil, err
		}
		return exec.Command(binary, path), nil
	})
}

func runTempFileConverter(data []byte, suffix string, build func(path string) (*exec.Cmd, error)) (string, error) {
	input, err := os.CreateTemp("", "rag-doc-*"+suffix)
	if err != nil {
		return "", err
	}
	inputPath := input.Name()
	defer os.Remove(inputPath)
	if _, err := input.Write(data); err != nil {
		input.Close()
		return "", err
	}
	if err := input.Close(); err != nil {
		return "", err
	}

	cmd, err := build(inputPath)
	if err != nil {
		return "", err
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return "", err
	}
	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return "", errors.New("converted text is empty")
	}
	return output, nil
}
