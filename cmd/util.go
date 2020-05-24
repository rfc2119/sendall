package cmd

import (
	"path/filepath"
	"fmt"
)

func sanitize(fileName string) string {
	// didn't know about path.Clean()! credit goes to DutchCoders
	return filepath.Clean(filepath.Base(fileName))
}

func prepareFiles(files []string) []string {
	fileList := make([]string, len(files)) // TODO: be careful if you changed command syntax
	for idx, file := range files {
		if absPath, err := filepath.Abs(file); err != nil { // calls filepath.Clean() on exit
			fmt.Println(err)
		} else {
			fileList[idx] = absPath
		}
	}

	return fileList
}
