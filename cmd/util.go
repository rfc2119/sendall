package cmd

import (
	"path"
)

func sanitize(fileName string) string{
	// didn't know about path.Clean()! credit goes to DutchCoders
	return path.Clean(path.Base(fileName))
}
