package container

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"testing"
	"time"

	"golang.org/x/net/context"
)

/**** not using actual files from file system for testing.... use these funcs if we want to do that
func getCodeChainBytes(pathtocodechain string) (io.Reader, error) {
	inputbuf := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(inputbuf)
	tr := tar.NewWriter(gw)
	// Get the Tar contents for the image
	err := writeCodeChainTar(pathtocodechain, tr)
	tr.Close()
	gw.Close()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error getting codechain tar: %s", err))
	}
        ioutil.WriteFile("/tmp/chaincode.tar", inputbuf.Bytes(), 0644)
	return inputbuf, nil
}

func writeCodeChainTar(pathtocodechain string, tw *tar.Writer) error {
	root_directory := pathtocodechain //use full path
	fmt.Printf("tar %s start(%s)\n", root_directory, time.Now())

	walkFn := func(path string, info os.FileInfo, err error) error {
	        fmt.Printf("path %s(%s)\n", path, info.Name())
                if info == nil {
	             return errors.New(fmt.Sprintf("Error walking the path: %s", path))
                }

		if info.Mode().IsDir() {
			return nil
		}
		// Because of scoping we can reference the external root_directory variable
		//new_path := fmt.Sprintf("%s", path[len(root_directory):])
		new_path := info.Name()

		if len(new_path) == 0 {
			return nil
		}

		fr, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fr.Close()

		if h, err := tar.FileInfoHeader(info, new_path); err != nil {
			fmt.Printf(fmt.Sprintf("Error getting FileInfoHeader: %s\n", err))
			return err
		} else {
			h.Name = new_path
			if err = tw.WriteHeader(h); err != nil {
				fmt.Printf(fmt.Sprintf("Error writing header: %s\n", err))
				return err
			}
		}
		if length, err := io.Copy(tw, fr); err != nil {
			return err
		} else {
			fmt.Printf("Length of entry = %d\n", length)
		}
		return nil
	}

	if err := filepath.Walk(root_directory, walkFn); err != nil {
		fmt.Printf("Error walking root_directory: %s\n", err)
		return err
	} else {
		// Write the tar file out
		if err := tw.Close(); err != nil {
                    return err
		}
	}
	fmt.Printf("tar end = %s\n", time.Now())
	return nil
}
*********************/

func getCodeChainBytesInMem() (io.Reader, error) {
	startTime := time.Now()
	inputbuf := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(inputbuf)
	tr := tar.NewWriter(gw)
	dockerFileContents := []byte("FROM busybox:latest\n\nCMD sleep 300")
	dockerFileSize := int64(len([]byte(dockerFileContents)))

	tr.WriteHeader(&tar.Header{Name: "Dockerfile", Size: dockerFileSize, ModTime: startTime, AccessTime: startTime, ChangeTime: startTime})
	tr.Write([]byte(dockerFileContents))
	tr.Close()
	gw.Close()
	ioutil.WriteFile("/tmp/chaincode.tar", inputbuf.Bytes(), 0644)
	return inputbuf, nil
}

func TestVMCBuildImage(t *testing.T) {
	var ctxt = context.Background()

	//get the tarball for codechain
	tarRdr, err := getCodeChainBytesInMem()
	if err != nil {
		t.Fail()
		t.Logf("Error reading tar file: %s", err)
		return
	}

	c := make(chan struct{})

	//creat a CreateImageReq obj and send it to VMCProcess
	go func() {
		defer close(c)
		cir := CreateImageReq{ID: "simple", Reader: tarRdr}
		_, err := VMCProcess(ctxt, "Docker", cir)
		if err != nil {
			t.Fail()
			t.Logf("Error creating image: %s", err)
			return
		}
	}()

	//wait for VMController to complete.
	fmt.Println("VMCBuildImage-waiting for response")
	<-c
}

func TestVMCStartContainer(t *testing.T) {

	var ctxt = context.Background()

	c := make(chan struct{})

	//creat a StartImageReq obj and send it to VMCProcess
	go func() {
		defer close(c)
		args := []string{"echo", "hello"}
		var outbuf bytes.Buffer
		sir := StartImageReq{ID: "simple", Args: args, Instream: nil, Outstream: &outbuf}
		_, err := VMCProcess(ctxt, "Docker", sir)
		if err != nil {
			t.Fail()
			t.Logf("Error starting container: %s", err)
			return
		}
		fmt.Printf("Output-%s", string(outbuf.Bytes()))
		if "hello\n" != string(outbuf.Bytes()) {
			t.Fail()
			t.Logf("expected hello, received : %s", string(outbuf.Bytes()))
			return
		}
	}()

	//wait for VMController to complete.
	fmt.Println("VMCStartContainer-waiting for response")
	<-c
}

func TestVMCStartContainerSync(t *testing.T) {
	var ctxt = context.Background()

	//creat a StartImageReq obj and send it to VMCProcess
	args := []string{"echo", "hi there"}
	var outbuf bytes.Buffer
	sir := StartImageReq{ID: "simple", Args: args, Instream: nil, Outstream: &outbuf}
	_, err := VMCProcess(ctxt, "Docker", sir)
	if err != nil {
		t.Fail()
		t.Logf("Error starting container: %s", err)
		return
	}
	fmt.Printf("Output-%s", string(outbuf.Bytes()))
	if "hi there\n" != string(outbuf.Bytes()) {
		t.Fail()
		t.Logf("expected hello, received : %s", string(outbuf.Bytes()))
		return
	}
}

func TestVMCStopContainer(t *testing.T) {
	var ctxt = context.Background()

	c := make(chan struct{})

	//creat a StopImageReq obj and send it to VMCProcess
	go func() {
		defer close(c)
		sir := StopImageReq{ID: "simple", Timeout: 0}
		_, err := VMCProcess(ctxt, "Docker", sir)
		if err != nil {
			t.Fail()
			t.Logf("Error stopping container: %s", err)
			return
		}
	}()

	//wait for VMController to complete.
	fmt.Println("VMCStopContainer-waiting for response")
	<-c
}
