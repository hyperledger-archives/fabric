package util

import (
	"archive/tar"
	"fmt"
	"github.com/op/go-logging"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var vmLogger = logging.MustGetLogger("container")

func WriteGopathSrc(tw *tar.Writer, excludeDir string) error {
	gopath := os.Getenv("GOPATH")
	if strings.LastIndex(gopath, "/") == len(gopath)-1 {
		gopath = gopath[:len(gopath)]
	}
	rootDirectory := fmt.Sprintf("%s%s%s", os.Getenv("GOPATH"), string(os.PathSeparator), "src")
	vmLogger.Info("rootDirectory = %s", rootDirectory)

	//append "/" if necessary
	if excludeDir != "" && strings.LastIndex(excludeDir, "/") < len(excludeDir)-1 {
		excludeDir = excludeDir + "/"
	}

	rootDirLen := len(rootDirectory)
	walkFn := func(path string, info os.FileInfo, err error) error {

		// If path includes .git, ignore
		if strings.Contains(path, ".git") {
			return nil
		}

		if info.Mode().IsDir() {
			return nil
		}

		//exclude any files with excludeDir prefix. They should already be in the tar
		if excludeDir != "" && strings.Index(path, excludeDir) == rootDirLen+1 { //1 for "/"
			return nil
		}
		// Because of scoping we can reference the external rootDirectory variable
		newPath := fmt.Sprintf("src%s", path[rootDirLen:])
		//newPath := path[len(rootDirectory):]
		if len(newPath) == 0 {
			return nil
		}

		fr, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("Error opening path %s: %s", path, err)
		}
		defer fr.Close()

		h, err := tar.FileInfoHeader(info, newPath)
		if err != nil {
			vmLogger.Error(fmt.Sprintf("Error getting FileInfoHeader: %s", err))
			return fmt.Errorf("Error getting file header %s: %s", newPath, err)
		}
		//Let's take the variance out of the tar, make headers identical everywhere by using zero time
		oldname := h.Name
		var zeroTime time.Time
		h.AccessTime = zeroTime
		h.ModTime = zeroTime
		h.ChangeTime = zeroTime
		h.Name = newPath
		if err = tw.WriteHeader(h); err != nil {
			return fmt.Errorf("Error write header for (path: %s, oldname:%s,newname:%s,sz:%d) : %s", path, oldname, newPath, h.Size, err)
		}
		if _, err := io.Copy(tw, fr); err != nil {
			return fmt.Errorf("Error copy (path: %s, oldname:%s,newname:%s,sz:%d) : %s", path, oldname, newPath, h.Size, err)
		}
		return nil
	}

	if err := filepath.Walk(rootDirectory, walkFn); err != nil {
		vmLogger.Info("Error walking rootDirectory: %s", err)
		return err
	}
	// Write the tar file out
	if err := tw.Close(); err != nil {
		return err
	}
	//ioutil.WriteFile("/tmp/chaincode_deployment.tar", inputbuf.Bytes(), 0644)
	return nil
}
