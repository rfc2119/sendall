package cmd

import (
	"path"
)

func sanitize(fileName string) string{
	// didn't know about path.Clean()! credits goes to DutchCoders
	return path.Clean(path.Base(fileName))
}
