/*
* File: minicap.go
* Author : bigwavelet
* Description: android minicap service
* Created: 2016-09-13
 */

package minicap

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	ErrAlreadyClosed = errors.New("already closed")
)

type Options struct {
	Serial string
	Host   string
	Port   int
	Adb    string
}

type Service struct {
	d        AdbDevice
	ps       *exec.Cmd
	port     int
	host     string
	r        Rotation
	dispInfo DisplayInfo

	closed   bool
	imageC   chan image.Image
	mu       sync.Mutex
	imBuffer image.Image
}

func NewService(option Options) (s Service, err error) {
	s = Service{}
	s.d, err = NewAdbDevice(option.Serial, option.Adb)
	if option.Port == 0 {
		port, err := randPort()
		if err != nil {
			return s, errors.New("port required")
		}
		s.port = port
	} else {
		s.port = option.Port
	}
	if option.Host == "" {
		s.host = "localhost"
	} else {
		s.host = option.Host
	}
	s.closed = true
	s.r, err = newRotation(option)
	if err != nil {
		return
	}
	return
}

//install minicap
func (s *Service) Install() (err error) {
	err = s.r.install()
	if err != nil {
		return
	}
	isMinicapSoExisted, _ := s.d.isFileExisted("/data/local/tmp/minicap.so")
	isMinicapExisted, _ := s.d.isFileExisted("/data/local/tmp/minicap")
	if isMinicapExisted && isMinicapSoExisted {
		return
	}
	abi, err := s.d.getProp("ro.product.cpu.abi")
	if err != nil {
		return
	}
	sdk, err := s.d.getProp("ro.build.version.sdk")
	if err != nil {
		return
	}
	tmpDir := os.TempDir()
	// download minicap.so from github
	url := "https://github.com/openstf/stf/raw/master/vendor/minicap/shared/android-" + sdk + "/" + abi + "/minicap.so"
	fileName := filepath.Join(tmpDir, "minicap.so")
	err = downloadFile(fileName, url)
	if err != nil {
		return
	}
	//push minicap.so to device
	_, err = s.d.run("push", fileName, "/data/local/tmp")
	if err != nil {
		return
	}
	//download minicap
	url = "https://github.com/openstf/stf/raw/master/vendor/minicap/bin/" + abi + "/minicap"
	fileName = filepath.Join(tmpDir, "minicap")
	err = downloadFile(fileName, url)
	if err != nil {
		return
	}
	//push minicap to device
	_, err = s.d.run("push", fileName, "/data/local/tmp")
	if err != nil {
		return
	}
	// chmod
	cmdstr := "chmod 0755 /data/local/tmp/minicap"
	_, err = s.d.shell(cmdstr)
	if err != nil {
		return
	}
	return
}

//check minicap
func (s *Service) IsSupported() bool {
	out, err := s.d.shell("LD_LIBRARY_PATH=/data/local/tmp /data/local/tmp/minicap -i")
	if err != nil {
		return false
	}
	supported := strings.Contains(out, "height") && strings.Contains(out, "width")
	return supported
}

//get Fps
func (s *Service) FPS() (fps int, err error) {
	out, err := s.d.shell("LD_LIBRARY_PATH=/data/local/tmp /data/local/tmp/minicap -i")
	if err != nil {
		return 0, err
	}
	supported := strings.Contains(out, "height") && strings.Contains(out, "width") && strings.Contains(out, "fps")
	if !supported {
		return 0, errors.New("minicap not supported")
	}
	//json str 转map
	var dat map[string]interface{}
	if err := json.Unmarshal([]byte(out), &dat); err == nil {
		fpsStr := fmt.Sprintf("%v", dat["fps"])
		fps, err := strconv.Atoi(fpsStr)
		return fps, err
	} else {
		return 0, err
	}

}

// uninstall minicap
func (s *Service) Uninstall() (err error) {
	isMinicapExisted, _ := s.d.isFileExisted("/data/local/tmp/minicap.so")
	if isMinicapExisted {
		if _, err := s.d.shell("rm /data/local/tmp/minicap.so"); err != nil {
			return err
		}
	}
	isMinicapsoExisted, _ := s.d.isFileExisted("/data/local/tmp/minicap")
	if isMinicapsoExisted {
		if _, err := s.d.shell("rm /data/local/tmp/minicap"); err != nil {
			return err
		}
	}
	return nil
}

// Capture a screen
func (s *Service) Screenshot() (im image.Image, err error) {
	if !s.IsSupported() {
		err = errors.New("sorry, minicap not supported")
		return
	}
	dispInfo, err := s.d.getDisplayInfo()
	if err != nil {
		return
	}
	if dispInfo.Width > dispInfo.Height {
		dispInfo.Width, dispInfo.Height = dispInfo.Height, dispInfo.Width
	}
	params := fmt.Sprintf("%dx%d@%dx%d/%d", dispInfo.Width, dispInfo.Height,
		dispInfo.Width, dispInfo.Height, dispInfo.Orientation*90)
	fName := randSeq(10)
	fName = fmt.Sprintf("go_%v.jpg", fName)
	cmd := fmt.Sprintf("LD_LIBRARY_PATH=/data/local/tmp /data/local/tmp/minicap -n minicap -P %v -s > /data/local/tmp/%v", params, fName)
	_, err = s.d.shell(cmd)
	if err != nil {
		return
	}
	//decode from jpg file
	tmpDir := os.TempDir()
	LFName := filepath.Join(tmpDir, fName)
	cmds := []string{"pull", fmt.Sprintf("/data/local/tmp/%v", fName), LFName}
	_, err = s.d.run(cmds...)
	if err != nil {
		return
	}
	s.d.shell(fmt.Sprintf("rm /data/local/tmp/%v", fName))

	fout, err := os.Open(LFName)
	if err != nil {
		return
	}
	im, err = jpeg.Decode(fout)
	return
}

//start rotation watcher
func (s *Service) Capture() (imageC <-chan image.Image, err error) {
	err = s.r.start()
	if err != nil {
		return
	}
	orienC, err := s.r.watch()
	if err != nil {
		return
	}

	<-orienC
	if err = s.captureMinicap(); err != nil {
		return
	}
	go func() {
		for {
			orientation := <-orienC
			if err := s.startMinicap(orientation); err != nil {
				break
			}
			time.Sleep(time.Duration(10+rand.Intn(100)) * time.Millisecond)
		}
	}()
	return s.imageC, nil
}

//start minicap
func (s *Service) startMinicap(orientation int) (err error) {
	if !s.IsSupported() {
		err = errors.New("sorry, minicap not supported")
		return
	}
	if s.dispInfo.Height == 0 {
		s.dispInfo, err = s.d.getDisplayInfo()
		if err != nil {
			return
		}
	}
	if s.dispInfo.Width > s.dispInfo.Height {
		s.dispInfo.Width, s.dispInfo.Height = s.dispInfo.Height, s.dispInfo.Width
	}

	s.closeMinicap()
	params := fmt.Sprintf("%dx%d@%dx%d/%d", s.dispInfo.Width, s.dispInfo.Height,
		s.dispInfo.Width, s.dispInfo.Height, orientation)
	s.ps, err = s.d.shellNowait("LD_LIBRARY_PATH=/data/local/tmp", "/data/local/tmp/minicap", "-P", params, "-S")
	if err != nil {
		return
	}
	time.Sleep(time.Millisecond * 500)
	if _, err = s.d.run("forward", fmt.Sprintf("tcp:%d", s.port), "localabstract:minicap"); err != nil {
		return
	}
	s.closed = false
	return
}

// close minicap service
/*
1. kill minicap ps
2. remove adb forward
*/
func (s *Service) Close() (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrAlreadyClosed
	}
	s.closed = true
	close(s.imageC)

	if s.ps != nil && s.ps.Process != nil {
		s.ps.Process.Signal(syscall.SIGTERM)
	}
	//kill minicap ps on device
	err = s.d.killPs("minicap")
	s.d.run("forward", "--remove", fmt.Sprintf("tcp:%d", s.port))
	return
}

func (s *Service) closeMinicap() (err error) {
	if s.ps != nil && s.ps.Process != nil {
		s.ps.Process.Signal(syscall.SIGTERM)
	}
	//kill minicap ps on device
	err = s.d.killPs("minicap")
	return
}

//check if minicap service is closed.
func (s *Service) IsClosed() (Closed bool) {
	return s.closed
}

// screen capture
func (s *Service) captureMinicap() (err error) {
	var conn net.Conn
	s.dispInfo, err = s.d.getDisplayInfo()
	if err != nil {
		return
	}
	if err = s.startMinicap(s.dispInfo.Orientation * 90); err != nil {
		return
	}
	s.imageC = make(chan image.Image, 1)
	go func() {
		for {
			conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", s.host, s.port))
			if err != nil {
				continue
			}
			var pid, rw, rh, vw, vh uint32
			var version uint8
			var unused uint8
			var orientation uint8
			binRead := func(data interface{}) error {
				if err != nil {
					return err
				}
				err = binary.Read(conn, binary.LittleEndian, data)
				return err
			}
			binRead(&version)
			binRead(&unused)
			binRead(&pid)
			binRead(&rw)
			binRead(&rh)
			binRead(&vw)
			binRead(&vh)
			binRead(&orientation)
			binRead(&unused)
			if err != nil {
				continue
			}
			bufrd := bufio.NewReader(conn) // Do not put it into for loop
			for {
				var size uint32
				if err = binRead(&size); err != nil {
					break
				}
				lr := &io.LimitedReader{bufrd, int64(size)}
				var im image.Image
				im, err = jpeg.Decode(lr)
				if err != nil {
					break
				}
				s.mu.Lock()
				if s.closed {
					break
				}
				s.imBuffer = im
				select {
				case s.imageC <- im:
				default:
				}
				s.mu.Unlock()
			}
			conn.Close()
		}
	}()
	return nil
}

// get last screenshot
func (s *Service) LastScreenshot() (im image.Image, err error) {
	if s.imBuffer == nil {
		return nil, errors.New("screenshot not found")
	}
	return s.imBuffer, nil
}
