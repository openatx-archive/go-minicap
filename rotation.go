/*
* File: rotation.go
* Author : bigwavelet
* Description: android rotation watcher
* Created: 2016-09-13
 */

package minicap

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Rotation struct {
	d           AdbDevice
	orientation int
	ps          *exec.Cmd
	closed      bool
	brd         *bufio.Reader
}

func newRotation(option Options) (r Rotation, err error) {
	r = Rotation{}
	r.d, err = NewAdbDevice(option.Serial, option.Adb)
	r.closed = true
	return
}

//install rotationWatcher.apk
func (r *Rotation) install() (err error) {
	tmpDir := os.TempDir()
	url := "https://github.com/NetEaseGame/AutomatorX/raw/master/atx/vendor/RotationWatcher.apk"
	//check package
	pkgName := "jp.co.cyberagent.stf.rotationwatcher"
	plist, err := r.d.getPackageList()
	for _, val := range plist {
		if strings.Contains(val, pkgName) {
			return
		}
	}
	//downlaod apk
	fileName := filepath.Join(tmpDir, "RotationWatcher.apk")
	err = downloadFile(fileName, url)
	if err != nil {
		return
	}
	//install apk
	_, err = r.d.run("install", "-rt", fileName)
	if err != nil {
		return
	}
	return
}

//start rotation service
func (r *Rotation) start() (err error) {
	pkgName := "jp.co.cyberagent.stf.rotationwatcher"
	out, err := r.d.shell("pm path " + pkgName)
	if err != nil {
		return
	}
	fields := strings.Split(strip(out), ":")
	path := fields[len(fields)-1]
	r.ps = r.d.buildCommand("CLASSPATH="+path, "app_process", "/system/bin", "jp.co.cyberagent.stf.rotationwatcher.RotationWatcher")
	r.ps.Stderr = os.Stderr
	stdoutReader, err := r.ps.StdoutPipe()
	if err != nil {
		return
	}
	r.brd = bufio.NewReader(stdoutReader)
	return r.ps.Start()
}

func (r *Rotation) watch() (orienC <-chan int, err error) {
	rC := make(chan int, 0)
	go func() {
		for {
			line, _, er := r.brd.ReadLine()
			if er != nil {
				break
			}
			tmp := strings.Replace(string(line), "\r", "", -1)
			tmp = strings.Replace(tmp, "\n", "", -1)
			orientation, er := strconv.Atoi(string(tmp))
			if er != nil {
				break
			}
			rC <- orientation
		}
	}()
	orienC = rC
	return
}
