package archive

import "os"

func openBlobFile(path string) (*os.File, error) {
	return os.Open(path)
}
